package bittorrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
)

type PeerClient struct {
	conn     net.Conn
	manifest Manifest
}

func NewPeerClient(manifest Manifest, peerAddress string) (*PeerClient, error) {
	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return nil, fmt.Errorf("could not connect to peer address %s: %w", peerAddress, err)
	}

	c := &PeerClient{conn: conn, manifest: manifest}

	return c, nil
}

func (c *PeerClient) Handshake() (HandshakeMessage, error) {
	if err := c.sendHandshakeMessage(); err != nil {
		return HandshakeMessage{}, err
	}

	msg, err := c.receiveHandshakeMessage()
	if err != nil {
		return HandshakeMessage{}, err
	}

	return msg, nil
}

func (c *PeerClient) Close() error {
	return c.conn.Close()
}

func (c *PeerClient) sendPeerMessage(msg outgoingPeerMessage) error {
	envelope := messageEnvelope{messageID: msg.id(), payload: msg.bytes()}
	if _, err := c.conn.Write(envelope.bytes()); err != nil {
		return err
	}
	return nil
}

func (c *PeerClient) receivePeerMessage(msg incomingPeerMessage) error {
	var header [5]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		return fmt.Errorf("could not read peer message header: %w", err)
	}

	messageID := header[4]
	envelope := messageEnvelope{messageID: messageID}
	length := binary.BigEndian.Uint32(header[:4])

	if length > 1 {
		envelope.payload = make([]byte, length-1)
		if _, err := io.ReadFull(c.conn, envelope.payload); err != nil {
			return fmt.Errorf("could not read peer message payload: %w", err)
		}
	}

	return envelope.coerceInto(msg)
}

const blockMaxSize = 16 * 1024

func (c *PeerClient) DownloadPiece(pieceIndex int) ([]byte, error) {
	if _, err := c.Handshake(); err != nil {
		return nil, err
	}

	if err := c.receivePeerMessage(&bitfieldMessage{}); err != nil {
		return nil, err
	}

	if err := c.sendPeerMessage(&interestedMessage{}); err != nil {
		return nil, err
	}

	if err := c.receivePeerMessage(&unchokeMessage{}); err != nil {
		return nil, err
	}

	pieceLength := min(c.manifest.PieceLength, c.manifest.Length-c.manifest.PieceLength*pieceIndex)
	n := int(math.Round(float64(pieceLength) / float64(blockMaxSize)))
	data := make([]byte, pieceLength)

	for i := 0; i < n; i++ {
		blockLength := min(blockMaxSize, pieceLength-i*blockMaxSize)
		request := requestMessage{
			index:  uint32(pieceIndex),
			begin:  uint32(i * blockMaxSize),
			length: uint32(blockLength),
		}

		if err := c.sendPeerMessage(&request); err != nil {
			return nil, err
		}

		var piece pieceMessage
		if err := c.receivePeerMessage(&piece); err != nil {
			return nil, err
		}

		copy(data[piece.begin:], piece.block)
	}

	return data, nil
}

func (c *PeerClient) sendHandshakeMessage() error {
	msg := HandshakeMessage{Hash: c.manifest.Hash, PeerID: []byte("00112233445566778899")}

	if _, err := c.conn.Write(msg.bytes()); err != nil {
		return fmt.Errorf("error sending handshake message: %w", err)
	}

	return nil
}

func (c *PeerClient) receiveHandshakeMessage() (HandshakeMessage, error) {
	buf := make([]byte, 68)
	var msg HandshakeMessage
	if _, err := io.ReadFull(c.conn, buf); err != nil {
		return msg, fmt.Errorf("error reading handshake message: %w", err)
	}
	msg.load(buf[:])
	return msg, nil
}
