package line

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogWriter appends message lines to <dir>/<YYYY-MM-DD>/<name>.txt,
// creating directories and files as needed.
type LogWriter struct {
	dir string

	mu sync.Mutex
}

func NewLogWriter(dir string) *LogWriter {
	return &LogWriter{dir: dir}
}

// Append writes line, followed by a newline, to the log file for chatName
// on the date of t.
func (w *LogWriter) Append(t time.Time, chatName, line string) error {
	dateDir := t.Format("2006-01-02")
	dirPath := filepath.Join(w.dir, dateDir)

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	fileName := sanitizeFileName(chatName) + ".txt"
	filePath := filepath.Join(dirPath, fileName)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}
