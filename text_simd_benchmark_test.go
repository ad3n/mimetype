package mimetype

import (
	"bytes"
	"math/rand"
	"testing"
)

func benchmarkTextSIMDInputs(b *testing.B) []struct {
	data []byte
	name string
} {
	b.Helper()
	random := make([]byte, 4096)
	if _, err := rand.New(rand.NewSource(42)).Read(random); err != nil {
		b.Fatal(err)
	}

	return []struct {
		data []byte
		name string
	}{
		{[]byte("hello world"), "Text11"},
		{bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 94)[:4096], "Text4K"},
		{[]byte(`<html><head></head><body>` + string(bytes.Repeat([]byte("hello "), 675)) + `</body></html>`), "HTML4K"},
		{[]byte(`{"key":"` + string(bytes.Repeat([]byte("a"), 4000)) + `"}`), "JSON4K"},
		{bytes.Repeat([]byte("name,value,count\nalpha,beta,123\n"), 130)[:4096], "CSV4K"},
		{append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 4088)...), "PNG4K"},
		{random, "Random4K"},
	}
}

func BenchmarkTextSIMDSerial(b *testing.B) {
	for _, tc := range benchmarkTextSIMDInputs(b) {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("Detect", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.data)))
				for b.Loop() {
					Detect(tc.data)
				}
			})
			b.Run("Type", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.data)))
				for b.Loop() {
					DetectType(tc.data)
				}
			})
			b.Run("Reader", func(b *testing.B) {
				r := bytes.NewReader(tc.data)
				b.ReportAllocs()
				b.SetBytes(int64(len(tc.data)))
				for b.Loop() {
					r.Reset(tc.data)
					if _, err := DetectReader(r); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkTextSIMDParallel(b *testing.B) {
	for _, tc := range benchmarkTextSIMDInputs(b) {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.data)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					Detect(tc.data)
				}
			})
		})
	}
}
