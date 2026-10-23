// Package blocks holds this project's StreamField block types.
package blocks

import "github.com/a-h/templ"

// Paragraph is the simplest possible block: one text field.
type Paragraph struct {
	Text string `json:"Text"`
}

func (Paragraph) BlockType() string { return "paragraph" }

func (p Paragraph) Render() templ.Component { return paragraph(p.Text) }
