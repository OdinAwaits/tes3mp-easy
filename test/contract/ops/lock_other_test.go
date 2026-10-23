//go:build !linux

package ops

import "testing"

// holdOpsLock is Linux-specific because the production lock uses flock.
func holdOpsLock(t *testing.T, root string) {
	t.Helper()
	t.Skip("ops lock contract test requires flock")
}
