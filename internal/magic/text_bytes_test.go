package magic

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ad3n/mimetype/internal/charset"
)

func TestTextBytesExhaustive(t *testing.T) {
	if testing.Short() {
		t.Skip("extended scanner coverage")
	}

	for _, size := range []int{0, 1, 15, 16, 17, 31, 32, 33, 63, 64, 65} {
		for offset := range 16 {
			storage := bytes.Repeat([]byte{'a'}, size+32)
			raw := storage[offset : offset+size]
			if !textBytes(raw) {
				t.Fatalf("ASCII rejected: size=%d offset=%d", size, offset)
			}

			for pos := range raw {
				for value := range 256 {
					raw[pos] = byte(value)
					if got, want := textBytes(raw), textBytesGo(raw); got != want {
						t.Fatalf("size=%d offset=%d pos=%d byte=%02x: got %t want %t", size, offset, pos, value, got, want)
					}
				}

				raw[pos] = 'a'
			}
		}
	}
}

func TestTextBytesRandom(t *testing.T) {
	if testing.Short() {
		t.Skip("extended scanner coverage")
	}

	r := rand.New(rand.NewSource(42))
	for range 2000 {
		raw := make([]byte, r.Intn(8193))
		if _, err := r.Read(raw); err != nil {
			t.Fatal(err)
		}

		if got, want := textBytes(raw), textBytesGo(raw); got != want {
			t.Fatalf("length=%d: got %t want %t", len(raw), got, want)
		}

		for i, b := range raw {
			if b < 32 {
				raw[i] = []byte{'\t', '\n', '\f', '\r', 0x1b}[int(b)%5]
			}
		}

		if !textBytes(raw) {
			t.Fatalf("valid mixed text rejected: length=%d", len(raw))
		}
	}
}

func TestTextSIMDSemantics(t *testing.T) {
	for _, bom := range [][]byte{{0xef, 0xbb, 0xbf}, {0xfe, 0xff}, {0xff, 0xfe}, {0, 0, 0xfe, 0xff}, {0xff, 0xfe, 0, 0}} {
		if !Text(append(bytes.Clone(bom), make([]byte, 64)...), 0) {
			t.Fatalf("BOM rejected: %x", bom)
		}
	}

	raw := bytes.Repeat([]byte{'a'}, 4097)
	raw[4096] = 0
	if !Text(raw, 0) {
		t.Fatal("read beyond 4096-byte limit")
	}

	raw[4095] = 0
	if Text(raw, 0) {
		t.Fatal("missed final binary byte within limit")
	}
}

func FuzzTextSIMD(f *testing.F) {
	for _, raw := range [][]byte{nil, []byte("hello"), bytes.Repeat([]byte{'a'}, 4097), {0xff, 0xfe, 0, 0}, {9, 10, 11, 12, 13, 27, 31, 32, 128, 255}} {
		f.Add(raw)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if got, want := textBytes(raw), textBytesGo(raw); got != want {
			t.Fatalf("scanner mismatch: got %t want %t", got, want)
		}

		want := charset.FromBOM(raw) != "" || textBytesGo(raw[:min(len(raw), 4096)])
		if got := Text(raw, 0); got != want {
			t.Fatalf("Text mismatch: got %t want %t", got, want)
		}
	})
}

func BenchmarkTextScan(b *testing.B) {
	for _, size := range []int{0, 11, 16, 64, 1024, 4096} {
		for _, kind := range []string{"ASCII", "Whitespace", "BinaryFirst", "BinaryLast"} {
			raw := bytes.Repeat([]byte{'a'}, size)
			if kind == "Whitespace" {
				for i := range raw {
					raw[i] = []byte{'a', '\n', '\t', '\r', 0x1b}[i%5]
				}
			}

			if size > 0 && kind == "BinaryFirst" {
				raw[0] = 0
			}

			if size > 0 && kind == "BinaryLast" {
				raw[size-1] = 0
			}

			b.Run(fmt.Sprintf("%s/%d", kind, size), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					Text(raw, 0)
				}
			})
		}
	}
}
