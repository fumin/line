package line

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
)

// DeleteOldLogs deletes date-named directories directly under root (e.g.
// "2026-03-15", as created by LogWriter) whose date is older than 6 months.
// Entries that aren't directories, or whose name isn't a "2006-01-02" date,
// are left alone.
func DeleteOldLogs(root *os.Root) error {
	cutoff := time.Now().AddDate(0, -6, 0)

	f, err := root.Open(".")
	if err != nil {
		return errors.Wrap(err, "")
	}
	defer f.Close()

	entries, err := f.ReadDir(-1)
	if err != nil {
		return errors.Wrap(err, "")
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		t, err := time.Parse("2006-01-02", e.Name())
		if err != nil {
			continue
		}
		if t.After(cutoff) {
			continue
		}

		if err := root.RemoveAll(e.Name()); err != nil {
			log.Printf("delete old log dir %q: %+v", e.Name(), err)
		}
	}

	return nil
}
