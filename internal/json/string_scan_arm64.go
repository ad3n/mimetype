//go:build !purego

package json

func indexStringSpecialBlock(raw []byte) int {
	if len(raw) < 16 {
		return indexStringSpecialGo(raw)
	}

	return indexStringSpecialNEON(raw)
}

//go:noescape
func indexStringSpecialNEON(raw []byte) int
