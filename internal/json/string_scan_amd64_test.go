//go:build !purego

package json

import "testing"

func TestStringScannerAMD64(t *testing.T) {
	t.Logf("AVX2 enabled: %t", stringScanAVX2)
	if testing.Short() {
		t.Skip("extended scanner coverage")
	}

	t.Run("SSE2", func(t *testing.T) { checkStringScanner(t, indexStringSpecialSSE2) })
	t.Run("AVX2", func(t *testing.T) {
		if !stringScanAVX2 {
			t.Skip("CPU or OS does not support AVX2")
		}

		checkStringScanner(t, indexStringSpecialAVX2)
	})
}
