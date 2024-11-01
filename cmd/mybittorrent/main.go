package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/cli"
)

func main() {
	if err := cli.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
