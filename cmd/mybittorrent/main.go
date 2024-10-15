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
	torrent, err := bittorrent.LoadManifest(args[2])
	if err != nil {
		return err
	}

	fmt.Printf("Tracker URL: %s\n", torrent.TrackerURL)
	fmt.Printf("Length: %d\n", torrent.Length)
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(torrent.Hash))
	fmt.Printf("Piece Length: %d\n", torrent.PieceLength)
	fmt.Println("Piece Hashes:")
	for _, hash := range torrent.PieceHashes {
		fmt.Println(hex.EncodeToString(hash))
	}

	return nil
}

func runPeersCommand(args []string) error {
	manifest, err := bittorrent.LoadManifest(args[2])
	if err != nil {
		return err
	}

	peerAddresses, err := manifest.PeerAddresses()
	if err != nil {
		return err
	}

	for _, peerAddress := range peerAddresses {
		fmt.Println(peerAddress)
	}

	return nil
}

func runHandshakeCommand(args []string) error {
	manifest, err := bittorrent.LoadManifest(args[2])
	if err != nil {
		return err
	}

	client, err := bittorrent.NewPeerClient(manifest, args[3])
	if err != nil {
		return err
	}
	defer client.Close()

	msg, err := client.Handshake()
	if err != nil {
		return err
	}

	fmt.Println("Peer ID:", hex.EncodeToString(msg.PeerID))

	return nil
}

func runDownloadPieceCommand(args []string) error {
	outputFile := args[3]
	inputFile := args[4]

	pieceIndex, err := strconv.Atoi(args[5])
	if err != nil {
		return err
	}

	manifest, err := bittorrent.LoadManifest(inputFile)
	if err != nil {
		return err
	}

	peerAddresses, err := manifest.PeerAddresses()
	if err != nil {
		return err
	}

	c, err := bittorrent.NewPeerClient(manifest, peerAddresses[0])
	if err != nil {
		return err
	}
	defer c.Close()

	data, err := c.DownloadPiece(pieceIndex)
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
