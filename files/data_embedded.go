//go:build embedded

package files
import "embed"

//go:embed all:assets
var embeddedFS embed.FS


func init() {
    sourceFileSystem = embeddedFS
}