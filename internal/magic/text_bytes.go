package magic

func textBytesGo(raw []byte) bool {
	for _, b := range raw {
		if b <= 0x08 || b == 0x0B || 0x0E <= b && b <= 0x1A || 0x1C <= b && b <= 0x1F {
			return false
		}
	}

	return true
}
