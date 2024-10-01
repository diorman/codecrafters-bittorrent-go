package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	pieces      [][]byte
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
		pieces:      pieces,
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
	for _, piece := range info.pieces {
		fmt.Println(hex.EncodeToString(piece))
	}

	return nil
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

	u, err := url.Parse(info.trackerURL)
	if err != nil {
		return fmt.Errorf("could not parse URL in peers command: %w", err)
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
		return fmt.Errorf("could not make HTTP request in peers command: %w", err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("could not read HTTP response in peers command: %w", err)
	}
	defer r.Body.Close()

	parsedResp, _, err := decodeBencode(string(body))
	if err != nil {
		return err
	}

	m := parsedResp.(map[string]interface{})
	peers := []byte(m["peers"].(string))

	for i := 0; i < len(peers); i += 6 {
		peer := peers[i : i+6]
		ip := fmt.Sprintf("%d.%d.%d.%d", peer[0], peer[1], peer[2], peer[3])
		port := binary.BigEndian.Uint16(peer[4:])
		fmt.Printf("%s:%d\n", ip, port)
	}

	return nil
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

	message := []byte{19}
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	message = append(message, info.hash...)
	message = append(message, []byte("00112233445566778899")...)

	if _, err := conn.Write(message); err != nil {
		return fmt.Errorf("error sending message to peer: %w", err)
	}

	var resp []byte
	if _, err := conn.Read(resp); err != nil {
		return fmt.Errorf("error reading message from peer: %w", err)
	}

	fmt.Println(string(resp[48:68]))

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
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
