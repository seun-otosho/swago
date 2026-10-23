package blocks_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/seun-otosho/swago/block"
	"github.com/seun-otosho/swago/examples/site/blocks"
	"github.com/seun-otosho/swago/render"
)

func TestParagraphStream(t *testing.T) {
	block.Register[blocks.Paragraph]()

	stream, err := block.Decode([]byte(`[
		{"type":"paragraph","id":"b1","value":{"Text":"Hello, swago."}},
		{"type":"paragraph","id":"b2","value":{"Text":"Second block."}}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(stream) != 2 {
		t.Fatalf("got %d blocks, want 2", len(stream))
	}

	var buf bytes.Buffer
	if err := render.Stream(stream).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	want := "<p>Hello, swago.</p><p>Second block.</p>"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
