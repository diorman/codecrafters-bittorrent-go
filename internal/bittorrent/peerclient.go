package bittorrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
)

type PeerClient struct {
	conn   net.Conn
	PeerID [20]byte
}

func NewPeerClient(peerAddress string, peerID, hash [20]byte, extensionSupport bool) (*PeerClient, error) {
	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return nil, fmt.Errorf("could not connect to peer address %s: %w", peerAddress, err)
	}

	c := &PeerClient{conn: conn}

	if err := c.writeMessage(handshakeMessage{hash: hash, peerID: peerID, extensionSupport: extensionSupport}); err != nil {
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

func (c *PeerClient) prepareForDownload() error {
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

func peerAddresses(trackerURL string, peerID, hash [20]byte, left int) ([]string, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse BitTorrent tracker URL: %w", err)
	}

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
