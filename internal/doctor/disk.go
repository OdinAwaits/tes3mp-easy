package doctor

import "fmt"

const (
	minFreeBytes  int64 = 512 << 20
	warnFreeBytes int64 = 1 << 30
)

func checkDisk(root string, free func(string) (int64, error)) Check {
	if root == "" {
		return errorCheck("disk", "root path is empty")
	}
	bytes, err := free(root)
	if err != nil {
		return errorCheck("disk", "cannot determine free space for %s: %v",
			root, err)
	}
	switch {
	case bytes < minFreeBytes:
		return errorCheck("disk",
			"free space %s is below the 512 MiB minimum", humanBytes(bytes))
	case bytes < warnFreeBytes:
		return warnCheck("disk",
			"free space %s is below the 1 GiB warning threshold",
			humanBytes(bytes))
	default:
		return okCheck("disk", "%s free", humanBytes(bytes))
	}
}

func humanBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
