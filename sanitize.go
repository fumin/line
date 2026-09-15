package line

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// invalidChars matches characters that are illegal or awkward in a Linux
// file name. '/' and NUL are the only characters the filesystem itself
// forbids, but control characters are also replaced so names stay safe to
// print and work with on the command line. Commas and spaces are also
// replaced, since LINE group names commonly contain them (e.g. an
// auto-generated name listing member names separated by ", ").
var invalidChars = regexp.MustCompile(`[/\x00-\x1f\x7f, ]`)

// sanitizeFileName turns an arbitrary string (e.g. a LINE group name) into
// a safe Linux file name component.
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = invalidChars.ReplaceAllString(name, "_")

	// "." and ".." are reserved directory entries, and a name starting
	// with "." would otherwise become a hidden file.
	if strings.HasPrefix(name, ".") {
		name = "_" + name
	}
	if name == "" {
		name = "_"
	}

	// Keep well under the common 255-byte filename limit even after
	// appending ".txt".
	const maxBytes = 200
	if len(name) > maxBytes {
		cut := maxBytes
		for cut > 0 {
			r, size := utf8.DecodeLastRuneInString(name[:cut])
			if r != utf8.RuneError || size != 1 {
				break
			}
			cut--
		}
		name = name[:cut]
	}
	return name
}
