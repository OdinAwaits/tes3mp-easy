//go:build windows

package morrowind

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

type windowsRegistry struct{}

func (windowsRegistry) ReadString(key, value string) (string, error) {
	handle, err := registry.OpenKey(registry.LOCAL_MACHINE, key,
		registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	result, _, err := handle.GetStringValue(value)
	if err != nil {
		return "", fmt.Errorf("read registry %s/%s: %w", key, value, err)
	}
	return result, nil
}

func systemRegistry() Registry {
	return windowsRegistry{}
}
