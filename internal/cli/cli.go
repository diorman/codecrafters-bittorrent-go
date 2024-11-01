package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/internal/peer"
	"github.com/codecrafters-io/bittorrent-starter-go/internal/torrent"
)

var commands = map[string]func([]string) error{
	"decode": func(args []string) error {
		obj, err := bencode.Decode([]byte(args[2]))
		if err != nil {
			return err
		}

		jsonOutput, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		fmt.Println(string(jsonOutput))

		return nil
	},

	"info": func(args []string) error {
		t, err := torrent.FromFile(args[2])
		if err != nil {
			return err
		}

		fmt.Printf("Tracker URL: %s\n", t.TrackerURL)
		fmt.Printf("Length: %d\n", t.Length)
		fmt.Printf("Info Hash: %s\n", hex.EncodeToString(t.Hash[:]))
		fmt.Printf("Piece Length: %d\n", t.PieceLength)
		fmt.Println("Piece Hashes:")
		for _, hash := range t.PieceHashes {
			fmt.Println(hex.EncodeToString(hash[:]))
		}

		return nil
	},

	"peers": func(args []string) error {
		t, err := torrent.FromFile(args[2])
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(t.TrackerURL, t.Hash, t.Length)
		if err != nil {
			return err
		}

		for _, peerAddress := range peerAddresses {
			fmt.Println(peerAddress)
		}

		return nil
	},

	"handshake": func(args []string) error {
		inputFile := args[2]
		peerAddress := args[3]

		t, err := torrent.FromFile(inputFile)
		if err != nil {
			return err
		}

		client, err := peer.NewClient(peerAddress)
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.Handshake(t.Hash); err != nil {
			return err
		}

		peerID := client.PeerID()
		fmt.Println("Peer ID:", hex.EncodeToString(peerID[:]))

		return nil
	},

	"download_piece": func(args []string) error {
		outputFile := args[3]
		inputFile := args[4]

		pieceIndex, err := strconv.Atoi(args[5])
		if err != nil {
			return err
		}

		t, err := torrent.FromFile(inputFile)
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(t.TrackerURL, t.Hash, t.Length)
		if err != nil {
			return err
		}

		clients, err := peer.NewClients(peerAddresses)
		if err != nil {
			return err
		}
		defer clients.Close()

		if err := clients.Handshake(t.Hash); err != nil {
			return err
		}

		if err := clients.Unchoke(); err != nil {
			return err
		}

		data, err := t.DownloadPiece(clients, pieceIndex)
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
	},

	"download": func(args []string) error {
		outputFile := args[3]
		inputFile := args[4]

		t, err := torrent.FromFile(inputFile)
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(t.TrackerURL, t.Hash, t.Length)
		if err != nil {
			return err
		}

		clients, err := peer.NewClients(peerAddresses)
		if err != nil {
			return err
		}
		defer clients.Close()

		if err := clients.Handshake(t.Hash); err != nil {
			return err
		}

		if err := clients.Unchoke(); err != nil {
			return err
		}

		data, err := t.Download(clients)
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
	},

	"magnet_parse": func(args []string) error {
		ml, err := torrent.ParseMagnetLink(args[2])
		if err != nil {
			return err
		}

		fmt.Printf("Tracker URL: %v\n", ml.TrackerURL)
		fmt.Printf("Info Hash: %s\n", hex.EncodeToString(ml.Hash[:]))
		return nil
	},

	"magnet_handshake": func(args []string) error {
		ml, err := torrent.ParseMagnetLink(args[2])
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(ml.TrackerURL, ml.Hash, 1)
		if err != nil {
			return err
		}

		client, err := peer.NewClient(peerAddresses[0])
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.HandshakeWithMetadataExtension(ml.Hash); err != nil {
			return err
		}

		peerID := client.PeerID()
		fmt.Printf("Peer ID: %v\n", hex.EncodeToString(peerID[:]))
		fmt.Printf("Peer Metadata Extension ID: %v\n", client.MetadataExtensionID())

		return nil
	},

	"magnet_info": func(args []string) error {
		ml, err := torrent.ParseMagnetLink(args[2])
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(ml.TrackerURL, ml.Hash, 1)
		if err != nil {
			return err
		}

		client, err := peer.NewClient(peerAddresses[0])
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.HandshakeWithMetadataExtension(ml.Hash); err != nil {
			return err
		}

		metadata, err := client.RequestMetadata()
		if err != nil {
			return err
		}

		fmt.Printf("Tracker URL: %s\n", ml.TrackerURL)
		fmt.Printf("Length: %d\n", metadata.Length)
		fmt.Printf("Info Hash: %s\n", hex.EncodeToString(ml.Hash[:]))
		fmt.Printf("Piece Length: %d\n", metadata.PieceLength)
		fmt.Println("Piece Hashes:")
		for _, hash := range metadata.PieceHashes {
			fmt.Println(hex.EncodeToString(hash[:]))
		}

		return nil
	},

	"magnet_download_piece": func(args []string) error {
		outputFile := args[3]

		pieceIndex, err := strconv.Atoi(args[5])
		if err != nil {
			return err
		}

		ml, err := torrent.ParseMagnetLink(args[4])
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(ml.TrackerURL, ml.Hash, 1)
		if err != nil {
			return err
		}

		clients, err := peer.NewClients(peerAddresses)
		if err != nil {
			return err
		}
		defer clients.Close()

		if err := clients.HandshakeWithMetadataExtension(ml.Hash); err != nil {
			return err
		}

		metadata, err := clients[0].RequestMetadata()
		if err != nil {
			return err
		}

		t := torrent.Torrent{
			TrackerURL:  ml.TrackerURL,
			Hash:        ml.Hash,
			Length:      metadata.Length,
			PieceLength: metadata.PieceLength,
			PieceHashes: metadata.PieceHashes,
		}

		if err := clients.Unchoke(); err != nil {
			return err
		}

		data, err := t.DownloadPiece(clients, pieceIndex)
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
	},

	"magnet_download": func(args []string) error {
		outputFile := args[3]

		ml, err := torrent.ParseMagnetLink(args[4])
		if err != nil {
			return err
		}

		peerAddresses, err := peer.FetchAddresses(ml.TrackerURL, ml.Hash, 1)
		if err != nil {
			return err
		}

		clients, err := peer.NewClients(peerAddresses)
		if err != nil {
			return err
		}
		defer clients.Close()

		if err := clients.HandshakeWithMetadataExtension(ml.Hash); err != nil {
			return err
		}

		metadata, err := clients[0].RequestMetadata()
		if err != nil {
			return err
		}

		t := torrent.Torrent{
			TrackerURL:  ml.TrackerURL,
			Hash:        ml.Hash,
			Length:      metadata.Length,
			PieceLength: metadata.PieceLength,
			PieceHashes: metadata.PieceHashes,
		}

		if err := clients.Unchoke(); err != nil {
			return err
		}

		data, err := t.Download(clients)
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
	},
}

func Run(args []string) error {
	cmdKey := os.Args[1]
	if commands[cmdKey] == nil {
		return fmt.Errorf("unknown command: %v", cmdKey)
	}
	return commands[cmdKey](os.Args)
}
