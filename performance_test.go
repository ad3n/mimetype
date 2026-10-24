package mimetype

import (
	"slices"
	"testing"
)

func TestCloneHierarchyIsolation(t *testing.T) {
	for _, charset := range []string{"", "utf-8", "windows-1252"} {
		t.Run(charset, func(t *testing.T) {
			ancestor := newMIME("application/octet-stream", "", nil)
			parent := newMIME("text/plain", ".txt", nil).alias("text/x-plain")
			parent.parent = ancestor
			leaf := newMIME("text/html", ".html", nil).alias("application/x-html")
			leaf.parent = parent
			first, second := leaf.cloneHierarchy(charset), leaf.cloneHierarchy(charset)
			original, other := leaf, second
			for got := first; got != nil; got = got.parent {
				if original == nil || other == nil {
					t.Fatal("unexpected hierarchy depth")
				}

				want := original.mime
				if got == first && charset != "" {
					want += "; charset=" + charset
				}

				if got.mime != want || got.extension != original.extension || !slices.Equal(got.aliases, original.aliases) {
					t.Fatalf("clone metadata differs: got %q, want %q", got.mime, want)
				}

				if got == original || got == other || got.detector != nil || got.children != nil {
					t.Fatal("clone shares a node or retains detector state")
				}

				got.Extend(func([]byte, uint32) bool { return false }, "application/x-clone", ".clone")
				if len(original.children) != 0 || len(other.children) != 0 {
					t.Fatal("extending one clone changed another hierarchy")
				}

				original, other = original.parent, other.parent
			}

			if original != nil || other != nil {
				t.Fatal("clone lost ancestors")
			}
		})
	}
}

func TestMatchCharsetDispatch(t *testing.T) {
	for _, tc := range []struct {
		mime string
		data string
		want string
	}{
		{"text/plain", "hello", "text/plain; charset=utf-8"},
		{"text/html", "<html>hello</html>", "text/html; charset=utf-8"},
		{"text/xml", "<?xml version=\"1.0\" encoding=\"iso-8859-1\"?><a/>", "text/xml; charset=iso-8859-1"},
		{"application/x-custom", "hello", "application/x-custom"},
	} {
		t.Run(tc.mime, func(t *testing.T) {
			m := newMIME(tc.mime, "", nil)
			got := m.match([]byte(tc.data), defaultLimit)
			if got.String() != tc.want {
				t.Fatalf("got %q, want %q", got.String(), tc.want)
			}

			if tc.mime == "application/x-custom" && got != m {
				t.Fatal("non-text detection no longer returns the original node")
			}
		})
	}
}
