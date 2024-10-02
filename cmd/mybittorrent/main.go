package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(value string) (interface{}, int, error) {
	switch {
	case unicode.IsDigit(rune(value[0])):
		return decodeString(value)
	case value[0] == 'i':
		return decodeInteger(value)
	case value[0] == 'l':
		return decodeList(value)
	case value[0] == 'd':
		return decodeDictionary(value)
	default:
		return "", 0, fmt.Errorf("Invalid payload")
	}
}

func encode(data interface{}) string {
	if str, ok := data.(string); ok {
		return fmt.Sprintf("%d:%s", len(str), str)
	}

	if integer, ok := data.(int); ok {
		return fmt.Sprintf("i%de", integer)
	}

	if list, ok := data.([]interface{}); ok {
		var encodedValue string
		for _, item := range list {
			encodedValue += encode(item)
		}
		return fmt.Sprintf("l%se", encodedValue)
	}

	if dict, ok := data.(map[string]interface{}); ok {
		var keys []string
		for k := range dict {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		var encodedValue string
		for _, k := range keys {
			encodedValue += encode(k) + encode(dict[k])
		}
		return fmt.Sprintf("d%se", encodedValue)
	}

	return ""
}

func decodeDictionary(value string) (interface{}, int, error) {
	dict := map[string]interface{}{}
	index := 1
	var key string
	var parsedValues int

	for index < len(value) {
		if value[index] == 'e' {
			return dict, index + 1, nil
		}

		r, length, err := decodeBencode(value[index:])

		if err != nil {
			return nil, 0, err
		}

		index += length

		if parsedValues%2 != 0 {
			dict[key] = r
		} else if k, ok := r.(string); ok {
			key = k
		} else {
			return nil, 0, fmt.Errorf("invalid key")
		}
		parsedValues++
	}

	return nil, 0, fmt.Errorf("invalid dictionary")
}

func decodeList(value string) (interface{}, int, error) {
	list := []interface{}{}
	index := 1

	for index < len(value) {
		if value[index] == 'e' {
			return list, index + 1, nil
		}

		result, length, err := decodeBencode(value[index:])
		if err != nil {
			return nil, 0, err
		}
		list = append(list, result)
		index += length
	}

	return nil, 0, fmt.Errorf("invalid list")
}

func decodeInteger(value string) (interface{}, int, error) {
	end := strings.IndexRune(value, 'e')
	if end == -1 {
		return nil, 0, fmt.Errorf("invalid integer")
	}
	integer, err := strconv.Atoi(string(value[1:end]))
	if err != nil {
		return nil, 0, fmt.Errorf("invalid integer: %w", err)
	}

	return integer, end + 1, nil
}

func decodeString(value string) (interface{}, int, error) {
	firstColonIndex := strings.IndexRune(value, ':')
	lengthStr := value[:firstColonIndex]

	length, err := strconv.Atoi(string(lengthStr))
	if err != nil {
		return "", 0, err
	}

	return string(value[firstColonIndex+1 : firstColonIndex+1+length]), len(lengthStr) + 1 + length, nil
}

func runDecodeCommand(value string) error {
	decoded, _, err := decodeBencode(value)
	if err != nil {
		return err
	}

	jsonOutput, _ := json.Marshal(decoded)
	fmt.Println(string(jsonOutput))
	return nil
}

type TorrentInfo struct {
	trackerURL  string
	length      int
	hash        []byte
	pieceLength int
	pieceHashes [][]byte
}

func getInfo(payload string) (*TorrentInfo, error) {
	decodedValue, _, err := decodeBencode(payload)

	if err != nil {
		return nil, err
	}

	dict := decodedValue.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	h := sha1.New()

	if _, err := h.Write([]byte(encode(info))); err != nil {
		return nil, err
	}

	rawPieces := info["pieces"].(string)
	var pieces [][]byte
	for i := 0; i < len(rawPieces); i += 20 {
		pieces = append(pieces, []byte(rawPieces[i:i+20]))
	}

	return &TorrentInfo{
		trackerURL:  dict["announce"].(string),
		length:      info["length"].(int),
		hash:        h.Sum(nil),
		pieceLength: info["piece length"].(int),
		pieceHashes: pieces,
	}, nil
}

func runInfoCommand(file string) error {
	payload, err := os.ReadFile(file)

	if err != nil {
		return err
	}

	info, err := getInfo(string(payload))

	if err != nil {
		return err
	}

	fmt.Printf("Tracker URL: %s\n", info.trackerURL)
	fmt.Printf("Length: %d\n", info.length)
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(info.hash))
	fmt.Printf("Piece Length: %d\n", info.pieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range info.pieceHashes {
		fmt.Println(hex.EncodeToString(piece))
	}

	return nil
}

func getPeers(info TorrentInfo) ([]string, error) {
	u, err := url.Parse(info.trackerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}

	query := u.Query()
	query.Add("info_hash", string(info.hash))
	query.Add("peer_id", "00112233445566778899")
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(info.length))
	query.Add("compact", "1")

	u.RawQuery = query.Encode()

	r, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("could not make HTTP request: %w", err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read HTTP response: %w", err)
	}
	defer r.Body.Close()

	parsedResp, _, err := decodeBencode(string(body))
	if err != nil {
		return nil, err
	}

	m := parsedResp.(map[string]interface{})
	rawPeers := []byte(m["peers"].(string))
	var peers []string

	for i := 0; i < len(rawPeers); i += 6 {
		peer := rawPeers[i : i+6]
		ip := fmt.Sprintf("%d.%d.%d.%d", peer[0], peer[1], peer[2], peer[3])
		port := binary.BigEndian.Uint16(peer[4:])
		peers = append(peers, fmt.Sprintf("%s:%d", ip, port))
	}

	return peers, nil
}

func runPeersCommand(file string) error {
	payload, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read file in peers command: %w", err)
	}

	info, err := getInfo(string(payload))

	if err != nil {
		return err
	}

	peers, err := getPeers(*info)
	if err != nil {
		return err
	}

	for _, peer := range peers {
		fmt.Println(peer)
	}

	return nil
}

func performHandshake(conn net.Conn, info TorrentInfo) ([]byte, error) {
	message := []byte{19}
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	message = append(message, info.hash...)
	message = append(message, []byte("00112233445566778899")...)

	if _, err := conn.Write(message); err != nil {
		return nil, fmt.Errorf("error sending message to peer: %w", err)
	}

	resp := make([]byte, 68)
	if _, err := conn.Read(resp); err != nil {
		return nil, fmt.Errorf("error reading message from peer: %w", err)
	}

	return resp, nil
}

func runHandshakeCommand(file, peer string) error {
	payload, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not read file in peers command: %w", err)
	}

	info, err := getInfo(string(payload))

	if err != nil {
		return err
	}

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return err
	}

	defer conn.Close()

	resp, err := performHandshake(conn, *info)
	if err != nil {
		return err
	}

	fmt.Println("Peer ID:", hex.EncodeToString(resp[48:68]))

	return nil
}

func sendPeerMessage(conn net.Conn, messageId uint32, payload []byte) error {
	length := len(payload) + 1
	message := make([]byte, length+4)
	binary.BigEndian.PutUint32(message, uint32(length))
	message[4] = byte(messageId)
	for i, b := range payload {
		message[i+5] = b
	}

	if _, err := conn.Write(message); err != nil {
		return err
	}

	fmt.Printf("sent message id: %d with len: %d, len2: %d\n", messageId, length, len(message))
	fmt.Println(message)

	return nil
}

type PeerMessage struct {
	id      byte
	payload []byte
}

func readPeerMessage(conn net.Conn) (*PeerMessage, error) {
	header := make([]byte, 5)
	if _, err := conn.Read(header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header[:4])
	id := header[4]

	fmt.Printf("received message id: %d with len: %d\n", id, length)

	payload := make([]byte, length-1)

	if _, err := io.ReadAtLeast(conn, payload, len(payload)); err != nil {
		// if _, err := conn.Read(payload); err != nil {
		return nil, err
	}

	return &PeerMessage{id: id, payload: payload}, nil
}

func runDownloadPieceCommand(args []string) error {
	outputFile := args[3]
	inputFile := args[4]

	pieceIndex, err := strconv.Atoi(args[5])
	if err != nil {
		return err
	}

	payload, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("could not read input file: %w", err)
	}

	info, err := getInfo(string(payload))
	if err != nil {
		return err
	}

	peers, err := getPeers(*info)
	if err != nil {
		return err
	}

	conn, err := net.Dial("tcp", peers[0])
	if err != nil {
		return fmt.Errorf("could not connect to peer %s: %w", peers[0], err)
	}

	defer conn.Close()

	if _, err := performHandshake(conn, *info); err != nil {
		return err
	}

	bitfieldMsg, err := readPeerMessage(conn)
	if err != nil {
		return err
	}
	if bitfieldMsg.id != 5 {
		return fmt.Errorf("unexpected bitfield message. Got id: %d", bitfieldMsg.id)
	}

	if err := sendPeerMessage(conn, 2, nil); err != nil {
		return err
	}

	unchokeMsg, err := readPeerMessage(conn)
	if err != nil {
		return err
	}
	if unchokeMsg.id != 1 {
		return fmt.Errorf("unexpected unchoke message. Got id: %d", unchokeMsg.id)
	}

	blockMaxSize := 16 * 1024
	pieceLength := min(info.pieceLength, info.length-info.pieceLength*pieceIndex)
	nblocks := int(math.Round(float64(pieceLength) / float64(blockMaxSize)))
	blocks := make([][]byte, nblocks)
	fmt.Println("piece len:", pieceLength)

	for i := 0; i < nblocks; i++ {
		blockLength := min(blockMaxSize, pieceLength-i*blockMaxSize)
		var message []byte

		message = binary.BigEndian.AppendUint32(message, uint32(pieceIndex))
		message = binary.BigEndian.AppendUint32(message, uint32(i*blockMaxSize))
		message = binary.BigEndian.AppendUint32(message, uint32(blockLength))

		fmt.Println("offset: ", i*blockMaxSize, "len: ", blockLength, message)

		if err := sendPeerMessage(conn, 6, message); err != nil {
			return err
		}
		// }

		// for i := 0; i < nblocks; i++ {
		// if i == 1 {
		// 	continue
		// }

		pieceMsg, err := readPeerMessage(conn)
		if err != nil {
			return err
		}

		if pieceMsg.id != 7 {
			return fmt.Errorf("unexpected piece message. Got id: %d", pieceMsg.id)
		}

		index := binary.BigEndian.Uint32(pieceMsg.payload[4:8]) / uint32(blockMaxSize)
		fmt.Println(binary.BigEndian.Uint32(pieceMsg.payload[0:4]), index)
		blocks[index] = pieceMsg.payload[8:]
	}

	f, err := os.Create(outputFile)
	if err != err {
		return err
	}
	defer f.Close()

	for _, block := range blocks {
		if _, err := f.Write(block); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		if err := runDecodeCommand(os.Args[2]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "info":
		if err := runInfoCommand(os.Args[2]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "peers":
		if err := runPeersCommand(os.Args[2]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "handshake":
		if err := runHandshakeCommand(os.Args[2], os.Args[3]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "download_piece":
		if err := runDownloadPieceCommand(os.Args); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
