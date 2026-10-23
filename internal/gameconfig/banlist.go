package gameconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
)

type banlist struct {
	PlayerNames []string `json:"playerNames"`
	IPAddresses []string `json:"ipAddresses"`
}

func parseBanlist(path string, data []byte) (banlist, error) {
	var raw struct {
		PlayerNames *[]string `json:"playerNames"`
		IPAddresses *[]string `json:"ipAddresses"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return banlist{}, invalidf(FileBanlist, "",
			"%s: %v", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return banlist{}, invalidf(FileBanlist, "", "%s: %v", path, err)
	}
	if raw.PlayerNames == nil || raw.IPAddresses == nil {
		return banlist{}, invalidf(FileBanlist, "",
			"%s: playerNames and ipAddresses must both be arrays", path)
	}
	for _, name := range *raw.PlayerNames {
		if err := validateBanlistName(name); err != nil {
			return banlist{}, invalidf(FileBanlist, "playerNames",
				"%s: %v", path, err)
		}
	}
	for _, address := range *raw.IPAddresses {
		if err := validateBanlistIP(address); err != nil {
			return banlist{}, invalidf(FileBanlist, "ipAddresses",
				"%s: %v", path, err)
		}
	}
	return banlist{
		PlayerNames: append([]string{}, (*raw.PlayerNames)...),
		IPAddresses: append([]string{}, (*raw.IPAddresses)...),
	}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("unexpected trailing JSON value")
	}
	return err
}

func readBanlist(path string, data []byte) (Snapshot, error) {
	list, err := parseBanlist(path, data)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		File: FileBanlist,
		Values: map[string]any{
			"playerNames": append([]string{}, list.PlayerNames...),
			"ipAddresses": append([]string{}, list.IPAddresses...),
		},
		ReadOnly: []string{},
	}, nil
}

func setBanlist(path string, data []byte, key, rawValue string,
) ([]byte, any, error) {
	list, err := parseBanlist(path, data)
	if err != nil {
		return nil, nil, err
	}
	if key != "playerNames" && key != "ipAddresses" {
		return nil, nil, invalidf(FileBanlist, key,
			"key must be playerNames or ipAddresses")
	}

	items, err := parseBanlistValue(rawValue)
	if err != nil {
		return nil, nil, invalidf(FileBanlist, key, "%v", err)
	}
	if key == "playerNames" {
		for _, name := range items {
			if err := validateBanlistName(name); err != nil {
				return nil, nil, invalidf(FileBanlist, key, "%v", err)
			}
		}
		list.PlayerNames = items
	} else {
		for _, address := range items {
			if err := validateBanlistIP(address); err != nil {
				return nil, nil, invalidf(FileBanlist, key, "%v", err)
			}
		}
		list.IPAddresses = items
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(list); err != nil {
		return nil, nil, fmt.Errorf("encode %s: %w", path, err)
	}
	return out.Bytes(), items, nil
}

func parseBanlistValue(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var items []string
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&items); err != nil {
			return nil, fmt.Errorf("value must be a JSON string array: %v", err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("value must be a JSON string array: %v", err)
		}
		return append([]string{}, items...), nil
	}

	var items []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	if items == nil {
		return []string{}, nil
	}
	return items, nil
}

func validateBanlistName(name string) error {
	if name == "" {
		return fmt.Errorf("player name must not be empty")
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("player name %q contains a control character", name)
	}
	return nil
}

func validateBanlistIP(address string) error {
	if address == "" {
		return fmt.Errorf("IP address must not be empty")
	}
	if net.ParseIP(address) == nil {
		return fmt.Errorf("%q is not a valid IP address", address)
	}
	return nil
}
