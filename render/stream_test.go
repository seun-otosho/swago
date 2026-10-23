package render

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"

	"github.com/seun-otosho/swago/block"
)

type testParagraph struct{ Text string }

func (testParagraph) BlockType() string { return "stream-test-paragraph" }
func (p testParagraph) Render() templ.Component {
	return templ.Raw("<p>" + p.Text + "</p>")
}

func TestStream(t *testing.T) {
	block.Register[testParagraph]()

	stream, err := block.Decode([]byte(`[
		{"type":"stream-test-paragraph","id":"a1","value":{"Text":"one"}},
		{"type":"stream-test-paragraph","id":"a2","value":{"Text":"two"}}
	]`))
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Stream(stream).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "<p>one</p><p>two</p>"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
