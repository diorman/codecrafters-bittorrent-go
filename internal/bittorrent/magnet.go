package bittorrent

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

type MagnetLink struct {
	Hash       [20]byte
	TrackerURL string
	Filename   string
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

	return MagnetLink{Hash: hash, TrackerURL: tr, Filename: dn}, nil
}
