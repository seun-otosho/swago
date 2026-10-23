// Package block implements StreamField-style content: an ordered list of typed, JSON
// blocks, each a Go type registered under a name, the same shape as package page's
// registry. A StreamField on disk is `[{"type":"...","id":"...","value":...}, ...]`, same
// as Wagtail's.
package block

import (
	"encoding/json"
	"fmt"

	"github.com/a-h/templ"
)

// Block is implemented by a project's block type, e.g. a Paragraph or an Image.
//
// BlockType must be implemented with a value receiver and must not depend on the
// receiver's field values (it is a constant per type, like a type tag): Register calls it
// on a zero value, the same convention as package page's Page interface.
type Block interface {
	// BlockType is the registered name, stored in each block's "type" field.
	BlockType() string
	// Render returns the block's markup.
	Render() templ.Component
}

var registry = map[string]func() Block{}

// Register adds T under the name its zero value's BlockType() returns. Call it once per
// block type, normally from a project's init().
func Register[T Block]() {
	var zero T
	registry[zero.BlockType()] = func() Block { return any(new(T)).(Block) }
}

// raw is one element of a StreamField JSON array.
type raw struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

// Instance is one decoded block in a stream: its id alongside the typed value.
type Instance struct {
	ID    string
	Block Block
}

// Decode unmarshals a StreamField JSON array (the `{type,id,value}` shape) into typed,
// registered blocks, in order. Empty or nil data decodes to a nil, empty stream.
func Decode(data []byte) ([]Instance, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var raws []raw
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("block: decoding stream: %w", err)
	}
	out := make([]Instance, len(raws))
	for i, r := range raws {
		newBlock, ok := registry[r.Type]
		if !ok {
			return nil, fmt.Errorf("block: no type registered as %q", r.Type)
		}
		b := newBlock()
		if len(r.Value) > 0 {
			if err := json.Unmarshal(r.Value, b); err != nil {
				return nil, fmt.Errorf("block: decoding %q: %w", r.Type, err)
			}
		}
		out[i] = Instance{ID: r.ID, Block: b}
	}
	return out, nil
}
