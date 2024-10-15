package bittorrent

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

type Torrent struct {
	TrackerURL  string
	Length      int
	Hash        [20]byte
	PieceLength int
	PieceHashes [][20]byte
	PeerID      [20]byte
}

func (t Torrent) PeerAddresses() ([]string, error) {
	u, err := url.Parse(t.TrackerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse BitTorrent tracker URL: %w", err)
	}

	query := u.Query()
	query.Add("info_hash", string(t.Hash[:]))
	query.Add("peer_id", string(t.PeerID[:]))
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

func (t Torrent) DownloadPiece(pieceIndex int) ([]byte, error) {
	clients, err := t.clients()
	if err != nil {
		return nil, err
	}
	defer clients.close()

	return t.downloadPiece(clients, pieceIndex)
}

func (t Torrent) Download() ([]byte, error) {
	clients, err := t.clients()
	if err != nil {
		return nil, err
	}
	defer clients.close()

	data := make([]byte, t.Length)

	for i := range len(t.PieceHashes) {
		pieceData, err := t.downloadPiece(clients, i)
		if err != nil {
			return nil, err
		}
		copy(data[i*t.PieceLength:], pieceData)
	}

	return data, nil
}

func (t Torrent) downloadPiece(clients []*PeerClient, pieceIndex int) ([]byte, error) {
	const blockMaxSize = 16 * 1024
	pieceLength := min(t.PieceLength, t.Length-t.PieceLength*pieceIndex)
	totalBlocks := int(math.Ceil(float64(pieceLength) / float64(blockMaxSize)))
	tasks := make(chan blockRequest)
	results := make(chan pieceMessagePayload, totalBlocks)
	pieceData := make([]byte, pieceLength)
	errors := make(chan error, len(clients))
	var wg sync.WaitGroup

	for _, c := range clients {
		wg.Add(1)
		go func(c *PeerClient) {
			defer wg.Done()
			if err := downloadWorker(c, tasks, results); err != nil {
				errors <- err
			}
		}(c)
	}

	for i := range totalBlocks {
		blockLength := min(blockMaxSize, pieceLength-i*blockMaxSize)

		tasks <- blockRequest{
			index:  pieceIndex,
			begin:  i * blockMaxSize,
			length: blockLength,
		}
	}
	close(tasks)
	wg.Wait()

	select {
	case err := <-errors:
		return nil, err
	default:
	}

	for range totalBlocks {
		block := <-results
		copy(pieceData[block.begin():], block.data())
	}

	hash := sha1.Sum(pieceData)
	if !bytes.Equal(hash[:], t.PieceHashes[pieceIndex][:]) {
		return nil, fmt.Errorf("could not check integrity of piece %v", pieceIndex)
	}

	return pieceData, nil
}

func CreateFromFile(file string) (Torrent, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Torrent{}, fmt.Errorf("could not read BitTorrent manifest: %w", err)
	}

	torrent, err := parseTorrentData(data)
	if err != nil {
		return Torrent{}, err
	}

	if _, err := rand.Read(torrent.PeerID[:]); err != nil {
		return Torrent{}, err
	}

	return torrent, nil
}

func (t *Torrent) clients() (clientList, error) {
	peerAddresses, err := t.PeerAddresses()
	if err != nil {
		return nil, err
	}

	clients := make(clientList, 0, len(peerAddresses))
	for _, a := range peerAddresses {
		c, err := NewPeerClient(a, t.PeerID, t.Hash)
		if err != nil {
			return nil, err
		}

		if err := c.prepare(); err != nil {
			return nil, err
		}

		clients = append(clients, c)
	}

	return clients, nil
}

type blockRequest struct {
	index  int
	begin  int
	length int
}

func parseTorrentData(data []byte) (Torrent, error) {
	decodedValue, err := bencode.Decode(data)
	if err != nil {
		return Torrent{}, fmt.Errorf("could not decode BitTorrent data: %w", err)
	}

	dict := decodedValue.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	h := sha1.New()

	if _, err := h.Write(bencode.Encode(info)); err != nil {
		return Torrent{}, fmt.Errorf("could not create BitTorrent hash: %w", err)
	}

	rawPieces := info["pieces"].(string)
	pieceHashes := make([][20]byte, len(rawPieces)/20)
	for i := range pieceHashes {
		copy(pieceHashes[i][:], rawPieces[i*20:])
	}

	return Torrent{
		TrackerURL:  dict["announce"].(string),
		Length:      info["length"].(int),
		Hash:        [20]byte(h.Sum(nil)),
		PieceLength: info["piece length"].(int),
		PieceHashes: pieceHashes,
	}, nil
}

func downloadWorker(c *PeerClient, tasks <-chan blockRequest, results chan<- pieceMessagePayload) error {
	requestsInFlight := 0
	tasksClosed := false

	for requestsInFlight > 0 || !tasksClosed {
		if !tasksClosed && requestsInFlight < 5 {
			b, ok := <-tasks
			if !ok {
				tasksClosed = true
				continue
			}

			if err := c.writeMessage(createRequestMessage(b.index, b.begin, b.length)); err != nil {
				return err
			}

			requestsInFlight++
			continue
		}

		if requestsInFlight > 0 {
			m, err := c.readAndValidatePeerMessage(validatePieceMessage)
			if err != nil {
				return err
			}

			requestsInFlight--
			results <- pieceMessagePayload(m.payload)
		}
	}

	return nil
}
