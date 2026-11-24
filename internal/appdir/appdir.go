// Package appdir is the per-user folder of BlueLatch, %LocalAppData%\BlueLatch.
// It holds the log file, the journal of the service switches in progress, and
// the data of the WebView2 window.
package appdir

import (
	"os"
	"path/filepath"
)

// Dir returns the folder. It does not create it.
func Dir() (string, error) {
	base, err := os.UserCacheDir() // %LocalAppData% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "BlueLatch"), nil
}
