package charset

import (
	"bytes"
	"unicode/utf8"

	"github.com/ad3n/mimetype/internal/markup"
	"github.com/ad3n/mimetype/internal/scan"
)

const (
	F = 0
	T = 1
	I = 2
	X = 3
)

var (
	boms = []struct {
		bom []byte
		enc string
	}{
		{[]byte{0xEF, 0xBB, 0xBF}, "utf-8"},
		{[]byte{0x00, 0x00, 0xFE, 0xFF}, "utf-32be"},
		{[]byte{0xFF, 0xFE, 0x00, 0x00}, "utf-32le"},
		{[]byte{0xFE, 0xFF}, "utf-16be"},
		{[]byte{0xFF, 0xFE}, "utf-16le"},
	}

	textChars = [256]byte{
		F, F, F, F, F, F, F, T, T, T, T, T, T, T, F, F,

		F, F, F, F, F, F, F, F, F, F, F, T, F, F, F, F,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, T,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, T,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, T,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, T,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, T,
		T, T, T, T, T, T, T, T, T, T, T, T, T, T, T, F,

		X, X, X, X, X, T, X, X, X, X, X, X, X, X, X, X,
		X, X, X, X, X, X, X, X, X, X, X, X, X, X, X, X,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
		I, I, I, I, I, I, I, I, I, I, I, I, I, I, I, I,
	}
)

func FromBOM(content []byte) string {
	for _, b := range boms {
		if bytes.HasPrefix(content, b.bom) {
			return b.enc
		}
	}

	return ""
}

func FromPlain(content []byte) string {
	if len(content) == 0 {
		return ""
	}

	if cset := FromBOM(content); cset != "" {
		return cset
	}

	origContent := content

	for i := len(content) - 1; i >= 0 && i > len(content)-4; i-- {
		b := content[i]
		if b < 0x80 {
			break
		}

		if utf8.RuneStart(b) {
			content = content[:i]
			break
		}
	}

	if utf8.Valid(content) {
		return "utf-8"
	}

	return latin(origContent)
}

func latin(content []byte) string {
	hasControlBytes := false
	for _, b := range content {
		t := textChars[b]
		if t != T && t != I {
			return ""
		}

		if b >= 0x80 && b <= 0x9F {
			hasControlBytes = true
		}
	}

	if hasControlBytes {
		return "windows-1252"
	}

	return "iso-8859-1"
}

func FromXML(content []byte) string {
	if cset := fromXML(content); cset != "" {
		return cset
	}

	return FromPlain(content)
}
func fromXML(s scan.Bytes) string {
	xml := []byte("<?xml")
	lxml := len(xml)
	for {
		s.TrimLWS()
		if len(s) <= lxml {
			return ""
		}

		i, k := s.Search(xml, 0)
		if i == -1 {
			return ""
		}

		s.Advance(i + k)
		var aName, aVal []byte
		hasMore := true
		for hasMore {
			aName, aVal, hasMore = markup.GetAnAttribute(&s)
			if scan.Bytes(aName).Match([]byte("encoding"), 0) != -1 && len(aVal) != 0 {
				return charsetString(aVal)
			}
		}
	}
}

func FromHTML(content []byte) string {
	if cset := FromBOM(content); cset != "" {
		return cset
	}

	if cset := fromHTML(content); cset != "" {
		return cset
	}

	return FromPlain(content)
}

func fromHTML(s scan.Bytes) string {
	const (
		dontKnow = iota
		doNeedPragma
		doNotNeedPragma
	)
	meta := []byte("<META")
	body := []byte("<BODY")
	lmeta := len(meta)
	for {
		if markup.SkipAComment(&s) {
			continue
		}

		if len(s) <= lmeta {
			return ""
		}

		if s.Match(body, scan.IgnoreCase) != -1 {
			return ""
		}

		if s.Match(meta, scan.IgnoreCase) == -1 {
			s = s[1:]
			continue
		}

		s = s[lmeta:]
		c := s.Pop()
		if c == 0 || (!scan.ByteIsWS(c) && c != '/') {
			return ""
		}

		var seen uint8
		gotPragma := false
		needPragma := dontKnow

		charset := ""
		var aNameB, aValB []byte
		hasMore := true
		for hasMore {
			aNameB, aValB, hasMore = markup.GetAnAttribute(&s)
			var attr uint8
			switch {
			case len(aNameB) == 10 && bytes.EqualFold(aNameB, []byte("http-equiv")):
				attr = 1
			case len(aNameB) == 7 && bytes.EqualFold(aNameB, []byte("content")):
				attr = 2
			case len(aNameB) == 7 && bytes.EqualFold(aNameB, []byte("charset")):
				attr = 4
			}

			if seen&attr != 0 {
				continue
			}

			if len(aNameB) == 0 && len(aValB) == 0 {
				if needPragma == dontKnow {
					continue
				}

				if needPragma == doNeedPragma && !gotPragma {
					continue
				}
			}

			seen |= attr
			switch attr {
			case 1:
				if scan.Bytes(aValB).Match([]byte("CONTENT-TYPE"), scan.IgnoreCase) != -1 {
					gotPragma = true
				}
			case 2:
				charset = charsetString(extractCharsetFromMeta(scan.Bytes(aValB)))
				if len(charset) != 0 {
					needPragma = doNeedPragma
				}
			case 4:
				charset = charsetString(aValB)
				needPragma = doNotNeedPragma
			}
		}

		if needPragma == dontKnow || needPragma == doNeedPragma && !gotPragma {
			continue
		}

		return charset
	}
}

func extractCharsetFromMeta(s scan.Bytes) []byte {
	for {
		i := bytes.Index(s, []byte("charset"))
		if i == -1 {
			return nil
		}

		s.Advance(i + len("charset"))
		for scan.ByteIsWS(s.Peek()) {
			s.Advance(1)
		}

		if s.Pop() != '=' {
			continue
		}

		for scan.ByteIsWS(s.Peek()) {
			s.Advance(1)
		}

		quote := s.Peek()
		if quote == 0 {
			return nil
		}

		if quote == '"' || quote == '\'' {
			s.Advance(1)
			return bytes.TrimSpace(s.PopUntil(quote))
		}

		return bytes.TrimSpace(s.PopUntil(';', '\t', '\n', '\x0c', '\r', ' '))
	}
}

func charsetString(raw []byte) string {
	switch string(raw) {
	case "utf-8":
		return "utf-8"
	case "UTF-8":
		return "UTF-8"
	case "iso-8859-1":
		return "iso-8859-1"
	case "ISO-8859-1":
		return "ISO-8859-1"
	case "windows-1252":
		return "windows-1252"
	}

	return string(raw)
}
