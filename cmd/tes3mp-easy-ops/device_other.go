//go:build !unix

package main

import "errors"

// deviceOfPath is unavailable outside unix. Ops targets Linux; other platforms
// must not silently accept a cross-filesystem import.
func deviceOfPath(string) (uint64, error) {
	return 0, errors.New("device inspection is supported only on unix")
}
