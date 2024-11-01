package peer

import (
	"errors"
	"fmt"
	"net"
)

type Client struct {
	conn                   net.Conn
	peerID                 [20]byte
	withExtensionSupport   bool
	metadataExtensionID    byte
	bitfieldMessageWasRead bool
}

func (c *Client) PeerID() [20]byte {
	return c.peerID
}

func (c *Client) MetadataExtensionID() byte {
	return c.metadataExtensionID
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Handshake(hash [20]byte) error {
	return c.handshake(hash, false)
}

func (c *Client) HandshakeWithMetadataExtension(hash [20]byte) error {
	if err := c.handshake(hash, true); err != nil {
		return err
	}

	if err := c.readBitfieldMessage(); err != nil {
		return err
	}

	if err := c.writeMessage(&extensionHandshakeMessage{metadataExtensionID: 1}); err != nil {
		return err
	}

	var msg extensionHandshakeMessage
	if err := c.readMessage(&msg); err != nil {
		return err
	}

	c.metadataExtensionID = msg.metadataExtensionID

	return nil
}

type RequestMetadataOutput struct {
	PieceLength int
	Length      int
	PieceHashes [][20]byte
}

func (c *Client) RequestMetadata() (RequestMetadataOutput, error) {
	if err := c.writeMessage(&metadataRequestMessage{metadataExtensionID: c.metadataExtensionID}); err != nil {
		return RequestMetadataOutput{}, err
	}

	var msg metadataDataMessage
	if err := c.readMessage(&msg); err != nil {
		return RequestMetadataOutput{}, err
	}

	return RequestMetadataOutput{PieceLength: msg.pieceLength, Length: msg.length, PieceHashes: msg.pieceHashes}, nil
}

func (c *Client) readBitfieldMessage() error {
	if c.bitfieldMessageWasRead {
		return nil
	}

	if err := c.readMessage(&bitfieldMessage{}); err != nil {
		return err
	}

	c.bitfieldMessageWasRead = true
	return nil
}

func (c *Client) Unchoke() error {
	if err := c.readBitfieldMessage(); err != nil {
		return err
	}

	if err := c.writeMessage(&interestedMessage{}); err != nil {
		return err
	}

	if err := c.readMessage(&unchokeMessage{}); err != nil {
		return err
	}

	return nil
}

type RequestPieceInput struct {
	Index  int
	Begin  int
	Length int
}

func (c *Client) RequestPiece(input RequestPieceInput) error {
	return c.writeMessage(&requestMessage{index: input.Index, begin: input.Begin, length: input.Length})
}

type ReadPieceOutput struct {
	Index int
	Begin int
	Data  []byte
}

func (c *Client) ReadPiece() (ReadPieceOutput, error) {
	var msg pieceMessage
	if err := c.readMessage(&msg); err != nil {
		return ReadPieceOutput{}, err
	}
	return ReadPieceOutput{Index: msg.index, Begin: msg.begin, Data: msg.data}, nil
}

func (c *Client) writeMessage(m messageWriter) error {
	return m.write(c.conn)
}

func (c *Client) readMessage(m messageReader) error {
	return m.read(c.conn)
}

func (c *Client) handshake(hash [20]byte, withExtensionSupport bool) error {
	if err := c.writeMessage(&handshakeMessage{peerID: peerID(), hash: hash, withExtensionSupport: withExtensionSupport}); err != nil {
		return err
	}

	var handshake handshakeMessage
	if err := c.readMessage(&handshake); err != nil {
		return err
	}

	c.peerID = handshake.peerID
	c.withExtensionSupport = handshake.withExtensionSupport

	if withExtensionSupport && withExtensionSupport != handshake.withExtensionSupport {
		return errors.New("client does not support extensions")
	}

	return nil
}

func NewClient(peerAddress string) (*Client, error) {
	conn, err := net.Dial("tcp", peerAddress)
	if err != nil {
		return nil, fmt.Errorf("could not connect to peer address %s: %w", peerAddress, err)
	}

	return &Client{conn: conn}, nil
}

type Clients []*Client

func (s Clients) Close() {
	for _, c := range s {
		c.Close()
	}
}

func (s Clients) Handshake(hash [20]byte) error {
	for _, c := range s {
		if err := c.Handshake(hash); err != nil {
			return err
		}
	}
	return nil
}

func (s Clients) HandshakeWithMetadataExtension(hash [20]byte) error {
	for _, c := range s {
		if err := c.HandshakeWithMetadataExtension(hash); err != nil {
			return err
		}
	}
	return nil
}

func (s Clients) Unchoke() error {
	for _, c := range s {
		if err := c.Unchoke(); err != nil {
			return err
		}
	}
	return nil
}

func NewClients(peerAddresses []string) (Clients, error) {
	clients := Clients(make([]*Client, 0, len(peerAddresses)))
	for _, address := range peerAddresses {
		client, err := NewClient(address)
		if err != nil {
			clients.Close()
			return nil, err
		}
		clients = append(clients, client)
	}
	return clients, nil
}
