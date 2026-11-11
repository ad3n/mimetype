package mimetype

import (
	"bytes"
	"io"
	"mime"
	"os"
	"sync"
	"sync/atomic"
)

const (
	defaultLimit            uint32 = 4096
	maxReaderBufferCapacity        = 64 << 10
)

var (
	readLimit          uint32 = defaultLimit
	hasCustomDetectors bool
	readerBufferPool   = sync.Pool{New: func() any {
		b := make([]byte, defaultLimit)
		return &b
	}}
)

func Detect(in []byte) *MIME {
	l := atomic.LoadUint32(&readLimit)
	if l > 0 && uint64(len(in)) > uint64(l) {
		in = in[:l]
	}

	mu.RLock()
	defer mu.RUnlock()

	return root.match(in, l)
}

func DetectReader(r io.Reader) (*MIME, error) {
	m, charset, err := detectReader(r)
	if charset != "" {
		m = m.cloneHierarchy(charset)
	}

	return m, err
}

func DetectType(in []byte) (mediaType, extension string) {
	l := atomic.LoadUint32(&readLimit)
	if l > 0 && uint64(len(in)) > uint64(l) {
		in = in[:l]
	}

	mu.RLock()
	defer mu.RUnlock()

	m, charset := root.matchType(in, l)
	if charset != "" {
		return mimeWithCharset(m.mime, charset), m.extension
	}

	return m.mime, m.extension
}

func DetectReaderType(r io.Reader) (mediaType, extension string, err error) {
	m, charset, err := detectReader(r)
	if charset != "" {
		return mimeWithCharset(m.mime, charset), m.extension, err
	}

	return m.mime, m.extension, err
}

func detectReader(r io.Reader) (*MIME, string, error) {
	var in []byte
	var err error
	var pooled *[]byte

	l := atomic.LoadUint32(&readLimit)
	if l == 0 {
		in, err = io.ReadAll(r)
		if err != nil {
			return errMIME, "", err
		}
	}

	if l > 0 {
		if l <= maxReaderBufferCapacity {
			pooled = readerBufferPool.Get().(*[]byte)
			if cap(*pooled) < int(l) {
				*pooled = make([]byte, l)
			}

			*pooled = (*pooled)[:l]
			in = (*pooled)[:l:l]
			defer func() {
				clear((*pooled)[:cap(*pooled)])
				readerBufferPool.Put(pooled)
			}()
		}

		if pooled == nil {
			in = make([]byte, l)
		}

		n, readErr := io.ReadFull(r, in)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return errMIME, "", readErr
		}

		in = in[:n]
	}

	mu.RLock()
	defer mu.RUnlock()

	if pooled != nil && hasCustomDetectors {
		in = bytes.Clone(in)
	}

	m, charset := root.matchType(in, l)
	return m, charset, nil
}

func DetectFile(path string) (*MIME, error) {
	f, err := os.Open(path)
	if err != nil {
		return errMIME, err
	}
	defer f.Close()

	return DetectReader(f)
}

func EqualsAny(s string, mimes ...string) bool {
	s, _, _ = mime.ParseMediaType(s)
	for _, m := range mimes {
		m, _, _ = mime.ParseMediaType(m)
		if s == m {
			return true
		}
	}

	return false
}

func SetLimit(limit uint32) {
	atomic.StoreUint32(&readLimit, limit)
}

func Extend(detector func(raw []byte, limit uint32) bool, mime, extension string, aliases ...string) {
	root.Extend(detector, mime, extension, aliases...)
}

func Lookup(m string) *MIME {
	m, _, _ = mime.ParseMediaType(m)
	mu.RLock()
	defer mu.RUnlock()

	return root.lookup(m)
}
