package json

const stringScanBlockSize = 4096

func indexStringSpecial(raw []byte) int {
	for offset := 0; offset < len(raw); {
		size := min(len(raw)-offset, stringScanBlockSize)
		if i := indexStringSpecialBlock(raw[offset : offset+size]); i != size {
			return offset + i
		}

		offset += size
	}

	return len(raw)
}

func indexStringSpecialGo(raw []byte) int {
	for i, c := range raw {
		if c == '"' || c == '\\' {
			return i
		}
	}

	return len(raw)
}
