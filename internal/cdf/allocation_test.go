package cdf

import (
	"sync"
	"testing"
)

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

func TestFragmentedDetectionConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for _, size := range testSecSizes {
		data := makeFragmentedDirCDF(size)
		for range 8 {
			wg.Go(func() {
				for range 100 {
					if got := Detect(data); got != CDFTypeDoc {
						t.Errorf("got %v, want Word document", got)
						return
					}
				}
			})
		}
	}

	wg.Wait()
}
