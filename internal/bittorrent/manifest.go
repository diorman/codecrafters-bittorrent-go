package bittorrent

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

type Manifest struct {
	TrackerURL  string
	Length      int
	Hash        []byte
	PieceLength int
	PieceHashes [][]byte
}

func (t Manifest) PeerAddresses() ([]string, error) {
	u, err := url.Parse(t.TrackerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse BitTorrent tracker URL: %w", err)
	}

	query := u.Query()
	query.Add("info_hash", string(t.Hash))
	query.Add("peer_id", "00112233445566778899")
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(t.Length))
	query.Add("compact", "1")
	u.RawQuery = query.Encode()

	r, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("could not request BitTorrent tracker URL: %w", err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read BitTorrent tracker response: %w", err)
	}
	defer r.Body.Close()

	obj, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("could not decode BitTorrent tracker response: %w", err)
	}

	m := obj.(map[string]interface{})
	rawPeerAddresses := []byte(m["peers"].(string))
	var peerAddresses []string

	for i := 0; i < len(rawPeerAddresses); i += 6 {
		rawPeerAddress := rawPeerAddresses[i : i+6]
		ip := fmt.Sprintf("%d.%d.%d.%d", rawPeerAddress[0], rawPeerAddress[1], rawPeerAddress[2], rawPeerAddress[3])
		port := binary.BigEndian.Uint16(rawPeerAddress[4:])
		peerAddresses = append(peerAddresses, fmt.Sprintf("%s:%d", ip, port))
	}

	return peerAddresses, nil
}

func LoadManifest(file string) (Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not read BitTorrent manifest: %w", err)
	}

	return parseManifest(data)
}

func parseManifest(data []byte) (Manifest, error) {
	decodedValue, err := bencode.Decode(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("could not decode BitTorrent manifest: %w", err)
	}

	dict := decodedValue.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	h := sha1.New()

	if _, err := h.Write(bencode.Encode(info)); err != nil {
		return Manifest{}, fmt.Errorf("could not create BitTorrent manifest hash: %w", err)
	}

	rawPieces := info["pieces"].(string)
	var pieceHashes [][]byte
	for i := 0; i < len(rawPieces); i += 20 {
		pieceHashes = append(pieceHashes, []byte(rawPieces[i:i+20]))
	}

	return Manifest{
		TrackerURL:  dict["announce"].(string),
		Length:      info["length"].(int),
		Hash:        h.Sum(nil),
		PieceLength: info["piece length"].(int),
		PieceHashes: pieceHashes,
	}, nil
}
