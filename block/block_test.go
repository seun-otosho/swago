package block

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
)

type testParagraph struct{ Text string }

func (testParagraph) BlockType() string { return "paragraph" }
func (p testParagraph) Render() templ.Component {
	return templ.Raw("<p>" + p.Text + "</p>")
}

func TestDecode(t *testing.T) {
	Register[testParagraph]()

	stream, err := Decode([]byte(`[
		{"type":"paragraph","id":"a1","value":{"Text":"hello"}},
		{"type":"paragraph","id":"a2","value":{"Text":"world"}}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(stream) != 2 {
		t.Fatalf("got %d blocks, want 2", len(stream))
	}
	if stream[0].ID != "a1" || stream[1].ID != "a2" {
		t.Fatalf("ids not preserved: %+v", stream)
	}
	p0, ok := stream[0].Block.(*testParagraph)
	if !ok || p0.Text != "hello" {
		t.Fatalf("got %+v, want testParagraph{Text: hello}", stream[0].Block)
	}

	empty, err := Decode(nil)
	if err != nil || empty != nil {
		t.Fatalf("empty data should decode to nil, nil: %v %v", empty, err)
	}

	if _, err := Decode([]byte(`[{"type":"unknown","value":{}}]`)); err == nil {
		t.Fatal("unregistered type should error")
	}
}

func TestRender(t *testing.T) {
	stream, err := Decode([]byte(`[{"type":"paragraph","id":"a1","value":{"Text":"hi"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := stream[0].Block.Render().Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "<p>hi</p>" {
		t.Fatalf("got %q", buf.String())
	}
}
