package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, int, error) {
	switch {
	case unicode.IsDigit(rune(bencodedString[0])):
		return decodeString(bencodedString)
	case bencodedString[0] == 'i':
		return decodeInteger(bencodedString)
	case bencodedString[0] == 'l':
		return decodeList(bencodedString)
	default:
		return "", 0, fmt.Errorf("Invalid payload")
	}
}

func decodeList(bencodedString string) (interface{}, int, error) {
	list := []interface{}{}
	index := 1

	for index < len(bencodedString) {
		if bencodedString[index] == 'e' {
			return list, index + 1, nil
		}

		result, length, err := decodeBencode(bencodedString[index:])
		if err != nil {
			return nil, 0, err
		}
		list = append(list, result)
		index += length
	}

	return nil, 0, fmt.Errorf("invalid list")
}

func decodeInteger(bencodedString string) (interface{}, int, error) {
	var rawInteger []byte

	for i := 1; i < len(bencodedString); i++ {
		if (i == 1 && bencodedString[i] == '-') || unicode.IsDigit(rune(bencodedString[i])) {
			rawInteger = append(rawInteger, bencodedString[i])
			continue
		}

		if bencodedString[i] == 'e' {
			integer, err := strconv.Atoi(string(rawInteger))

			if err != nil {
				return nil, 0, err
			}

			return integer, len(rawInteger) + 2, nil
		}

		break
	}

	return nil, 0, fmt.Errorf("invalid integer")
}

func decodeString(bencodedString string) (interface{}, int, error) {
	firstColonIndex := strings.Index(bencodedString, ":")
	lengthStr := bencodedString[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, err
	}

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], len(lengthStr) + 1 + length, nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, _, err := decodeBencode(bencodedValue)
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
