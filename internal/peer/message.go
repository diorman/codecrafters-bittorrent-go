package peer

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

type message interface {
	marshal() []byte
}

type messageWriter interface {
	write(io.Writer) error
}

type messageReader interface {
	read(io.Reader) error
}

type handshakeMessage struct {
	hash                 [20]byte
	peerID               [20]byte
	withExtensionSupport bool
}

func (m *handshakeMessage) write(w io.Writer) error {
	var buf [68]byte
	buf[0] = 19
	copy(buf[1:], "BitTorrent protocol")
	copy(buf[28:], m.hash[:])
	copy(buf[48:], m.peerID[:])

	if m.withExtensionSupport {
		buf[25] = 16
	}

	if _, err := w.Write(buf[:]); err != nil {
		return err
	}

	return nil
}

func (m *handshakeMessage) read(r io.Reader) error {
	buf := make([]byte, 68)

	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("error reading handshake message: %w", err)
	}

	m.hash = [20]byte(buf[28:48])
	m.peerID = [20]byte(buf[48:])
	m.withExtensionSupport = buf[25] == 16

	return nil
}

type bitfieldMessage struct{}

func (m *bitfieldMessage) read(r io.Reader) error {
	var pm peerMessage
	if err := pm.read(r); err != nil {
		return err
	}
	return verifyMessageID(pm, 5)
}

type interestedMessage struct{}

func (m *interestedMessage) write(w io.Writer) error {
	pm := peerMessage{id: 2}
	return pm.write(w)
}

type unchokeMessage struct{}

func (m *unchokeMessage) read(r io.Reader) error {
	var pm peerMessage
	if err := pm.read(r); err != nil {
		return err
	}
	return verifyMessageID(pm, 1)
}

type extensionHandshakeMessage struct {
	metadataExtensionID byte
}

func (m *extensionHandshakeMessage) write(w io.Writer) error {
	dictEncoded, err := bencode.Encode(map[string]interface{}{"m": map[string]interface{}{"ut_metadata": int(m.metadataExtensionID)}})
	if err != nil {
		return err
	}
	payload := make([]byte, len(dictEncoded)+1)
	payload[0] = 0
	copy(payload[1:], dictEncoded)

	pm := peerMessage{id: 20, payload: payload}
	return pm.write(w)
}

func (m *extensionHandshakeMessage) read(r io.Reader) error {
	var pm peerMessage
	if err := pm.read(r); err != nil {
		return err
	}

	if err := verifyMessageID(pm, 20); err != nil {
		return err
	}

	if pm.payload[0] != 0 {
		return fmt.Errorf("unexpected extension message id: %v", pm.payload[0])
	}

	p, err := bencode.Decode(pm.payload[1:])
	if err != nil {
		return err
	}

	m.metadataExtensionID = byte(p.(map[string]interface{})["m"].(map[string]interface{})["ut_metadata"].(int))

	return nil
}

type requestMessage struct {
	index  int
	begin  int
	length int
}

func (m *requestMessage) write(w io.Writer) error {
	var payload [12]byte
	binary.BigEndian.PutUint32(payload[0:], uint32(m.index))
	binary.BigEndian.PutUint32(payload[4:], uint32(m.begin))
	binary.BigEndian.PutUint32(payload[8:], uint32(m.length))
	pm := peerMessage{id: 6, payload: payload[:]}
	return pm.write(w)
}

type pieceMessage struct {
	index int
	begin int
	data  []byte
}

func (m *pieceMessage) read(r io.Reader) error {
	var pm peerMessage
	if err := pm.read(r); err != nil {
		return err
	}

	if err := verifyMessageID(pm, 7); err != nil {
		return err
	}

	m.index = int(binary.BigEndian.Uint32(pm.payload))
	m.begin = int(binary.BigEndian.Uint32(pm.payload[4:]))
	m.data = pm.payload[8:]

	return nil
}

type metadataRequestMessage struct {
	metadataExtensionID byte
}

func (m *metadataRequestMessage) write(w io.Writer) error {
	dictEncoded, err := bencode.Encode(map[string]interface{}{"msg_type": 0, "piece": 0})
	if err != nil {
		return err
	}
	payload := make([]byte, len(dictEncoded)+1)
	payload[0] = m.metadataExtensionID
	copy(payload[1:], dictEncoded)

	pm := peerMessage{id: 20, payload: payload}
	return pm.write(w)
}

type metadataDataMessage struct {
	metadataExtensionID byte
	pieceLength         int
	pieceHashes         [][20]byte
	length              int
	name                string
}

func (m *metadataDataMessage) read(r io.Reader) error {
	var pm peerMessage
	if err := pm.read(r); err != nil {
		return err
	}

	if err := verifyMessageID(pm, 20); err != nil {
		return err
	}

	p, err := bencode.Decode(pm.payload[1:])
	if err != nil {
		return err
	}

	size := p.(map[string]interface{})["total_size"].(int)

	metadata, err := bencode.Decode(pm.payload[len(pm.payload)-size:])
	if err != nil {
		return err
	}

	dict := metadata.(map[string]interface{})
	rawPieceHashes := dict["pieces"].(string)
	pieceHashes := make([][20]byte, len(rawPieceHashes)/20)
	for i := range len(pieceHashes) {
		copy(pieceHashes[i][:], rawPieceHashes[i*20:])
	}

	m.pieceLength = dict["piece length"].(int)
	m.length = dict["length"].(int)
	m.pieceHashes = pieceHashes
	m.name = dict["name"].(string)
	m.metadataExtensionID = pm.payload[0]

	return nil
}

type peerMessage struct {
	id      byte
	payload []byte
}

func (m *peerMessage) write(w io.Writer) error {
	length := len(m.payload) + 1
	buf := make([]byte, length+4)
	binary.BigEndian.PutUint32(buf, uint32(length))
	buf[4] = byte(m.id)
	for i, b := range m.payload {
		buf[i+5] = b
	}

	if _, err := w.Write(buf); err != nil {
		return err
	}

	return nil
}

func (m *peerMessage) read(r io.Reader) error {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return fmt.Errorf("could not read peer message header: %w", err)
	}

	m.id = header[4]
	length := binary.BigEndian.Uint32(header[:4])

	if length > 1 {
		m.payload = make([]byte, length-1)
		if _, err := io.ReadFull(r, m.payload); err != nil {
			return fmt.Errorf("could not read peer message payload: %w", err)
		}
	}

	return nil
}

func verifyMessageID(m peerMessage, id byte) error {
	if m.id != id {
		return fmt.Errorf("expected message id %v but got %v", id, m.id)
	}
	return nil
}
