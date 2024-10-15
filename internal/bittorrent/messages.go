package bittorrent

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type HandshakeMessage struct {
	Hash   []byte
	PeerID []byte
}

func (m *HandshakeMessage) bytes() []byte {
	var b [68]byte
	b[0] = 19
	copy(b[1:], "BitTorrent protocol")
	copy(b[28:], m.Hash)
	copy(b[48:], m.PeerID)
	return b[:]
}

func (m *HandshakeMessage) load(b []byte) error {
	if len(b) != 68 {
		return errors.New("invalid payload for handshake message")
	}

	m.Hash = b[28:47]
	m.PeerID = b[48:]

	return nil
}

type outgoingPeerMessage interface {
	id() byte
	bytes() []byte
}

type incomingPeerMessage interface {
	id() byte
	load([]byte) error
}

type bitfieldMessage struct{}

func (m *bitfieldMessage) id() byte {
	return 5
}

func (m *bitfieldMessage) load(b []byte) error {
	return nil
}

type interestedMessage struct{}

func (m *interestedMessage) id() byte {
	return 2
}

func (m *interestedMessage) bytes() []byte {
	return nil
}

type unchokeMessage struct{}

func (m *unchokeMessage) id() byte {
	return 1
}

func (m *unchokeMessage) load(b []byte) error {
	if len(b) > 0 {
		return errors.New("unchoke message is not expected to have payload")
	}
	return nil
}

type requestMessage struct {
	index  uint32
	begin  uint32
	length uint32
}

func (m *requestMessage) id() byte {
	return 6
}

func (m *requestMessage) bytes() []byte {
	var buf [12]byte
	binary.BigEndian.PutUint32(buf[0:], m.index)
	binary.BigEndian.PutUint32(buf[4:], m.begin)
	binary.BigEndian.PutUint32(buf[8:], m.length)
	return buf[:]
}

type pieceMessage struct {
	index uint32
	begin uint32
	block []byte
}

func (m *pieceMessage) id() byte {
	return 7
}

func (m *pieceMessage) load(b []byte) error {
	if len(b) < 8 {
		return errors.New("piece message payload must be at least 8 bytes long")
	}
	m.index = binary.BigEndian.Uint32(b)
	m.begin = binary.BigEndian.Uint32(b[4:])
	if len(b) > 8 {
		m.block = b[8:]
	}
	return nil
}

type messageEnvelope struct {
	messageID byte
	payload   []byte
}

func (e messageEnvelope) bytes() []byte {
	length := len(e.payload) + 1
	buf := make([]byte, length+4)
	binary.BigEndian.PutUint32(buf, uint32(length))
	buf[4] = e.messageID
	for i, b := range e.payload {
		buf[i+5] = b
	}
	return buf
}

func (e messageEnvelope) coerceInto(msg incomingPeerMessage) error {
	if e.messageID != msg.id() {
		return fmt.Errorf("could not coerce envelop with message id %d to message id %d", e.messageID, msg.id())
	}
	return msg.load(e.payload)
}
