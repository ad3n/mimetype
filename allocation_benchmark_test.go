package mimetype

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type allocationErrorReader struct{ err error }

func BenchmarkAllocationPaths(b *testing.B) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"PNG", []byte("\x89PNG\r\n\x1a\n")},
		{"Text", []byte("hello world")},
		{"HTML", []byte("<!DOCTYPE html><html><head><meta CHARSET=\"utf-8\"></head><body>hello</body></html>")},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("Detect", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					Detect(tc.data)
				}
			})
			b.Run("Reader", func(b *testing.B) {
				r := bytes.NewReader(tc.data)
				b.ReportAllocs()
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

func TestReaderPoolBoundaries(t *testing.T) {
	oldLimit := atomic.LoadUint32(&readLimit)
	defer SetLimit(oldLimit)

	for _, limit := range []uint32{0, 1, defaultLimit, maxReaderBufferCapacity, maxReaderBufferCapacity + 1} {
		SetLimit(limit)
		for _, data := range [][]byte{[]byte("\x89PNG\r\n\x1a\n"), []byte("hello world"), nil} {
			want := Detect(data).String()
			for range 3 {
				got, err := DetectReader(bytes.NewReader(data))
				if err != nil || got.String() != want {
					t.Fatalf("limit %d: got %v, %v; want %s", limit, got, err, want)
				}
			}
		}
	}
}

func TestReaderPoolErrorAndReuse(t *testing.T) {
	wantErr := errors.New("read failure")
	if _, err := DetectReader(&allocationErrorReader{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("got %v, want %v", err, wantErr)
	}

	data := []byte("\x89PNG\r\n\x1a\n")
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 50 {
				got, err := DetectReader(bytes.NewReader(data))
				if err != nil || got.String() != "image/png" {
					t.Errorf("got %v, %v", got, err)
					return
				}
			}
		})
	}

	wg.Wait()
}

func TestReaderCustomDetectorRetainsInput(t *testing.T) {
	var retained []byte
	mu.Lock()
	savedChildren, savedCustom := root.children, hasCustomDetectors
	mu.Unlock()
	defer func() {
		mu.Lock()
		root.children = savedChildren
		hasCustomDetectors = savedCustom
		mu.Unlock()
	}()

	Extend(func(raw []byte, _ uint32) bool {
		if bytes.Equal(raw, []byte("custom payload")) {
			retained = raw
			return true
		}

		return false
	}, "application/x-pool-retention", ".pool")
	if _, err := DetectReader(bytes.NewBufferString("custom payload")); err != nil {
		t.Fatal(err)
	}

	if _, err := DetectReader(bytes.NewBufferString("replacement")); err != nil {
		t.Fatal(err)
	}

	if string(retained) != "custom payload" {
		t.Fatalf("detector input changed: %q", retained)
	}
}

func (r *allocationErrorReader) Read(p []byte) (int, error) {
	copy(p, "partial input")
	return min(len(p), len("partial input")), r.err
}
