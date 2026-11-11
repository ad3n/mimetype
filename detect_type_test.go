package mimetype

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
)

func TestDetectTypeMatchesHierarchy(t *testing.T) {
	defer SetLimit(defaultLimit)

	for _, limit := range []uint32{0, 1, defaultLimit, maxReaderBufferCapacity + 1} {
		SetLimit(limit)
		for _, tc := range testcases {
			data := []byte(tc.data)
			want := Detect(data)
			mediaType, extension := DetectType(data)
			if mediaType != want.String() || extension != want.Extension() {
				t.Fatalf("%s, limit %d: got %q %q, want %q %q", tc.name, limit, mediaType, extension, want.String(), want.Extension())
			}

			mediaType, extension, err := DetectReaderType(bytes.NewReader(data))
			if err != nil || mediaType != want.String() || extension != want.Extension() {
				t.Fatalf("%s, limit %d: reader got %q %q, %v", tc.name, limit, mediaType, extension, err)
			}
		}
	}
}

func TestDetectTypeLargeLimit(t *testing.T) {
	SetLimit(math.MaxUint32)
	defer SetLimit(defaultLimit)

	data := []byte("hello world")
	mediaType, _ := DetectType(data)
	if mediaType != Detect(data).String() || mediaType != "text/plain; charset=utf-8" {
		t.Fatal(mediaType)
	}
}

func TestDetectReaderTypeError(t *testing.T) {
	wantErr := errors.New("read failed")
	mediaType, extension, err := DetectReaderType(&allocationErrorReader{err: wantErr})
	if !errors.Is(err, wantErr) || mediaType != errMIME.String() || extension != errMIME.Extension() {
		t.Fatalf("got %q %q, %v", mediaType, extension, err)
	}
}

func TestDetectReaderTypeOwnsCharset(t *testing.T) {
	input := `<?xml version="1.0" encoding="private-encoding"?><a/>`
	want := "text/xml; charset=private-encoding"
	mediaType, _, err := DetectReaderType(strings.NewReader(input))
	if err != nil || mediaType != want {
		t.Fatalf("got %q, %v", mediaType, err)
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				got, _, err := DetectReaderType(strings.NewReader("hello world"))
				if err != nil || got != "text/plain; charset=utf-8" {
					t.Errorf("got %q, %v", got, err)
				}
			}
		})
	}

	wg.Wait()
	if mediaType != want {
		t.Fatalf("retained result changed to %q", mediaType)
	}
}

func TestDetectReaderTypeCustomDetectorRetainsInput(t *testing.T) {
	mu.Lock()
	savedChildren, savedCustom := root.children, hasCustomDetectors
	mu.Unlock()
	defer func() {
		mu.Lock()
		root.children, hasCustomDetectors = savedChildren, savedCustom
		mu.Unlock()
	}()

	var retained []byte
	Extend(func(raw []byte, _ uint32) bool {
		if bytes.Equal(raw, []byte("private payload")) {
			retained = raw
			return true
		}

		return false
	}, "application/x-retained", ".retained")
	mediaType, extension, err := DetectReaderType(strings.NewReader("private payload"))
	if err != nil || mediaType != "application/x-retained" || extension != ".retained" {
		t.Fatalf("got %q %q, %v", mediaType, extension, err)
	}

	for range 10 {
		if _, _, err := DetectReaderType(strings.NewReader("other payload")); err != nil {
			t.Fatal(err)
		}
	}

	if string(retained) != "private payload" {
		t.Fatalf("detector input changed to %q", retained)
	}
}

func FuzzDetectType(f *testing.F) {
	for _, input := range []string{"hello world", "\x89PNG\r\n\x1a\n", `{"a":[}`, `<?xml encoding="private-encoding"?><a/>`} {
		f.Add([]byte(input))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		want := Detect(data)
		mediaType, extension := DetectType(data)
		if mediaType != want.String() || extension != want.Extension() {
			t.Fatalf("got %q %q, want %q %q", mediaType, extension, want.String(), want.Extension())
		}

		mediaType, extension, err := DetectReaderType(bytes.NewReader(data))
		if err != nil || mediaType != want.String() || extension != want.Extension() {
			t.Fatalf("reader got %q %q, %v; want %q %q", mediaType, extension, err, want.String(), want.Extension())
		}
	})
}

func BenchmarkDetectType(b *testing.B) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"PNG", []byte("\x89PNG\r\n\x1a\n")},
		{"Text", []byte("hello world")},
		{"HTML", []byte("<!DOCTYPE html><html><head><meta CHARSET=\"utf-8\"></head><body>hello</body></html>")},
		{"XML", []byte(`<?xml version="1.0"?><a/>`)},
		{"JSON", []byte(`{"key":"value"}`)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("Detect", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					DetectType(tc.data)
				}
			})
			b.Run("Reader", func(b *testing.B) {
				r := bytes.NewReader(tc.data)
				b.ReportAllocs()
				for b.Loop() {
					r.Reset(tc.data)
					if _, _, err := DetectReaderType(r); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
