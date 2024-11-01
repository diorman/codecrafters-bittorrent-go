package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"unicode"
)

func Encode(data interface{}) ([]byte, error) {
	switch value := data.(type) {
	case string:
		return []byte(fmt.Sprintf("%d:%s", len(value), value)), nil

	case int:
		return []byte(fmt.Sprintf("i%de", value)), nil

	case []interface{}:
		var encodedValue []byte
		for _, item := range value {
			encodedItem, err := Encode(item)
			if err != nil {
				return nil, err
			}
			encodedValue = append(encodedValue, encodedItem...)
		}
		return []byte(fmt.Sprintf("l%se", encodedValue)), nil

	case map[string]interface{}:
		var keys []string
		for k := range value {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		var encodedValue []byte
		for _, k := range keys {
			encodedKey, err := Encode(k)
			if err != nil {
				return nil, err
			}

			encodedVal, err := Encode(value[k])
			if err != nil {
				return nil, err
			}

			encodedValue = append(encodedValue, encodedKey...)
			encodedValue = append(encodedValue, encodedVal...)
		}
		return []byte(fmt.Sprintf("d%se", encodedValue)), nil

	default:
		return nil, fmt.Errorf("could not bencode encode unsupported type: %T", value)
	}
}

func Decode(value []byte) (interface{}, error) {
	reader := bufio.NewReader(bytes.NewReader(value))

	obj, err := decode(reader)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func decode(reader *bufio.Reader) (interface{}, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch {
	case unicode.IsDigit(rune(b)):
		if err := reader.UnreadByte(); err != nil {
			return nil, err
		}
		return decodeString(reader)
	case b == 'i':
		return decodeInteger(reader)
	case b == 'l':
		return decodeList(reader)
	case b == 'd':
		return decodeDictionary(reader)
	default:
		return "", errors.New("invalid bencode payload")
	}
}

func decodeDictionary(reader *bufio.Reader) (map[string]interface{}, error) {
	dict := map[string]interface{}{}
	var lastKey string

	for isExpectingKey := true; ; isExpectingKey = !isExpectingKey {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			break
		}

		if err := reader.UnreadByte(); err != nil {
			return nil, err
		}

		obj, err := decode(reader)
		if err != nil {
			return nil, err
		}

		if !isExpectingKey {
			dict[lastKey] = obj
			continue
		}

		if key, ok := obj.(string); ok {
			lastKey = key
			continue
		}

		return nil, errors.New("dictionary keys must be strings")
	}

	return dict, nil
}

func decodeList(reader *bufio.Reader) ([]interface{}, error) {
	list := []interface{}{}
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == 'e' {
			break
		}

		if err := reader.UnreadByte(); err != nil {
			return nil, err
		}

		obj, err := decode(reader)
		if err != nil {
			return nil, err
		}
		list = append(list, obj)
	}

	return list, nil
}

func decodeInteger(reader *bufio.Reader) (int, error) {
	intbuf, err := reader.ReadBytes('e')
	if err != nil {
		return 0, fmt.Errorf("could not read integer bytes: %w", err)
	}

	end := len(intbuf) - 1
	num, err := strconv.Atoi(string(intbuf[:end]))
	if err != nil {
		return 0, fmt.Errorf("could not parse integer: %w", err)
	}

	return num, nil
}

func decodeString(reader *bufio.Reader) (string, error) {
	lbuf, err := reader.ReadBytes(':')
	if err != nil {
		return "", fmt.Errorf("could not find string separator: %w", err)
	}

	end := len(lbuf) - 1
	length, err := strconv.Atoi(string(lbuf[:end]))
	if err != nil {
		return "", fmt.Errorf("could not read string length: %w", err)
	}

	strbuf := make([]byte, length)
	if _, err := io.ReadFull(reader, strbuf); err != nil {
		return "", fmt.Errorf("could not read string payload: %w", err)
	}

	return string(strbuf), nil
}
