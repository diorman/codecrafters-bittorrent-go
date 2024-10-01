package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	switch {
	case unicode.IsDigit(rune(bencodedString[0])):
		return decodeString(bencodedString)
	case bencodedString[0] == 'i':
		return decodeInteger(bencodedString)
	default:
		return "", fmt.Errorf("Only strings are supported at the moment")
	}
}

func decodeInteger(bencodedString string) (interface{}, error) {
	var rawInteger []byte

	for i := 1; i < len(bencodedString); i++ {
		if (i == 1 && bencodedString[i] == '-') || unicode.IsDigit(rune(bencodedString[i])) {
			rawInteger = append(rawInteger, bencodedString[i])
			continue
		}

		if bencodedString[i] == 'e' {
			return strconv.Atoi(string(rawInteger))
		}

		return nil, fmt.Errorf("invalid integer")
	}

	return nil, fmt.Errorf("invalid integer")
}

func decodeString(bencodedString string) (interface{}, error) {
	var firstColonIndex int

	for i := 0; i < len(bencodedString); i++ {
		if bencodedString[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := bencodedString[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", err
	}

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
