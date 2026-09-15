package line

import (
	"testing"
)

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
