package render

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFallback(t *testing.T) {
	var buf bytes.Buffer
	if err := Fallback("Home", "article").Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{"<title>Home</title>", "<h1>Home</h1>", "Page type: article"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered HTML missing %q:\n%s", want, html)
		}
	}
}
