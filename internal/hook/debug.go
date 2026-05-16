package hook

import (
	"fmt"
	"os"
	"strings"

	"github.com/micaelmalta/token-crunch/internal/config"
)

func debugf(format string, args ...any) {
	if !config.Load().Debug {
		return
	}
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
	fmt.Fprintf(os.Stderr, "token-crunch debug: %s\n", msg)
}
