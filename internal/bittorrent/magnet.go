package bittorrent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
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
