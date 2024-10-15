package bittorrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type PeerClient struct {
	conn   net.Conn
	PeerID [20]byte
}

func NewPeerClient(peerAddress string, peerID, hash [20]byte) (*PeerClient, error) {
	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return nil, fmt.Errorf("could not connect to peer address %s: %w", peerAddress, err)
	}

	c := &PeerClient{conn: conn}

	if err := c.writeMessage(handshakeMessage{hash: hash, peerID: peerID}); err != nil {
		return nil, err
	}

	hsmsg, err := c.readHandshakeMessage()
	if err != nil {
		return nil, err
	}

	c.PeerID = hsmsg.peerID

	return c, nil
}

func (c *PeerClient) Close() error {
	return c.conn.Close()
}

func (c *PeerClient) prepare() error {
	if _, err := c.readAndValidatePeerMessage(validateBitfieldMessage); err != nil {
		return err
	}

	if err := c.writeMessage(peerMessage{id: interestedMessageID}); err != nil {
		return err
	}

	if _, err := c.readAndValidatePeerMessage(validateUnchokeMessage); err != nil {
		return err
	}

	return nil
}

func (c *PeerClient) writeMessage(m message) error {
	if _, err := c.conn.Write(m.marshal()); err != nil {
		return err
	}
	return nil
}

func (c *PeerClient) readAndValidatePeerMessage(validator func(peerMessage) error) (peerMessage, error) {
	msg, err := c.readPeerMessage()
	if err != nil {
		return peerMessage{}, err
	}

	if err := validator(msg); err != nil {
		return peerMessage{}, err
	}

	return msg, nil
}

func (c *PeerClient) readPeerMessage() (peerMessage, error) {
	var header [5]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		return peerMessage{}, fmt.Errorf("could not read peer message header: %w", err)
	}

	msg := peerMessage{id: messageID(header[4])}
	length := binary.BigEndian.Uint32(header[:4])

	if length > 1 {
		msg.payload = make([]byte, length-1)
		if _, err := io.ReadFull(c.conn, msg.payload); err != nil {
			return peerMessage{}, fmt.Errorf("could not read peer message payload: %w", err)
		}
	}

	return msg, nil
}

func (c *PeerClient) readHandshakeMessage() (handshakeMessage, error) {
	buf := make([]byte, 68)

	if _, err := io.ReadFull(c.conn, buf); err != nil {
		return handshakeMessage{}, fmt.Errorf("error reading handshake message: %w", err)
	}

	m := handshakeMessage{}
	copy(m.hash[:], buf[28:47])
	copy(m.peerID[:], buf[48:])

	return m, nil
}

type clientList []*PeerClient

func (list clientList) close() error {
	for _, client := range list {
		if err := client.Close(); err != nil {
			return nil
		}
	}
	return nil
}
