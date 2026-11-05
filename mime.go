package mimetype

import (
	stdmime "mime"
	"slices"
	"strings"

	"github.com/ad3n/mimetype/internal/charset"
	"github.com/ad3n/mimetype/internal/magic"
)

type MIME struct {
	detector  magic.Detector
	parent    *MIME
	mime      string
	extension string
	aliases   []string
	children  []*MIME
}

func (m *MIME) String() string {
	return m.mime
}

func (m *MIME) Extension() string {
	return m.extension
}

func (m *MIME) Parent() *MIME {
	return m.parent
}

func (m *MIME) Is(expectedMIME string) bool {
	expectedMIME, _, _ = stdmime.ParseMediaType(expectedMIME)
	found, _, _ := stdmime.ParseMediaType(m.mime)

	if expectedMIME == found {
		return true
	}

	if slices.Contains(m.aliases, expectedMIME) {
		return true
	}

	return false
}

func newMIME(
	mime, extension string,
	detector magic.Detector,
	children ...*MIME) *MIME {
	m := &MIME{
		mime:      mime,
		extension: extension,
		detector:  detector,
		children:  children,
	}

	for _, c := range children {
		c.parent = m
	}

	return m
}

func (m *MIME) alias(aliases ...string) *MIME {
	m.aliases = aliases
	return m
}

func (m *MIME) match(in []byte, readLimit uint32) *MIME {
	for _, c := range m.children {
		if c.detector(in, readLimit) {
			return c.match(in, readLimit)
		}
	}

	var detectedCharset string
	switch m.mime {
	case "text/plain":
		detectedCharset = charset.FromPlain(in[:min(len(in), 1024)])
	case "text/html":
		detectedCharset = charset.FromHTML(in[:min(len(in), 1024)])
	case "text/xml":
		detectedCharset = charset.FromXML(in[:min(len(in), 1024)])
	}

	if m == root || detectedCharset == "" {
		return m
	}

	return m.cloneHierarchy(detectedCharset)
}

func (m *MIME) flatten() []*MIME {
	out := []*MIME{m} //nolint:prealloc
	for _, c := range m.children {
		out = append(out, c.flatten()...)
	}

	return out
}

func (m *MIME) hierarchy() string {
	var h strings.Builder
	for m := m; m != nil; m = m.Parent() {
		e := strings.TrimPrefix(m.Extension(), ".")
		if e == "" {
			e = strings.Split(m.String(), "/")[1]
			if m.Is("application/octet-stream") {
				e = "root"
			}
		}

		h.WriteString(">")
		h.WriteString(e)
	}

	return strings.TrimPrefix(h.String(), ">")
}

func (m *MIME) cloneHierarchy(charset string) *MIME {
	n := 0
	for p := m; p != nil; p = p.parent {
		n++
	}

	nodes := make([]MIME, n)
	for i, p := 0, m; p != nil; i, p = i+1, p.parent {
		nodes[i] = MIME{
			mime:      p.mime,
			aliases:   p.aliases,
			extension: p.extension,
		}
		if i+1 < n {
			nodes[i].parent = &nodes[i+1]
		}
	}

	if charset != "" {
		nodes[0].mime = mimeWithCharset(nodes[0].mime, charset)
	}

	return &nodes[0]
}

func (m *MIME) lookup(mime string) *MIME {
	if mime == m.mime {
		return m
	}

	if slices.Contains(m.aliases, mime) {
		return m
	}

	for _, c := range m.children {
		if m := c.lookup(mime); m != nil {
			return m
		}
	}

	return nil
}

func (m *MIME) Extend(detector func(raw []byte, limit uint32) bool, mime, extension string, aliases ...string) {
	mime, _, _ = stdmime.ParseMediaType(mime)
	normalisedAliases := make([]string, len(aliases))
	for i, alias := range aliases {
		normalisedAliases[i], _, _ = stdmime.ParseMediaType(alias)
	}

	c := &MIME{
		mime:      mime,
		extension: extension,
		detector:  detector,
		parent:    m,
		aliases:   normalisedAliases,
	}

	mu.Lock()
	hasCustomDetectors = true
	m.children = append([]*MIME{c}, m.children...)
	mu.Unlock()
}

func mimeWithCharset(mime, charset string) string {
	if charset == "utf-8" {
		switch mime {
		case "text/plain":
			return "text/plain; charset=utf-8"
		case "text/html":
			return "text/html; charset=utf-8"
		case "text/xml":
			return "text/xml; charset=utf-8"
		}
	}

	return mime + "; charset=" + charset
}
