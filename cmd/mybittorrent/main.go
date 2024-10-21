package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/internal/bittorrent"
)

func runDecodeCommand(args []string) error {
	obj, err := bencode.Decode([]byte(args[2]))
	if err != nil {
		return err
	}

	jsonOutput, _ := json.Marshal(obj)
	fmt.Println(string(jsonOutput))
	return nil
}

func runInfoCommand(args []string) error {
	torrent, err := bittorrent.CreateFromFile(args[2])
	if err != nil {
		return err
	}

	fmt.Printf("Tracker URL: %s\n", torrent.TrackerURL)
	fmt.Printf("Length: %d\n", torrent.Length)
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(torrent.Hash[:]))
	fmt.Printf("Piece Length: %d\n", torrent.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, hash := range torrent.PieceHashes {
		fmt.Println(hex.EncodeToString(hash[:]))
	}

	return nil
}

func runPeersCommand(args []string) error {
	torrent, err := bittorrent.CreateFromFile(args[2])
	if err != nil {
		return err
	}

	peerAddresses, err := torrent.PeerAddresses()
	if err != nil {
		return err
	}

	for _, peerAddress := range peerAddresses {
		fmt.Println(peerAddress)
	}

	return nil
}

func runHandshakeCommand(args []string) error {
	torrent, err := bittorrent.CreateFromFile(args[2])
	if err != nil {
		return err
	}

	client, err := bittorrent.NewPeerClient(args[3], torrent.PeerID, torrent.Hash, false)
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Println("Peer ID:", hex.EncodeToString(client.PeerID[:]))

	return nil
}

func runDownloadPieceCommand(args []string) error {
	outputFile := args[3]
	inputFile := args[4]

	pieceIndex, err := strconv.Atoi(args[5])
	if err != nil {
		return err
	}

	torrent, err := bittorrent.CreateFromFile(inputFile)
	if err != nil {
		return err
	}

	data, err := torrent.DownloadPiece(pieceIndex)
	if err != nil {
		return err
	}

	f, err := os.Create(outputFile)
	if err != err {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}

func runDownloadCommand(args []string) error {
	outputFile := args[3]
	inputFile := args[4]

	torrent, err := bittorrent.CreateFromFile(inputFile)
	if err != nil {
		return err
	}

	data, err := torrent.Download()
	if err != nil {
		return err
	}

	f, err := os.Create(outputFile)
	if err != err {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}

func runMagnetParseCommand(args []string) error {
	m, err := bittorrent.ParseMagnetLink(args[2])
	if err != nil {
		return err
	}

	fmt.Printf("Tracker URL: %v\n", m.TrackerURL)
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(m.Hash[:]))
	return nil
}

func runMagnetHandshakeCommand(args []string) error {
	m, err := bittorrent.ParseMagnetLink(args[2])
	if err != nil {
		return err
	}

	peerAddresses, err := m.PeerAddresses()
	if err != nil {
		return err
	}

	client, err := bittorrent.NewPeerClient(peerAddresses[0], m.PeerID, m.Hash, true)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := m.Handshake(client); err != nil {
		return err
	}

	fmt.Printf("Peer ID: %v\n", hex.EncodeToString(client.PeerID[:]))
	fmt.Printf("Peer Metadata Extension ID: %v\n", client.MetadataExtensionID)

	return nil
}

func main() {
	command := os.Args[1]
	commandFunc := (func() func([]string) error {
		switch command {
		case "decode":
			return runDecodeCommand
		case "info":
			return runInfoCommand
		case "peers":
			return runPeersCommand
		case "handshake":
			return runHandshakeCommand
		case "download_piece":
			return runDownloadPieceCommand
		case "download":
			return runDownloadCommand
		case "magnet_parse":
			return runMagnetParseCommand
		case "magnet_handshake":
			return runMagnetHandshakeCommand
		default:
			return nil
		}
	})()

	if commandFunc == nil {
		fmt.Println("unknown command")
		os.Exit(1)
	}

	if err := commandFunc(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
