package compress

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mmalta/token-crunch/internal/session"
)

func TestDedup_empty(t *testing.T) {
	s := session.NewEphemeral()
	out, ok := Dedup(s, "Bash", "")
	if ok || out != "" {
		t.Fatal("empty content must not compress")
	}
}

func TestDedup_firstSeen(t *testing.T) {
	s := session.NewEphemeral()
	content := "some output that hasn't been seen before"
	out, ok := Dedup(s, "Bash", content)
	if ok || out != content {
		t.Fatal("first-seen content must pass through unchanged")
	}
}

func TestDedup_exactMatch(t *testing.T) {
	s := session.NewEphemeral()
	content := strings.Repeat("line of output\n", 30) // long enough that replacement is shorter
	s.Put("Bash", content, len(content))

	out, ok := Dedup(s, "Bash", content)
	if !ok {
		t.Fatal("exact match must be compressed")
	}
	if !strings.HasPrefix(out, "[unchanged") {
		t.Fatalf("expected [unchanged...] prefix, got: %s", out)
	}
	if len(out) >= len(content) {
		t.Fatal("compressed output must be shorter than original")
	}
}

func TestDedup_exactMatchTooShort(t *testing.T) {
	// If the replacement label is longer than the original, pass through
	s := session.NewEphemeral()
	content := "hi" // shorter than "[unchanged — seen...]"
	s.Put("Bash", content, len(content))

	out, ok := Dedup(s, "Bash", content)
	if ok {
		t.Fatal("should not compress when replacement is longer than original")
	}
	if out != content {
		t.Fatal("should return original unchanged")
	}
}

func TestDedup_nearMatch(t *testing.T) {
	s := session.NewEphemeral()
	// 500 lines, only the last 2 changed — diff is much shorter than the full content.
	var baseLines []string
	for i := 0; i < 500; i++ {
		baseLines = append(baseLines, fmt.Sprintf("config.key_%03d = value_%03d", i, i))
	}
	modLines := make([]string, len(baseLines))
	copy(modLines, baseLines)
	modLines[498] = "config.key_498 = UPDATED_VALUE"
	modLines[499] = "config.key_499 = UPDATED_VALUE"

	base := strings.Join(baseLines, "\n")
	modified := strings.Join(modLines, "\n")

	ratio := session.DiffRatio(base, modified)
	if ratio < session.NearMatchThreshold {
		t.Fatalf("fixture similarity %.2f is below threshold %.2f — fix the fixture", ratio, session.NearMatchThreshold)
	}
	s.Put("Bash", base, len(base))

	out, ok := Dedup(s, "Bash", modified)
	if !ok {
		t.Fatal("near-match must be compressed to diff")
	}
	if !strings.Contains(out, "near-match") {
		t.Fatalf("expected near-match header, got: %s", out[:min(80, len(out))])
	}
	if !strings.Contains(out, "+ config.key_498 = UPDATED_VALUE") {
		t.Fatalf("expected updated line in diff, got: %s", out[:min(200, len(out))])
	}
	if len(out) >= len(modified) {
		t.Fatalf("diff must be shorter than full content: diff=%d original=%d", len(out), len(modified))
	}
}

func TestDedup_belowThreshold(t *testing.T) {
	s := session.NewEphemeral()
	a := strings.Repeat("alpha beta gamma delta\n", 10)
	b := strings.Repeat("zeta theta iota kappa\n", 10) // completely different
	s.Put("Bash", a, len(a))

	_, ok := Dedup(s, "Bash", b)
	if ok {
		t.Fatal("content below similarity threshold must not compress")
	}
}

func TestDedup_turnLabel(t *testing.T) {
	s := session.NewEphemeral()
	content := strings.Repeat("git status output line\n", 20)
	s.Put("Bash", content, len(content))
	s.IncrementTurn()
	s.IncrementTurn()

	out, ok := Dedup(s, "Bash", content)
	if !ok {
		t.Fatal("must compress exact match")
	}
	if !strings.Contains(out, "2 turn(s) ago") {
		t.Fatalf("expected turn count in label, got: %s", out)
	}
}
