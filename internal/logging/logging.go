// Package logging opens the log file of BlueLatch,
// %LocalAppData%\BlueLatch\bluelatch.log. Every attempt to connect is logged
// there with the Windows error codes, because that is what a connection that
// will not come up has to be diagnosed from.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/shinession/bluelatch/internal/appdir"
)

// maxLogSize is the size above which the log is moved aside when it is opened.
// One previous log (bluelatch.log.1) is kept.
const maxLogSize = 1 << 20

// Path returns where the log file lives.
func Path() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bluelatch.log"), nil
}

// Open returns a logger that appends to the log file. With verbose it logs at
// debug level and also writes to stderr. The returned function closes the file.
//
// When the file cannot be opened the logger still works (it writes to stderr
// with verbose and discards otherwise) and the error says why the file is missing.
func Open(verbose bool, stderr io.Writer) (*slog.Logger, func(), error) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	var writers []io.Writer
	closeFile := func() {}
	file, openErr := openFile()
	if openErr == nil {
		writers = append(writers, file)
		closeFile = func() { file.Close() }
	}
	if verbose && stderr != nil {
		writers = append(writers, stderr)
	}
	if len(writers) == 0 {
		return slog.New(slog.DiscardHandler), closeFile, openErr
	}

	handler := slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	return slog.New(handler), closeFile, openErr
}

func openFile() (*os.File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		// On Windows os.Rename replaces an existing file.
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
