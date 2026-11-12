//go:build !purego

package json

var stringScanAVX2 = supportsStringAVX2()

func indexStringSpecialBlock(raw []byte) int {
	if len(raw) < 16 {
		return indexStringSpecialGo(raw)
	}

	if stringScanAVX2 && len(raw) >= 32 {
		return indexStringSpecialAVX2(raw)
	}

	return indexStringSpecialSSE2(raw)
}

func supportsStringAVX2() bool

//go:noescape
func indexStringSpecialSSE2(raw []byte) int

//go:noescape
func indexStringSpecialAVX2(raw []byte) int
