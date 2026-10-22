// Package tree implements treebeard-style materialized paths: base-36 digits (0-9, A-Z),
// four characters per level, so a node has at most 36^4 - 1 children.
package tree

import (
	"errors"
	"fmt"
	"strings"
)

const (
	StepLen  = 4
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxStep  = 36*36*36*36 - 1 // "ZZZZ"
)

var ErrFull = errors.New("tree: no more room for children under this node")

// Step encodes n (1..36^4-1) as a four-character path step.
func Step(n int) string {
	b := []byte("0000")
	for i := StepLen - 1; i >= 0 && n > 0; i-- {
		b[i] = alphabet[n%36]
		n /= 36
	}
	return string(b)
}

func stepValue(s string) (int, error) {
	n := 0
	for _, c := range s {
		i := strings.IndexRune(alphabet, c)
		if len(s) != StepLen || i < 0 {
			return 0, fmt.Errorf("tree: bad path step %q", s)
		}
		n = n*36 + i
	}
	return n, nil
}

// Root returns the path of the n-th root node, counting from 1.
func Root(n int) string { return Step(n) }

// Depth is the number of levels in path; a root has depth 1.
func Depth(path string) int { return len(path) / StepLen }

// Parent returns the parent path, or "" for a root.
func Parent(path string) string {
	if len(path) <= StepLen {
		return ""
	}
	return path[:len(path)-StepLen]
}

// FirstChild is the path of the first child of parent.
func FirstChild(parent string) string { return parent + Step(1) }

// NextSibling returns the path that follows last among its siblings.
func NextSibling(last string) (string, error) {
	n, err := stepValue(last[len(last)-StepLen:])
	if err != nil {
		return "", err
	}
	if n >= maxStep {
		return "", ErrFull
	}
	return last[:len(last)-StepLen] + Step(n+1), nil
}

// DescendantsPrefix is the LIKE prefix matching every descendant of path (not path itself).
func DescendantsPrefix(path string) string { return path }

// Rebase moves path from under oldPrefix to under newPrefix, for subtree moves.
func Rebase(path, oldPrefix, newPrefix string) string {
	return newPrefix + strings.TrimPrefix(path, oldPrefix)
}
