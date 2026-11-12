//go:build (!amd64 && !arm64) || purego

package magic

func textBytes(raw []byte) bool {
	return textBytesGo(raw)
}
