//go:build !windows

package morrowind

import "errors"

type unavailableRegistry struct{}

func (unavailableRegistry) ReadString(string, string) (string, error) {
	return "", errors.New("Windows registry is not available on this platform")
}

func systemRegistry() Registry {
	return unavailableRegistry{}
}
