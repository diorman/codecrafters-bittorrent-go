package torrent

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/internal/peer"
)

type Torrent struct {
	TrackerURL  string
	Length      int
	Hash        [20]byte
	PieceLength int
	PieceHashes [][20]byte
}

func (t Torrent) Download(clients peer.Clients) ([]byte, error) {
	data := make([]byte, t.Length)

	for i := range len(t.PieceHashes) {
		pieceData, err := t.DownloadPiece(clients, i)
		if err != nil {
			return nil, err
		}
		copy(data[i*t.PieceLength:], pieceData)
	}

	return data, nil
}

func (t Torrent) DownloadPiece(clients peer.Clients, pieceIndex int) ([]byte, error) {
	if pieceIndex < 0 || pieceIndex >= len(t.PieceHashes) {
		return nil, fmt.Errorf("unexpected piece index: %v", pieceIndex)
	}

	const blockMaxSize = 16 * 1024
	ctx, ctxCancel := context.WithCancelCause(context.Background())
	pieceLength := min(t.PieceLength, t.Length-t.PieceLength*pieceIndex)
	totalBlocks := int(math.Ceil(float64(pieceLength) / float64(blockMaxSize)))
	tasks := make(chan peer.RequestPieceInput, totalBlocks)
	results := make(chan peer.ReadPieceOutput, totalBlocks)
	pieceData := make([]byte, pieceLength)
	var wg sync.WaitGroup

	for _, c := range clients {
		wg.Add(1)
		go func(c *peer.Client) {
			defer wg.Done()
			if err := downloadWorker(ctx, c, tasks, results); err != nil {
				ctxCancel(err)
			}
		}(c)
	}

	for i := range totalBlocks {
		blockLength := min(blockMaxSize, pieceLength-i*blockMaxSize)
		if err := writeDownloadTask(ctx, tasks, peer.RequestPieceInput{Index: pieceIndex, Begin: i * blockMaxSize, Length: blockLength}); err != nil {
			break
		}
	}
	close(tasks)
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}

	for range totalBlocks {
		output := <-results
		copy(pieceData[output.Begin:], output.Data)
	}

	hash := sha1.Sum(pieceData)
	if !bytes.Equal(hash[:], t.PieceHashes[pieceIndex][:]) {
		return nil, fmt.Errorf("could not check integrity of piece %v", pieceIndex)
	}

	return pieceData, nil
}

func FromFile(file string) (Torrent, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Torrent{}, fmt.Errorf("could not read torrent file: %w", err)
	}

	torrent, err := parseTorrentData(data)
	if err != nil {
		return Torrent{}, err
	}

	return torrent, nil
}

func parseTorrentData(data []byte) (Torrent, error) {
	decodedValue, err := bencode.Decode(data)
	if err != nil {
		return Torrent{}, fmt.Errorf("could not decode torrent data: %w", err)
	}

	dict := decodedValue.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	h := sha1.New()

	encodedInfo, err := bencode.Encode(info)
	if err != nil {
		return Torrent{}, err
	}

	if _, err := h.Write(encodedInfo); err != nil {
		return Torrent{}, fmt.Errorf("could not create torrent hash: %w", err)
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

func writeDownloadTask(ctx context.Context, tasks chan<- peer.RequestPieceInput, task peer.RequestPieceInput) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case tasks <- task:
		return nil
	}
}

func readDownloadTask(ctx context.Context, tasks <-chan peer.RequestPieceInput) (peer.RequestPieceInput, bool, error) {
	select {
	case <-ctx.Done():
		return peer.RequestPieceInput{}, false, ctx.Err()

	case input, ok := <-tasks:
		return input, ok, nil
	}
}

func downloadWorker(ctx context.Context, c *peer.Client, tasks <-chan peer.RequestPieceInput, results chan<- peer.ReadPieceOutput) error {
	requestsInFlight := 0
	tasksClosed := false

	for requestsInFlight > 0 || !tasksClosed {
		if !tasksClosed && requestsInFlight < 5 {
			input, ok, err := readDownloadTask(ctx, tasks)
			if err != nil {
				return err
			}

			if !ok {
				tasksClosed = true
				continue
			}

			if err := c.RequestPiece(input); err != nil {
				return err
			}

			requestsInFlight++
			continue
		}

		if requestsInFlight > 0 {
			output, err := c.ReadPiece()
			if err != nil {
				return err
			}

			requestsInFlight--
			results <- output
		}
	}

	return nil
}
