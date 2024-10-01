package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(value string) (interface{}, int, error) {
	switch {
	case unicode.IsDigit(rune(value[0])):
		return decodeString(value)
	case value[0] == 'i':
		return decodeInteger(value)
	case value[0] == 'l':
		return decodeList(value)
	case value[0] == 'd':
		return decodeDictionary(value)
	default:
		return "", 0, fmt.Errorf("Invalid payload")
	}
}

func encode(data interface{}) string {
	if str, ok := data.(string); ok {
		return fmt.Sprintf("%d:%s", len(str), str)
	}

	if integer, ok := data.(int); ok {
		return fmt.Sprintf("i%de", integer)
	}

	if list, ok := data.([]interface{}); ok {
		var encodedValue string
		for _, item := range list {
			encodedValue += encode(item)
		}
		return fmt.Sprintf("l%se", encodedValue)
	}

	if dict, ok := data.(map[string]interface{}); ok {
		var encodedValue string
		for k, v := range dict {
			encodedValue += encode(k) + encode(v)
		}
		return fmt.Sprintf("d%se", encodedValue)
	}

	return ""
}

func decodeDictionary(value string) (interface{}, int, error) {
	dict := map[string]interface{}{}
	index := 1
	var key string
	var parsedValues int

	for index < len(value) {
		if value[index] == 'e' {
			return dict, index + 1, nil
		}

		r, length, err := decodeBencode(value[index:])

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

func decodeList(value string) (interface{}, int, error) {
	list := []interface{}{}
	index := 1

	for index < len(value) {
		if value[index] == 'e' {
			return list, index + 1, nil
		}

		result, length, err := decodeBencode(value[index:])
		if err != nil {
			return nil, 0, err
		}
		list = append(list, result)
		index += length
	}

	return nil, 0, fmt.Errorf("invalid list")
}

func decodeInteger(value string) (interface{}, int, error) {
	end := strings.IndexRune(value, 'e')
	if end == -1 {
		return nil, 0, fmt.Errorf("invalid integer")
	}
	integer, err := strconv.Atoi(string(value[1:end]))
	if err != nil {
		return nil, 0, fmt.Errorf("invalid integer: %w", err)
	}

	return integer, end + 1, nil
}

func decodeString(value string) (interface{}, int, error) {
	firstColonIndex := strings.IndexRune(value, ':')
	lengthStr := value[:firstColonIndex]

	length, err := strconv.Atoi(string(lengthStr))
	if err != nil {
		return "", 0, err
	}

	return string(value[firstColonIndex+1 : firstColonIndex+1+length]), len(lengthStr) + 1 + length, nil
}

func runDecodeCommand(value string) error {
	decoded, _, err := decodeBencode(value)
	if err != nil {
		return err
	}

	jsonOutput, _ := json.Marshal(decoded)
	fmt.Println(string(jsonOutput))
	return nil
}

func runInfoCommand(file string) error {
	payload, err := os.ReadFile(file)

	if err != nil {
		return err
	}

	decodedValue, _, err := decodeBencode(string(payload))

	if err != nil {
		return err
	}

	dict := decodedValue.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	h := sha1.New()

	if _, err := h.Write([]byte(encode(info))); err != nil {
		return err
	}

	infoHash := hex.EncodeToString(h.Sum(nil))

	fmt.Print("Tracker URL: ", dict["announce"], "Length: ", info["length"], "Info Hash: ", infoHash)
	return nil
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		if err := runDecodeCommand(os.Args[2]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "info":
		if err := runInfoCommand(os.Args[2]); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
