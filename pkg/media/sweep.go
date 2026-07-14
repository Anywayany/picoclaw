package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SweepExpiredFiles removes orphaned regular files in dir whose modification
// time is older than maxAge. It is intended for process-startup recovery when
// the in-memory MediaStore from a previous process no longer exists.
//
// Only direct children are considered. Directories and symbolic links are
// skipped so cleanup cannot traverse or delete outside the managed directory.
func SweepExpiredFiles(dir string, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read media sweep directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	var sweepErr error
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			sweepErr = errors.Join(sweepErr, fmt.Errorf("stat %s: %w", entry.Name(), infoErr))
			continue
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			sweepErr = errors.Join(sweepErr, fmt.Errorf("remove %s: %w", entry.Name(), removeErr))
			continue
		}
		removed++
	}

	return removed, sweepErr
}
