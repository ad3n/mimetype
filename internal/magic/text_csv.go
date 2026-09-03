package magic

import (
	"github.com/ad3n/mimetype/internal/csv"
	"github.com/ad3n/mimetype/internal/scan"
)

// CSV matches a comma-separated values file.
func CSV(raw []byte, limit uint32) bool {
	return sv(raw, ',', limit)
}

// TSV matches a tab-separated values file.
func TSV(raw []byte, limit uint32) bool {
	return sv(raw, '\t', limit)
}

func sv(in []byte, comma byte, limit uint32) bool {
	s := scan.Bytes(in)
	r := csv.NewParser(comma, '#', &s)

	headerFields, _, hasMore := r.CountFields(false)
	if headerFields < 2 || !hasMore {
		return false
	}
	csvLines := 1 // 1 for header
	for {
		fields, _, hasMore := r.CountFields(false)
		if !hasMore && fields == 0 {
			break
		}
		if fields == headerFields {
			csvLines++
			if csvLines >= 10 {
				return true
			}
			continue
		}

		// maybeTruncated signals the input was cut at the read limit,
		// meaning the last line may be an incomplete CSV record.
		maybeTruncated := limit > 0 && uint64(len(in)) >= uint64(limit)
		if maybeTruncated && fields < headerFields && len(s) == 0 {
			break
		}
		return false
	}

	return csvLines >= 2
}
