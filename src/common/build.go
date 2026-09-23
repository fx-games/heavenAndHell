package common

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"
)

// BuildHash is the first 8 hex digits of the SHA-256 of the running executable: which binary this is. It is shown
// in the start menu and written into every save, so a save can be matched to the exact build that wrote it.
// "unknown" when the executable can't be read.
var BuildHash = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	f, err := os.Open(exe)
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
})
