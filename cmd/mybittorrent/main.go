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
	case bencodedString[0] == 'd':
		return decodeDictionary(bencodedString)
	default:
		return "", 0, fmt.Errorf("Invalid payload")
	}
}

func decodeDictionary(bencodedString string) (interface{}, int, error) {
	dict := map[string]interface{}{}
	index := 1
	var key string
	var parsedValues int

	for index < len(bencodedString) {
		if bencodedString[index] == 'e' {
			return dict, index + 1, nil
		}

		r, length, err := decodeBencode(bencodedString[index:])

		if err != nil {
			return nil, 0, err
		}

		index += length

		if parsedValues%2 != 0 {
			dict[key] = r
		} else if k, ok := r.(string); ok {
			key = k
		} else {
			return nil, 0, fmt.Errorf("invalid key")
		}
		parsedValues++
	}

	return nil, 0, fmt.Errorf("invalid dictionary")
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
	end := strings.Index(bencodedString, "e")
	if end == -1 {
		return nil, 0, fmt.Errorf("invalid integer")
	}
	integer, err := strconv.Atoi(bencodedString[1:end])
	if err != nil {
		return nil, 0, fmt.Errorf("invalid integer: %w", err)
	}

	return integer, end + 1, nil
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
