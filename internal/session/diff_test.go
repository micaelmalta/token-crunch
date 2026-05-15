package session

import (
	"strings"
	"testing"
)

func TestDiffRatio_identical(t *testing.T) {
	s := "hello world foo bar"
	if r := DiffRatio(s, s); r != 1.0 {
		t.Fatalf("identical strings: want 1.0, got %f", r)
	}
}

func TestDiffRatio_empty(t *testing.T) {
	if r := DiffRatio("", ""); r != 1.0 {
		t.Fatalf("both empty: want 1.0, got %f", r)
	}
}

func TestDiffRatio_disjoint(t *testing.T) {
	if r := DiffRatio("aaa bbb ccc", "xxx yyy zzz"); r != 0.0 {
		t.Fatalf("disjoint: want 0.0, got %f", r)
	}
}

func TestDiffRatio_nearMatch(t *testing.T) {
	a := strings.Repeat("foo bar baz qux ", 20)
	// Change one token out of ~80 — should be well above 0.85
	b := strings.Replace(a, "foo", "FOO", 1)
	r := DiffRatio(a, b)
	if r < NearMatchThreshold {
		t.Fatalf("near-match: want >= %f, got %f", NearMatchThreshold, r)
	}
}

func TestDiffRatio_halfMatch(t *testing.T) {
	a := "a b c d e f g h"
	b := "a b c d w x y z"
	r := DiffRatio(a, b)
	if r < 0.4 || r > 0.7 {
		t.Fatalf("half-match: want ~0.5, got %f", r)
	}
}

func TestUnifiedDiff_identical(t *testing.T) {
	d := UnifiedDiff("a\nb\nc", "a\nb\nc")
	if strings.Contains(d, "+ ") || strings.Contains(d, "- ") {
		t.Fatalf("identical: expected no changes, got:\n%s", d)
	}
}

func TestUnifiedDiff_insertion(t *testing.T) {
	d := UnifiedDiff("a\nb", "a\nnew\nb")
	if !strings.Contains(d, "+ new") {
		t.Fatalf("expected '+ new' in diff:\n%s", d)
	}
}

func TestUnifiedDiff_deletion(t *testing.T) {
	d := UnifiedDiff("a\nremove\nb", "a\nb")
	if !strings.Contains(d, "- remove") {
		t.Fatalf("expected '- remove' in diff:\n%s", d)
	}
}

func TestUnifiedDiff_replacement(t *testing.T) {
	d := UnifiedDiff("hello world", "hello earth")
	if !strings.Contains(d, "- hello world") || !strings.Contains(d, "+ hello earth") {
		t.Fatalf("expected replacement diff, got:\n%s", d)
	}
}
