package line

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// testRoot opens a fresh temp directory as an *os.Root for use in tests.
func testRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestSanitizeFileName(t *testing.T) {
	cases := map[string]string{
		"RM AI專案實習":                "RM_AI專案實習",
		"聖倫老師, mybot, shaoyufumin": "聖倫老師__mybot__shaoyufumin",
		"a/b":                      "a_b",
		"..":                       "_..",
		"":                         "_",
		".hidden":                  "_.hidden",
	}
	for in, want := range cases {
		if got := sanitizeFileName(in); got != want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeFileNameStripsBidiAndInvisible(t *testing.T) {
	// U+202E RIGHT-TO-LEFT OVERRIDE and U+200B ZERO WIDTH SPACE.
	in := "safe" + "\u202E" + "exe.mp3" + "\u200B"
	want := "safe_exe.mp3_"
	if got := sanitizeFileName(in); got != want {
		t.Errorf("sanitizeFileName(%q) = %q, want %q", in, got, want)
	}
}

func TestLogWriterSaveFile(t *testing.T) {
	w := NewLogWriter(testRoot(t))
	tm := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	relPath, err := w.SaveFile(tm, "聖倫老師, mybot, shaoyufumin", "123.jpg", []byte("fake image bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-09-15/聖倫老師__mybot__shaoyufumin.files/123.jpg"; relPath != want {
		t.Errorf("relPath = %q, want %q", relPath, want)
	}
}

func TestImageFileName(t *testing.T) {
	tm := time.Date(2026, 9, 15, 22, 51, 50, 0, time.UTC)
	if got, want := imageFileName(tm, "image/jpeg"), "image_20260915_225150.jpg"; got != want {
		t.Errorf("imageFileName() = %q, want %q", got, want)
	}
}

func TestFileAnchor(t *testing.T) {
	relPath := "2026-09-15/聖倫老師__mybot__shaoyufumin/image_20260915_225150.jpg"
	name := "image_20260915_225150.jpg"

	got := fileAnchor(relPath, name)
	want := fmt.Sprintf(`<a href="%s">%s</a>`, serveContentURL(relPath), name)
	if got != want {
		t.Errorf("fileAnchor() = %q, want %q", got, want)
	}
	if want := "/ServeContent?p=2026-09-15%2F%E8%81%96%E5%80%AB%E8%80%81%E5%B8%AB__mybot__shaoyufumin%2Fimage_20260915_225150.jpg"; serveContentURL(relPath) != want {
		t.Errorf("serveContentURL(%q) = %q, want %q", relPath, serveContentURL(relPath), want)
	}
}
