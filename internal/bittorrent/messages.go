package bittorrent

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type message interface {
	marshal() []byte
}

type handshakeMessage struct {
	hash   [20]byte
	peerID [20]byte
}

func (m handshakeMessage) marshal() []byte {
	var b [68]byte
	b[0] = 19
	copy(b[1:], "BitTorrent protocol")
	copy(b[28:], m.hash[:])
	copy(b[48:], m.peerID[:])
	return b[:]
}

type messageID uint8

const (
	unchokeMessageID    messageID = 1
	interestedMessageID messageID = 2
	bitfieldMessageID   messageID = 5
	requestMessageID    messageID = 6
	pieceMessageID      messageID = 7
)

type peerMessage struct {
	id      messageID
	payload []byte
}

func (m peerMessage) marshal() []byte {
	length := len(m.payload) + 1
	buf := make([]byte, length+4)
	binary.BigEndian.PutUint32(buf, uint32(length))
	buf[4] = byte(m.id)
	for i, b := range m.payload {
		buf[i+5] = b
	}
	return buf
}

func validateMessageID(m peerMessage, id messageID) error {
	if m.id != id {
		return fmt.Errorf("expected message id %v but got %v", id, m.id)
	}
	return nil
}

func validateBitfieldMessage(m peerMessage) error {
	if err := validateMessageID(m, bitfieldMessageID); err != nil {
		return err
	}
	return nil
}

func validateUnchokeMessage(m peerMessage) error {
	if err := validateMessageID(m, unchokeMessageID); err != nil {
		return err
	}

	if len(m.payload) > 0 {
		return errors.New("received unchoke message with unexpected payload")
	}

	return nil
}

func createRequestMessage(index, begin, length int) peerMessage {
	var payload [12]byte
	binary.BigEndian.PutUint32(payload[0:], uint32(index))
	binary.BigEndian.PutUint32(payload[4:], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:], uint32(length))
	return peerMessage{id: requestMessageID, payload: payload[:]}
}

func validatePieceMessage(m peerMessage) error {
	if err := validateMessageID(m, pieceMessageID); err != nil {
		return err
	}

	if len(m.payload) < 8 {
		return errors.New("piece message payload must be at least 8 bytes long")
	}

	return nil
}

type pieceMessagePayload []byte

func (p pieceMessagePayload) index() int {
	return int(binary.BigEndian.Uint32(p))
}

func (p pieceMessagePayload) begin() int {
	return int(binary.BigEndian.Uint32(p[4:]))
}

func (p pieceMessagePayload) data() []byte {
	return p[8:]
}
