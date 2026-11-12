//go:build (!amd64 && !arm64) || purego

package json

func indexStringSpecialBlock(raw []byte) int {
	return indexStringSpecialGo(raw)
}
