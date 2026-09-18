package util

import (
	"flag"
	"fmt"
	"log"
	"slices"
	"testing"
)

func TestMakeLinks(t *testing.T) {
	tests := []struct {
		b    []byte
		want []byte
	}{
		{
			b: []byte(`謝~
https://a.b.com/d?us=sh&o=10

"顧問案一覽表" , 請大家也再確認一下, 謝謝!!
https://j.news/Ser?p=2026-09-18%2F%E8%AB%96%E5%A3%87%EF%BC%9F.html`),
			want: []byte(`謝~
<a href="https://a.b.com/d?us=sh&o=10">https://a.b.com/d?us=sh&o=10</a>

"顧問案一覽表" , 請大家也再確認一下, 謝謝!!
<a href="https://j.news/Ser?p=2026-09-18%2F%E8%AB%96%E5%A3%87%EF%BC%9F.html">https://j.news/Ser?p=2026-09-18%2F%E8%AB%96%E5%A3%87%EF%BC%9F.html</a>`),
		},
		{
			b:    []byte(`@洪署到	https://g.com/e 吗？在 http://g.io/f/ 这里`),
			want: []byte(`@洪署到	<a href="https://g.com/e">https://g.com/e</a> 吗？在 <a href="http://g.io/f/">http://g.io/f/</a> 这里`),
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got := MakeLinks(test.b)
			if !slices.Equal(got, test.want) {
				t.Errorf("%s want %s", got, test.want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	log.SetFlags(log.Lmicroseconds | log.Llongfile | log.LstdFlags)

	m.Run()
}
