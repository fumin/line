package util

import (
	"bytes"
	"time"

	"gitlab.com/golang-commonmark/linkify"
)

var (
	TaipeiTZ = time.FixedZone("Asia/Taipei", int((8 * time.Hour).Seconds()))
)

func MakeLinks(raw []byte) []byte {
	s := string(raw)

	var out bytes.Buffer
	last := 0
	for _, l := range linkify.Links(s) {
		out.WriteString(s[last:l.Start])
		url := s[l.Start:l.End]
		out.WriteString(`<a href="`)
		out.WriteString(url)
		out.WriteString(`">`)
		out.WriteString(url)
		out.WriteString(`</a>`)
		last = l.End
	}
	out.WriteString(s[last:])

	return out.Bytes()
}
