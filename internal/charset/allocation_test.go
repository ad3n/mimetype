package charset

import "testing"

func TestHTMLAttributeTracking(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{`<meta CHARSET="utf-8" charset="windows-1252">`, "utf-8"},
		{`<meta charset="" CHARSET="utf-8">`, ""},
		{`<meta CONTENT="text/html; charset=utf-8" content="text/html; charset=windows-1252" HTTP-EQUIV="content-type">`, "utf-8"},
		{`<meta http-equiv="other" HTTP-EQUIV="content-type" content="text/html; charset=utf-8">`, ""},
		{`<meta ignored="x" IGNORED="y" charset="CuStOm">`, "CuStOm"},
		{`<meta charſet="utf-8">`, ""},
	} {
		if got := fromHTML([]byte(tc.input)); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCharsetStringOwnsUnknownInput(t *testing.T) {
	raw := []byte("CuStOm")
	got := charsetString(raw)
	clear(raw)
	if got != "CuStOm" {
		t.Fatalf("charset changed: %q", got)
	}
}
