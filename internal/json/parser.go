package json

import (
	"bytes"
	"sync"
)

const (
	QueryNone    = "json"
	QueryGeo     = "geo"
	QueryHAR     = "har"
	QueryGLTF    = "gltf"
	QueryCDX     = "cdx"
	maxRecursion = 4096
)

const (
	TokInvalid = 0
	TokNull    = 1 << iota
	TokTrue
	TokFalse
	TokNumber
	TokString
	TokArray
	TokObject
	TokComma
)

type (
	parserState struct {
		currPath       [][]byte
		ib             int
		maxRecursion   int
		firstToken     int
		querySatisfied bool
	}

	query struct {
		SearchPath [][]byte
		SearchVals [][]byte
	}
)

var (
	queries = map[string][]query{
		QueryNone: nil,
		QueryGeo: {{
			SearchPath: [][]byte{[]byte("type")},
			SearchVals: [][]byte{
				[]byte(`"Feature"`),
				[]byte(`"FeatureCollection"`),
				[]byte(`"Point"`),
				[]byte(`"LineString"`),
				[]byte(`"Polygon"`),
				[]byte(`"MultiPoint"`),
				[]byte(`"MultiLineString"`),
				[]byte(`"MultiPolygon"`),
				[]byte(`"GeometryCollection"`),
			},
		}},
		QueryHAR: {{
			SearchPath: [][]byte{[]byte("log"), []byte("version")},
		}, {
			SearchPath: [][]byte{[]byte("log"), []byte("creator")},
		}, {
			SearchPath: [][]byte{[]byte("log"), []byte("entries")},
		}},
		QueryGLTF: {{
			SearchPath: [][]byte{[]byte("asset"), []byte("version")},
			SearchVals: [][]byte{[]byte(`"1.0"`), []byte(`"2.0"`)},
		}},
		QueryCDX: {{
			SearchPath: [][]byte{[]byte("bomFormat")},
			SearchVals: [][]byte{[]byte(`"CycloneDX"`)},
		}},
	}

	parserPool = sync.Pool{
		New: func() any {
			return &parserState{maxRecursion: maxRecursion, firstToken: TokInvalid}
		},
	}

	openArray = []byte{'['}
)

func eq(path1, path2 [][]byte) bool {
	if len(path1) != len(path2) {
		return false
	}

	for i := range path1 {
		if !bytes.Equal(path1[i], path2[i]) {
			return false
		}
	}

	return true
}

func Parse(queryType string, raw []byte) (parsed, inspected, firstToken int, querySatisfied bool) {
	p := parserPool.Get().(*parserState)
	defer func() {
		p.reset()
		parserPool.Put(p)
	}()

	qs := queries[queryType]
	got := p.consumeAny(raw, qs, 0)
	return got, p.ib, p.firstToken, p.querySatisfied
}

func (p *parserState) reset() {
	p.ib = 0
	if cap(p.currPath) > 128 {
		p.currPath = nil
	}

	clear(p.currPath[:cap(p.currPath)])
	p.currPath = p.currPath[:0]
	p.firstToken = TokInvalid
	p.querySatisfied = false
}

func (p *parserState) consumeSpace(b []byte) (n int) {
	for len(b) > 0 && isSpace(b[0]) {
		b = b[1:]
		n++
		p.ib++
	}

	return n
}

func (p *parserState) consumeConst(b, cnst []byte) int {
	lb := len(b)
	for i, c := range cnst {
		if lb <= i || b[i] != c {
			return 0
		}

		p.ib++
	}

	return len(cnst)
}

func (p *parserState) consumeString(b []byte) (n int) {
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

func (p *parserState) consumeNumber(b []byte) (n int) {
	got := false
	var i int

	if len(b) == 0 {
		goto out
	}

	if b[0] == '-' {
		b, i = b[1:], i+1
		p.ib++
	}

	for len(b) > 0 {
		if !isDigit(b[0]) {
			break
		}

		got = true
		b, i = b[1:], i+1
		p.ib++
	}

	if len(b) == 0 {
		goto out
	}

	if b[0] == '.' {
		b, i = b[1:], i+1
		p.ib++
	}

	for len(b) > 0 {
		if !isDigit(b[0]) {
			break
		}

		got = true
		b, i = b[1:], i+1
		p.ib++
	}

	if len(b) == 0 {
		goto out
	}

	if got && (b[0] == 'e' || b[0] == 'E') {
		b, i = b[1:], i+1
		p.ib++
		got = false
		if len(b) == 0 {
			goto out
		}

		if b[0] == '+' || b[0] == '-' {
			b, i = b[1:], i+1
			p.ib++
		}

		for len(b) > 0 {
			if !isDigit(b[0]) {
				break
			}

			got = true
			b, i = b[1:], i+1
			p.ib++
		}
	}

out:
	if got {
		return i
	}

	return 0
}

func (p *parserState) consumeArray(b []byte, qs []query, lvl int) (n int) {
	p.appendPath(openArray, qs)
	if len(b) == 0 {
		return 0
	}

	for n < len(b) {
		n += p.consumeSpace(b[n:])
		if len(b[n:]) == 0 {
			return 0
		}

		if b[n] == ']' {
			p.ib++
			p.popLastPath(qs)
			return n + 1
		}

		innerParsed := p.consumeAny(b[n:], qs, lvl)
		if innerParsed == 0 {
			return 0
		}

		n += innerParsed
		if len(b[n:]) == 0 {
			return 0
		}

		switch b[n] {
		case ',':
			n += 1
			p.ib++
			continue
		case ']':
			p.ib++
			p.popLastPath(qs)
			return n + 1
		default:
			return 0
		}
	}

	return 0
}

func queryPathMatch(qs []query, path [][]byte) int {
	for i := range qs {
		if eq(qs[i].SearchPath, path) {
			return i
		}
	}

	return -1
}

func (p *parserState) appendPath(path []byte, qs []query) {
	if len(qs) != 0 {
		p.currPath = append(p.currPath, path)
	}
}
func (p *parserState) popLastPath(qs []query) {
	if len(qs) != 0 {
		p.currPath = p.currPath[:len(p.currPath)-1]
	}
}

func (p *parserState) consumeObject(b []byte, qs []query, lvl int) (n int) {
	for n < len(b) {
		n += p.consumeSpace(b[n:])
		if len(b[n:]) == 0 {
			return 0
		}

		if b[n] == '}' {
			p.ib++
			return n + 1
		}

		if b[n] != '"' {
			return 0
		}

		n++
		p.ib++

		queryMatched := -1
		keyLen := p.consumeString(b[n:])
		if keyLen == 0 {
			return 0
		}

		p.appendPath(b[n:n+keyLen-1], qs)
		if !p.querySatisfied {
			queryMatched = queryPathMatch(qs, p.currPath)
		}

		n += keyLen

		n += p.consumeSpace(b[n:])
		if len(b[n:]) == 0 {
			return 0
		}

		if b[n] != ':' {
			return 0
		}

		n++
		p.ib++

		n += p.consumeSpace(b[n:])
		if len(b[n:]) == 0 {
			return 0
		}

		valLen := p.consumeAny(b[n:], qs, lvl)
		if valLen == 0 {
			return 0
		}

		if queryMatched != -1 {
			q := qs[queryMatched]
			if len(q.SearchVals) == 0 {
				p.querySatisfied = true
			}

			for _, val := range q.SearchVals {
				if bytes.Equal(val, bytes.TrimSpace(b[n:n+valLen])) {
					p.querySatisfied = true
				}
			}
		}

		n += valLen
		if len(b[n:]) == 0 {
			return 0
		}

		switch b[n] {
		case ',':
			p.popLastPath(qs)
			n++
			p.ib++
			continue
		case '}':
			p.popLastPath(qs)
			p.ib++
			return n + 1
		default:
			return 0
		}
	}

	return 0
}

func (p *parserState) consumeAny(b []byte, qs []query, lvl int) (n int) {
	if p.maxRecursion != 0 && lvl > p.maxRecursion {
		return 0
	}

	if len(qs) == 0 {
		p.querySatisfied = true
	}

	n += p.consumeSpace(b)
	if len(b[n:]) == 0 {
		return 0
	}

	var t, rv int
	switch b[n] {
	case '"':
		n++
		p.ib++
		rv = p.consumeString(b[n:])
		t = TokString
	case '[':
		n++
		p.ib++
		rv = p.consumeArray(b[n:], qs, lvl+1)
		t = TokArray
	case '{':
		n++
		p.ib++
		rv = p.consumeObject(b[n:], qs, lvl+1)
		t = TokObject
	case 't':
		rv = p.consumeConst(b[n:], []byte("true"))
		t = TokTrue
	case 'f':
		rv = p.consumeConst(b[n:], []byte("false"))
		t = TokFalse
	case 'n':
		rv = p.consumeConst(b[n:], []byte("null"))
		t = TokNull
	default:
		rv = p.consumeNumber(b[n:])
		t = TokNumber
	}

	if lvl == 0 {
		p.firstToken = t
	}

	if rv <= 0 {
		return 0
	}

	n += rv
	n += p.consumeSpace(b[n:])
	return n
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func isXDigit(c byte) bool {
	if isDigit(c) {
		return true
	}

	return ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}
