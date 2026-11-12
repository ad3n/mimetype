package json

import (
	"bytes"
	"testing"
)

func BenchmarkConsumeString(b *testing.B) {
	for _, tc := range []struct {
		data []byte
		name string
	}{
		{[]byte(`hello"`), "Short"},
		{append(bytes.Repeat([]byte{'a'}, 4000), '"'), "Long"},
		{append(bytes.Repeat([]byte(`hello\n`), 570), '"'), "Escaped"},
		{append(bytes.Repeat([]byte(`\u0041`), 660), '"'), "UnicodeEscape"},
		{bytes.Repeat([]byte{'a'}, 4000), "Unterminated"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			p := &parserState{}
			b.ReportAllocs()
			for b.Loop() {
				p.ib = 0
				p.consumeString(tc.data)
			}
		})
	}
}
