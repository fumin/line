package line

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// stripInvisible replaces characters that are illegal, awkward, or unsafe
// in a Linux file name with "_":
//   - '/' and NUL, the only characters the filesystem itself forbids
//   - other ASCII control characters
//   - Unicode formatting characters (category Cf: bidirectional overrides
//     like U+202E RIGHT-TO-LEFT OVERRIDE, zero-width characters, byte-order
//     marks, ...)
//   - commas and any kind of Unicode whitespace (not just ' '), since LINE
//     group names commonly contain them (e.g. an auto-generated name
//     listing member names separated by ", ")
func stripInvisible(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '/', r == '\\', r == ',':
			return '_'
		case unicode.IsSpace(r), unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			return '_'
		default:
			return r
		}
	}, name)
}

// sanitizeFileName turns an arbitrary string (e.g. a LINE group name) into
// a safe Linux file name component.
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = stripInvisible(name)

	// Truncate name if it too long.
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	const maxExtBytes = 16
	if len(ext) > maxExtBytes {
		ext = ext[:maxExtBytes]
	}
	base = truncateUTF8(base, 250-len(ext))
	name = base + ext

	// "." and ".." are reserved directory entries, and a name starting
	// with "." would otherwise become a hidden file.
	if strings.HasPrefix(name, ".") {
		name = "_" + name
	}
	if name == "" {
		name = "_"
	}

	return name
}

// truncateUTF8 cuts s to at most n bytes without splitting a multi-byte
// character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for ; cut > 0; cut-- {
		r, _ := utf8.DecodeLastRuneInString(s[:cut])
		if r != utf8.RuneError {
			break
		}
	}
	return s[:cut]
}
