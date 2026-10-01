package parse_test

import (
	"strings"
	"testing"

	"codegraphindexer/internal/parse"
)

func TestJSExportFunctionOnce(t *testing.T) {
	src := "export function foo() {\n  return 1\n}\n"
	r := parse.ParseJS(src, "a.ts")
	n := 0
	for _, s := range r.Symbols {
		if s.Name == "foo" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected 1 foo symbol, got %d: %+v", n, r.Symbols)
	}
}

func TestRustSanitizeStripsComment(t *testing.T) {
	src := "fn foo() {\n  // call(bar)\n  baz()\n}\n"
	r := parse.ParseRust(src, "a.rs")
	for _, ref := range r.Refs {
		if ref.ToName == "call" || ref.ToName == "bar" {
			t.Fatalf("comment leaked as ref: %+v", ref)
		}
	}
	found := false
	for _, ref := range r.Refs {
		if ref.ToName == "baz" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected call to baz, refs=%+v", r.Refs)
	}
	_ = strings.Contains
}
