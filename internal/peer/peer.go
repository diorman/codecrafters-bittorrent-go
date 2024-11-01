package peer

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

var peerID = sync.OnceValue(func() [20]byte {
	var peerID [20]byte
	if _, err := rand.Read(peerID[:]); err != nil {
		panic(err)
	}
	return peerID
})

func FetchAddresses(trackerURL string, hash [20]byte, left int) ([]string, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse torrent tracker URL: %w", err)
	}

	peerID := peerID()
	query := u.Query()
	query.Add("info_hash", string(hash[:]))
	query.Add("peer_id", string(peerID[:]))
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(left))
	query.Add("compact", "1")
	u.RawQuery = query.Encode()

	r, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("could not request torrent tracker URL: %w", err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read torrent tracker response: %w", err)
	}
	defer r.Body.Close()

	obj, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("could not decode torrent tracker response: %w", err)
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
