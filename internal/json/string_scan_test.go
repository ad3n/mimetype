package json

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"
)

func checkStringScanner(t *testing.T, scan func([]byte) int) {
	t.Helper()
	for _, size := range []int{0, 1, 15, 16, 17, 31, 32, 33, 63, 64, 65} {
		for offset := range 32 {
			storage := bytes.Repeat([]byte{'a'}, size+64)
			raw := storage[offset : offset+size]
			if got := scan(raw); got != size {
				t.Fatalf("length=%d got=%d", size, got)
			}

			for pos := range raw {
				for value := range 256 {
					raw[pos] = byte(value)
					if got, want := scan(raw), indexStringSpecialGo(raw); got != want {
						t.Fatalf("size=%d offset=%d pos=%d byte=%02x got=%d want=%d", size, offset, pos, value, got, want)
					}
				}

				raw[pos] = 'a'
			}
		}
	}

	for _, marker := range []byte{'"', '\\'} {
		raw := bytes.Repeat([]byte{marker}, 128)
		for pos := range raw {
			if got := scan(raw); got != pos {
				t.Fatalf("first match: got %d want %d", got, pos)
			}

			raw[pos] = 'a'
		}
	}
}

func TestStringScanner(t *testing.T) {
	if testing.Short() {
		t.Skip("extended scanner coverage")
	}

	checkStringScanner(t, indexStringSpecial)
}

func checkConsumeString(t *testing.T, raw []byte) {
	t.Helper()
	actual, reference := &parserState{ib: 123}, &parserState{ib: 123}
	got, want := actual.consumeString(raw), consumeStringReference(reference, raw)
	if got != want || actual.ib != reference.ib {
		t.Fatalf("input=%x got=(%d,%d) want=(%d,%d)", raw, got, actual.ib, want, reference.ib)
	}
}

func TestConsumeStringDifferential(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`"`), []byte(`hello"`), []byte(`\`), []byte(`\u`), []byte(`\u12`), []byte(`\u1234"`), []byte(`\u12x4"`), []byte(`a\"b"`), {0, 0xff, '"'}} {
		checkConsumeString(t, raw)
	}

	if testing.Short() {
		return
	}

	r := rand.New(rand.NewSource(42))
	for range 2000 {
		raw := make([]byte, r.Intn(8193))
		if _, err := r.Read(raw); err != nil {
			t.Fatal(err)
		}

		checkConsumeString(t, raw)
		for i, c := range raw {
			raw[i] = []byte("abc\\\"u0123x")[int(c)%11]
		}

		checkConsumeString(t, raw)
	}
}

func FuzzConsumeStringSIMD(f *testing.F) {
	for _, raw := range [][]byte{nil, []byte(`hello"`), []byte(`\u1234\n"`), []byte(`\u12`), bytes.Repeat([]byte{'a'}, 4096)} {
		f.Add(raw)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if got, want := indexStringSpecial(raw), indexStringSpecialGo(raw); got != want {
			t.Fatalf("scanner got=%d want=%d", got, want)
		}

		checkConsumeString(t, raw)
	})
}

func consumeStringReference(p *parserState, b []byte) (n int) {
	var c byte
	for len(b[n:]) > 0 {
		c, n = b[n], n+1
		p.ib++
		switch c {
		case '\\':
			if len(b[n:]) == 0 {
				return 0
			}

			switch b[n] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				n++
				p.ib++
				continue
			case 'u':
				n++
				p.ib++
				for j := 0; j < 4 && len(b[n:]) > 0; j++ {
					if !isXDigit(b[n]) {
						return 0
					}

					n++
					p.ib++
				}

				continue
			default:
				return 0
			}
		case '"':
			return n
		default:
			continue
		}
	}

	return 0
}

func TestStringScannerBlockBoundaries(t *testing.T) {
	for _, size := range []int{15, 16, 17, 127, 128, 129, 4095, 4096, 4097, 8191, 8192, 8193, 1 << 20} {
		raw := bytes.Repeat([]byte{'a'}, size)
		if got := indexStringSpecial(raw); got != size {
			t.Fatalf("size=%d got=%d", size, got)
		}

		checkConsumeString(t, raw)
		for _, pos := range []int{0, 15, 16, 31, 32, 126, 127, 128, 4094, 4095, 4096, 8191, size - 1} {
			if pos >= size {
				continue
			}

			for _, marker := range []byte{'"', '\\'} {
				raw[pos] = marker
				if got := indexStringSpecial(raw); got != pos {
					t.Fatalf("size=%d pos=%d got=%d", size, pos, got)
				}

				checkConsumeString(t, raw)
			}

			raw[pos] = 'a'
		}
	}
}

func TestParseSIMDConcurrent(t *testing.T) {
	inputs := [][]byte{
		[]byte(`{"type":"Feature","value":"` + string(bytes.Repeat([]byte{'a'}, 8192)) + `"}`),
		[]byte(`{"value":"a\n\u1234\"b"}`),
		[]byte(`{"value":"unterminated`),
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				for _, raw := range inputs {
					for _, query := range []string{QueryNone, QueryGeo, QueryHAR, QueryGLTF, QueryCDX} {
						want := &parserState{maxRecursion: maxRecursion, firstToken: TokInvalid}
						parsed := want.consumeAny(raw, queries[query], 0)
						got, inspected, token, satisfied := Parse(query, raw)
						if got != parsed || inspected != want.ib || token != want.firstToken || satisfied != want.querySatisfied {
							t.Error("pooled parser result differs from an isolated parser")
							return
						}
					}
				}
			}
		})
	}

	wg.Wait()
}
