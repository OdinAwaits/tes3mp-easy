package doctor

import "path/filepath"

func checkFilesystems(root string, deviceOf func(string) (uint64, error)) Check {
	if root == "" {
		return errorCheck("filesystems", "root path is empty")
	}
	if deviceOf == nil {
		return errorCheck("filesystems", "device probe is not configured")
	}

	uploads := filepath.Join(root, "panel-data", "uploads")
	backups := filepath.Join(root, "backups")
	uploadsDev, err := deviceOf(uploads)
	if err != nil {
		return errorCheck("filesystems", "cannot inspect %s: %v", uploads, err)
	}
	backupsDev, err := deviceOf(backups)
	if err != nil {
		return errorCheck("filesystems", "cannot inspect %s: %v", backups, err)
	}
	if uploadsDev != backupsDev {
		return errorCheck("filesystems",
			"%s and %s are on different filesystems (dev %d and %d); mods import needs rename",
			uploads, backups, uploadsDev, backupsDev)
	}
	return okCheck("filesystems", "uploads and backups share device %d", uploadsDev)
}
