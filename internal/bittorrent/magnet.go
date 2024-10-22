package bittorrent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

type MagnetLink struct {
	PeerID     [20]byte
	Hash       [20]byte
	TrackerURL string
	Filename   string
}

func (m MagnetLink) PeerAddresses() ([]string, error) {
	return peerAddresses(m.TrackerURL, m.PeerID, m.Hash, 1)
}

func (m MagnetLink) Handshake(client *PeerClient) error {
	// if err := client.writeMessage(peerMessage{id: bitfieldMessageID}); err != nil {
	//   return err
	// }

	if _, err := client.readAndValidatePeerMessage(validateBitfieldMessage); err != nil {
		return err
	}

	if !client.extensionSupport {
		return errors.New("client does not support extensions")
	}

	x := bencode.Encode(map[string]interface{}{"m": map[string]interface{}{"ut_metadata": 1}})

	p := make([]byte, 0, len(x)+1)
	p = append(p, 0)
	p = append(p, x...)

	if err := client.writeMessage(peerMessage{id: 20, payload: p}); err != nil {
		return err
	}

	msg, err := client.readPeerMessage()
	if err != nil {
		return err
	}

	if msg.id != 20 {
		return errors.New("unexpected extension handshake")
	}

	d, err := bencode.Decode(msg.payload[1:])
	if err != nil {
		return err
	}

	client.MetadataExtensionID = byte(d.(map[string]interface{})["m"].(map[string]interface{})["ut_metadata"].(int))

	return nil
}

func (m MagnetLink) Info(client *PeerClient) (Torrent, error) {
	x := bencode.Encode(map[string]interface{}{"msg_type": 0, "piece": 0})
	payload := make([]byte, 0, len(x)+1)
	payload = append(payload, client.MetadataExtensionID)
	payload = append(payload, x...)

	if err := client.writeMessage(peerMessage{id: 20, payload: payload}); err != nil {
		return Torrent{}, err
	}

	msg, err := client.readPeerMessage()
	if err != nil {
		return Torrent{}, err
	}

	if msg.id != 20 {
		return Torrent{}, fmt.Errorf("unexpected error id: %v", msg.id)
	}

	// if msg.payload[0] != client.MetadataExtensionID {
	// 	return MagnetInfo{}, fmt.Errorf("expected extension id %v but got %v", client.MetadataExtensionID, msg.payload[0])
	// }

	d, err := bencode.Decode(msg.payload[1:])
	if err != nil {
		return Torrent{}, err
	}

	size := d.(map[string]interface{})["total_size"].(int)

	metadata, err := bencode.Decode(msg.payload[len(msg.payload)-size:])
	if err != nil {
		return Torrent{}, err
	}

	dict := metadata.(map[string]interface{})
	rawPieceHashes := dict["pieces"].(string)
	pieceHashes := make([][20]byte, len(rawPieceHashes)/20)
	for i := range len(pieceHashes) {
		copy(pieceHashes[i][:], rawPieceHashes[i*20:])
	}

	return Torrent{
		TrackerURL: m.TrackerURL,
		// Name:        dict["name"].(string),
		PeerID:      m.PeerID,
		Hash:        m.Hash,
		PieceLength: dict["piece length"].(int),
		Length:      dict["length"].(int),
		PieceHashes: pieceHashes,
	}, nil
}

func ParseMagnetLink(rawURL string) (MagnetLink, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return MagnetLink{}, err
	}

	xt := u.Query().Get("xt")
	dn := u.Query().Get("dn")
	tr := u.Query().Get("tr")

	if !strings.HasPrefix(xt, "urn:btih:") || len(xt) != 49 {
		return MagnetLink{}, fmt.Errorf("invalid hash format: %v", xt)
	}

	var hash [20]byte
	if _, err := hex.Decode(hash[:], []byte(xt[9:])); err != nil {
		return MagnetLink{}, err
	}

	var peerID [20]byte
	if _, err := rand.Read(peerID[:]); err != nil {
		return MagnetLink{}, err
	}

	return MagnetLink{PeerID: peerID, Hash: hash, TrackerURL: tr, Filename: dn}, nil
}
