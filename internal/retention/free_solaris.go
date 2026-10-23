//go:build solaris

package retention

import "errors"

// Ops targets Linux. Solaris has no syscall.Statfs binding in the standard
// library; refuse to guess at free space instead of silently disabling the
// reserve check.
func diskFree(string) (int64, error) {
	return 0, errors.New("disk free is not supported on solaris")
}
