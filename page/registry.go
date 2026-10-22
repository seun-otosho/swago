// Package page is the project-facing page-type registry: typed Go structs implementing
// Page are registered under a name, matching the base schema.Page entity's `type` column;
// their fields round-trip through the JSON `data` column via encoding/json.
package page

import (
	"encoding/json"
	"fmt"
)

// Page is implemented by a project's page-type struct, e.g. a Home or an Article.
//
// PageType must be implemented with a value receiver and must not depend on the
// receiver's field values (it is a constant per type, like a type tag): Register calls it
// on a zero value, and New/Decode construct a type from its name alone.
type Page interface {
	// PageType is the registered name, stored in the base page's `type` column.
	PageType() string
}

var registry = map[string]func() Page{}

// Register adds T under the name its zero value's PageType() returns. Call it once per
// page type, normally from a project's init().
func Register[T Page]() {
	var zero T
	registry[zero.PageType()] = func() Page { return any(new(T)).(Page) }
}

// New returns a fresh instance of the page type registered under name.
func New(name string) (Page, bool) {
	newT, ok := registry[name]
	if !ok {
		return nil, false
	}
	return newT(), true
}

// Decode unmarshals data (the base page's JSON `data` column) into a new instance of the
// page type registered under name.
func Decode(name string, data []byte) (Page, error) {
	p, ok := New(name)
	if !ok {
		return nil, fmt.Errorf("page: no type registered as %q", name)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, p); err != nil {
			return nil, fmt.Errorf("page: decoding %q: %w", name, err)
		}
	}
	return p, nil
}
