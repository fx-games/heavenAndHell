//go:build !embedded && development

package files

import (
    "os"
)

func init() {
    dirFS := os.DirFS("files") // TODO: we really want this to be just the current working directory, in a release build..
    if asFileSystem, ok := dirFS.(FileSystem); ok {
        sourceFileSystem = asFileSystem
    }
}