package cdf

import "testing"

func TestDetectWithoutSSATAllocations(t *testing.T) {
	for _, secSize := range testSecSizes {
		data := makeCDF(secSize, nil, "\x05SummaryInformation", summaryStream("Microsoft Office Word", false))
		if got := Detect(data); got != CDFTypeDoc {
			t.Fatalf("got %v, want Word document", got)
		}

		if allocs := testing.AllocsPerRun(100, func() { Detect(data) }); allocs != 0 {
			t.Fatalf("sector size %d: got %g allocations, want zero", secSize, allocs)
		}
	}
}
