package json

import "testing"

func TestResetReleasesInput(t *testing.T) {
	for _, capacity := range []int{0, 8, 128, 256} {
		paths := make([][]byte, capacity)
		for i := range paths {
			paths[i] = []byte("retained input")
		}

		p := parserState{currPath: paths[:0], maxRecursion: maxRecursion, ib: 42, firstToken: TokObject, querySatisfied: true}
		p.reset()
		if p.ib != 0 || p.firstToken != TokInvalid || p.querySatisfied || len(p.currPath) != 0 || p.maxRecursion != maxRecursion {
			t.Fatal("parser state was not reset")
		}

		if capacity > 128 {
			if p.currPath != nil {
				t.Fatal("oversized path buffer retained")
			}

			continue
		}

		if cap(p.currPath) != capacity {
			t.Fatal("small path buffer was discarded")
		}

		for _, path := range p.currPath[:cap(p.currPath)] {
			if path != nil {
				t.Fatal("input retained beyond slice length")
			}
		}
	}
}
