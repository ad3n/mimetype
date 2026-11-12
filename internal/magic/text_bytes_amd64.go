//go:build !purego

package magic

//go:noescape
func textBytes(raw []byte) bool
