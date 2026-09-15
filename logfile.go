package line

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogWriter appends message lines to <root>/<YYYY-MM-DD>/<name>.html,
// creating directories and files as needed. All filesystem access goes
// through root, so a crafted chatName can never escape it. Lines may
// contain HTML (e.g. an <a> tag linking to a saved image), so the log file
// itself is HTML; callers are responsible for escaping any dynamic text
// that isn't meant to be interpreted as markup (see WebhookHandler.describe).
type LogWriter struct {
	root *os.Root

	mu sync.Mutex
}

func NewLogWriter(root *os.Root) *LogWriter {
	return &LogWriter{root: root}
}

// Append writes line, followed by a newline, to the log file for chatName
// on the date of t. A brand new log file starts with a "<pre>" tag, so
// lines display with their original line breaks and monospacing instead of
// running together as they would in plain flowed HTML.
func (w *LogWriter) Append(t time.Time, chatName, line string) error {
	dateDir := t.Format("2006-01-02")

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.root.MkdirAll(dateDir, 0o700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	fileName := sanitizeFileName(chatName) + ".html"
	filePath := filepath.Join(dateDir, fileName)

	f, err := w.root.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}

// SaveFile writes data to <root>/<YYYY-MM-DD>/<chatName>/<name>, creating
// directories as needed, and returns the file's path relative to root
// (using "/" as the separator, e.g. "2026-09-15/mychat/foo.jpg") for use
// with ServeContent.
func (w *LogWriter) SaveFile(t time.Time, chatName, name string, data []byte) (string, error) {
	dateDir := t.Format("2006-01-02")
	subDir := sanitizeFileName(chatName)
	dirPath := filepath.Join(dateDir, subDir)

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.root.MkdirAll(dirPath, 0o700); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}

	filePath := filepath.Join(dirPath, name)
	if err := w.root.WriteFile(filePath, data, 0o600); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return filePath, nil
}
