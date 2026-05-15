package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func capturePayload(event string, raw []byte) {
	dir := strings.TrimSpace(os.Getenv("TOKEN_CRUNCH_CAPTURE_DIR"))
	if dir == "" || len(raw) == 0 {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	name := fmt.Sprintf("%s-%s.json", time.Now().UTC().Format("20060102T150405.000000000Z"), safeEventName(event))
	_ = os.WriteFile(filepath.Join(dir, name), raw, 0o600)
}

func safeEventName(event string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(event) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "hook"
	}
	return b.String()
}
