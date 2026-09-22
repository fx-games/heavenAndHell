package util

import "os"

// WriteFileAtomic writes data to a temp file beside path and renames it over path, so a crash mid-write leaves
// the old file, never a truncated one.
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
