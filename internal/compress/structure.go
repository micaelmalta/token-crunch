package compress

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CollapseStructure detects output shape and applies structure-specific reduction.
// toolArgs is the map of arguments passed to the tool (used for tabular column hints).
// Returns (collapsed, wasCollapsed).
func CollapseStructure(content string, toolArgs map[string]any) (string, bool) {
	if out, ok := tryJSONArray(content); ok {
		return out, true
	}
	if out, ok := tryJSONObject(content); ok {
		return out, true
	}
	if out, ok := tryNDJSON(content); ok {
		return out, true
	}
	if out, ok := tryCSV(content); ok {
		return out, true
	}
	if out, ok := tryUnifiedDiff(content); ok {
		return out, true
	}
	if out, ok := tryDiagnostics(content); ok {
		return out, true
	}
	if out, ok := tryCoverage(content); ok {
		return out, true
	}
	if out, ok := tryPackageLog(content); ok {
		return out, true
	}
	if out, ok := tryMarkup(content); ok {
		return out, true
	}
	if out, ok := tryStackTrace(content); ok {
		return out, true
	}
	if out, ok := tryFileTree(content); ok {
		return out, true
	}
	if out, ok := tryTestOutput(content); ok {
		return out, true
	}
	if out, ok := tryLogStream(content); ok {
		return out, true
	}
	if out, ok := tryTabular(content, toolArgs); ok {
		return out, true
	}
	if out, ok := tryYAML(content); ok {
		return out, true
	}
	if out, ok := tryTOML(content); ok {
		return out, true
	}
	return content, false
}

// --- JSON / records ---

func tryJSONArray(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil || len(items) <= 3 {
		return "", false
	}
	var sb strings.Builder
	sb.WriteString(string(items[0]))
	sb.WriteString(",\n")
	sb.WriteString(string(items[1]))
	sb.WriteString(",\n")
	fmt.Fprintf(&sb, "…and %d more items", len(items)-2)
	return sb.String(), true
}

func tryJSONObject(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") {
		return "", false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil || len(obj) <= 8 {
		return "", false
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sortStrings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&sb, "%s: %s\n", k, obj[k])
	}
	fmt.Fprintf(&sb, "…and %d more keys", len(keys)-5)
	return strings.TrimRight(sb.String(), "\n"), true
}

func tryNDJSON(content string) (string, bool) {
	lines := nonEmptyLines(content)
	if len(lines) <= 3 {
		return "", false
	}
	parsed := 0
	for _, l := range lines {
		var obj map[string]any
		if json.Unmarshal([]byte(l), &obj) == nil {
			parsed++
		}
	}
	if parsed < len(lines) {
		return "", false
	}
	return strings.Join(lines[:2], "\n") + fmt.Sprintf("\n…and %d more JSON records", len(lines)-2), true
}

func tryCSV(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.Contains(trimmed, ",") {
		return "", false
	}
	r := csv.NewReader(strings.NewReader(trimmed))
	records, err := r.ReadAll()
	if err != nil || len(records) <= 4 || len(records[0]) < 3 {
		return "", false
	}
	cols := len(records[0])
	for _, rec := range records[1:] {
		if len(rec) != cols {
			return "", false
		}
	}
	var sb strings.Builder
	sb.WriteString(strings.Join(records[0], ","))
	sb.WriteByte('\n')
	sb.WriteString(strings.Join(records[1], ","))
	sb.WriteByte('\n')
	sb.WriteString(strings.Join(records[2], ","))
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "…and %d more rows", len(records)-3)
	return sb.String(), true
}

func tryYAML(content string) (string, bool) {
	lines := nonEmptyLines(content)
	if len(lines) < 12 {
		return "", false
	}
	keyRe := regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+:\s+`)
	keyLines := 0
	for _, l := range lines {
		if keyRe.MatchString(l) {
			keyLines++
		}
	}
	if keyLines < len(lines)/2 {
		return "", false
	}
	kept := append([]string{}, lines[:5]...)
	kept = append(kept, fmt.Sprintf("…and %d more YAML lines", len(lines)-5))
	return strings.Join(kept, "\n"), true
}

func tryTOML(content string) (string, bool) {
	lines := nonEmptyLines(content)
	if len(lines) < 10 {
		return "", false
	}
	sections, keys := 0, 0
	var kept []string
	for _, l := range lines {
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			sections++
			if len(kept) < 6 {
				kept = append(kept, l)
			}
		} else if strings.Contains(l, "=") {
			keys++
		}
	}
	if sections < 2 || keys < 6 {
		return "", false
	}
	kept = append(kept, fmt.Sprintf("… %d sections, %d keys", sections, keys))
	return strings.Join(kept, "\n"), true
}

// --- Diffs / diagnostics ---

func tryUnifiedDiff(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 20 || !strings.Contains(content, "@@") {
		return "", false
	}
	plus, minus := 0, 0
	var kept []string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git "), strings.HasPrefix(l, "+++ "), strings.HasPrefix(l, "--- "), strings.HasPrefix(l, "@@"):
			kept = append(kept, l)
		case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			plus++
			if plus <= 3 {
				kept = append(kept, l)
			}
		case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			minus++
			if minus <= 3 {
				kept = append(kept, l)
			}
		}
	}
	if plus+minus < 8 {
		return "", false
	}
	kept = append(kept, fmt.Sprintf("… %d additions, %d deletions", plus, minus))
	return strings.Join(kept, "\n"), true
}

var diagnosticRe = regexp.MustCompile(`(?m)^\S+\.(go|ts|tsx|js|jsx|rs|py|java|c|cc|cpp|h):\d+(:\d+)?:`)

func tryDiagnostics(content string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var matches []string
	for _, l := range lines {
		if diagnosticRe.MatchString(l) {
			matches = append(matches, l)
		}
	}
	if len(matches) < 8 {
		return "", false
	}
	kept := append([]string{}, matches[:5]...)
	kept = append(kept, fmt.Sprintf("…and %d more diagnostics", len(matches)-5))
	return strings.Join(kept, "\n"), true
}

func tryCoverage(content string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) < 8 || !strings.Contains(strings.ToLower(content), "coverage") {
		return "", false
	}
	var low []string
	total := ""
	for _, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "total") {
			total = l
		}
		if strings.Contains(l, "%") && (strings.Contains(l, " 0.") || strings.Contains(l, " 1.") || strings.Contains(l, " 2.") || strings.Contains(l, " 3.") || strings.Contains(l, " 4.")) {
			low = append(low, l)
		}
	}
	if total == "" && len(low) == 0 {
		return "", false
	}
	if len(low) > 5 {
		low = append(low[:5], fmt.Sprintf("…and %d more low-coverage entries", len(low)-5))
	}
	if total != "" {
		low = append(low, total)
	}
	return strings.Join(low, "\n"), true
}

func tryPackageLog(content string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) < 12 {
		return "", false
	}
	lower := strings.ToLower(content)
	ecosystem := packageLogEcosystem(lower)
	if ecosystem == "" {
		return "", false
	}
	var important []string
	warnings, errors, installed, removed, vulnerabilities := 0, 0, 0, 0, 0
	for _, l := range lines {
		ll := strings.ToLower(l)
		switch {
		case strings.Contains(ll, "error") || strings.Contains(ll, "err!"):
			errors++
		case strings.Contains(ll, "warn") || strings.Contains(ll, "warning"):
			warnings++
		}
		if strings.Contains(ll, "added ") || strings.Contains(ll, "installed ") || strings.Contains(ll, "successfully installed") || strings.Contains(ll, "compiling ") {
			installed++
		}
		if strings.Contains(ll, "removed ") || strings.Contains(ll, "uninstalled ") {
			removed++
		}
		if strings.Contains(ll, "vulnerab") {
			vulnerabilities++
		}
		if strings.Contains(ll, "error") || strings.Contains(ll, "err!") || strings.Contains(ll, "warn") || strings.Contains(ll, "warning") || strings.Contains(ll, "added ") || strings.Contains(ll, "removed ") || strings.Contains(ll, "audited ") || strings.Contains(ll, "vulnerab") || strings.Contains(ll, "successfully installed") || strings.Contains(ll, "compiling ") {
			important = append(important, l)
		}
	}
	if len(important) == 0 {
		return "", false
	}
	prefix := fmt.Sprintf("[package-log: %s | %d error(s), %d warning(s), %d install/build line(s), %d removal(s), %d vulnerability line(s)]", ecosystem, errors, warnings, installed, removed, vulnerabilities)
	if len(important) > 8 {
		important = append(important[:8], fmt.Sprintf("…and %d more package log lines", len(important)-8))
	}
	return prefix + "\n" + strings.Join(important, "\n"), true
}

func packageLogEcosystem(lower string) string {
	switch {
	case strings.Contains(lower, "npm ") || strings.Contains(lower, "npm warn") || strings.Contains(lower, "npm err"):
		return "npm"
	case strings.Contains(lower, "pnpm ") || strings.Contains(lower, " warn ") || strings.Contains(lower, " err_pnpm"):
		return "pnpm"
	case strings.Contains(lower, "yarn ") || strings.Contains(lower, "➤ yn"):
		return "yarn"
	case strings.Contains(lower, "pip ") || strings.Contains(lower, "successfully installed"):
		return "pip"
	case strings.Contains(lower, "cargo ") || strings.Contains(lower, "compiling ") || strings.Contains(lower, "finished `"):
		return "cargo"
	default:
		return ""
	}
}

func tryMarkup(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	if !(strings.HasPrefix(lower, "<") && (strings.Contains(lower, "</") || strings.Contains(lower, "/>"))) {
		return "", false
	}
	tagRe := regexp.MustCompile(`<\s*([a-zA-Z0-9:_-]+)`)
	matches := tagRe.FindAllStringSubmatch(trimmed, -1)
	if len(matches) < 12 {
		return "", false
	}
	counts := map[string]int{}
	var order []string
	for _, m := range matches {
		tag := strings.ToLower(m[1])
		if counts[tag] == 0 {
			order = append(order, tag)
		}
		counts[tag]++
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[markup: %d tags]\n", len(matches))
	for i, tag := range order {
		if i >= 6 {
			break
		}
		fmt.Fprintf(&sb, "%s: %d\n", tag, counts[tag])
	}
	if len(order) > 6 {
		fmt.Fprintf(&sb, "…and %d more tag types", len(order)-6)
	}
	return strings.TrimRight(sb.String(), "\n"), true
}

// --- Stack trace ---

// stackFrameRe matches actual stack frame lines across languages.
// Go:     "\t/path/file.go:42 +0x1a3"  (tab + path + colon + digits)
// JS/TS:  "    at Object.foo (file.js:1:2)"
// Python: "  File \"foo.py\", line 4"
// Java:   "	at com.example.Foo.bar(Foo.java:42)"
var stackFrameRe = regexp.MustCompile(`(?m)^\t\S.*:\d+|^\s+at \w|^\s+File "`)
var panicRe = regexp.MustCompile(`(?i)(panic:|exception in thread|traceback \(most recent|goroutine \d+ \[)`)

func tryStackTrace(content string) (string, bool) {
	// Require at least one panic/exception header — avoids false positives on source code.
	if !panicRe.MatchString(content) {
		return "", false
	}

	lines := strings.Split(content, "\n")
	frameCount := 0
	for _, l := range lines {
		if stackFrameRe.MatchString(l) {
			frameCount++
		}
	}
	if frameCount < 5 {
		return "", false
	}

	// Partition: header (before first frame line), frames (frame region), trailer (after last frame line)
	var header, trailer []string
	firstFrame, lastFrame := -1, -1
	for i, l := range lines {
		if stackFrameRe.MatchString(l) || panicRe.MatchString(l) {
			if firstFrame < 0 {
				firstFrame = i
			}
			lastFrame = i
		}
	}
	if firstFrame < 0 {
		return content, true // should not happen — we counted frames earlier
	}
	header = lines[:firstFrame]
	frames := lines[firstFrame : lastFrame+1]
	trailer = lines[lastFrame+1:]
	// Keep first frame + last 3
	if len(frames) > 5 {
		kept := append([]string{}, frames[:2]...)
		kept = append(kept, fmt.Sprintf("    … (%d frames omitted)", len(frames)-5))
		kept = append(kept, frames[len(frames)-3:]...)
		frames = kept
	}

	out := strings.Join(append(append(header, frames...), trailer...), "\n")
	return out, true
}

// --- File tree ---

func tryFileTree(content string) (string, bool) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) < 15 {
		return "", false
	}
	// Heuristic: file tree lines have consistent indent patterns and short content
	treeLines := 0
	for _, l := range lines {
		if regexp.MustCompile(`^[\s│├└─]+\S`).MatchString(l) || regexp.MustCompile(`^\s{2,}\S`).MatchString(l) {
			treeLines++
		}
	}
	if float64(treeLines) < float64(len(lines))*0.5 {
		return "", false
	}

	// Group by top-level dir, collapse subtrees > 10 items
	type group struct {
		prefix string
		lines  []string
	}
	var groups []group
	var cur *group
	for _, l := range lines {
		if leadingSpaces(l) == 0 && cur != nil {
			groups = append(groups, *cur)
			cur = nil
		}
		if cur == nil {
			cur = &group{prefix: l}
		} else {
			cur.lines = append(cur.lines, l)
		}
	}
	if cur != nil {
		groups = append(groups, *cur)
	}

	var sb strings.Builder
	for _, g := range groups {
		sb.WriteString(g.prefix)
		sb.WriteByte('\n')
		if len(g.lines) > 10 {
			for _, l := range g.lines[:3] {
				sb.WriteString(l)
				sb.WriteByte('\n')
			}
			fmt.Fprintf(&sb, "  … (%d more items)\n", len(g.lines)-3)
		} else {
			for _, l := range g.lines {
				sb.WriteString(l)
				sb.WriteByte('\n')
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n"), true
}

// --- Test output ---

var testPassRe = regexp.MustCompile(`(?i)^(ok |PASS|--- PASS:|\.+\s*$|test \w+ \.\.\. ok)`)
var testFailRe = regexp.MustCompile(`(?i)(FAIL|Error|panic|--- FAIL|^\s+\S+_test\.go:\d+)`)
var testSummaryRe = regexp.MustCompile(`(?i)(Tests run|passed|failed|skipped|ok\s+\S+\s+[\d.]+s)`)

func tryTestOutput(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	passCount, failCount := 0, 0
	for _, l := range lines {
		if testPassRe.MatchString(l) {
			passCount++
		}
		if testFailRe.MatchString(l) {
			failCount++
		}
	}
	// Only collapse if there are many passing tests
	if passCount < 5 {
		return "", false
	}

	var kept []string
	for _, l := range lines {
		if testFailRe.MatchString(l) || testSummaryRe.MatchString(l) {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		kept = append(kept, fmt.Sprintf("[%d tests passed, 0 failures]", passCount))
	} else {
		kept = append([]string{fmt.Sprintf("[%d passed, %d failures shown below]", passCount, failCount)}, kept...)
	}
	return strings.Join(kept, "\n"), true
}

// --- Log stream ---

// Matches: "2024-01-15 10:00:00", "2024/01/15T10:00:00", "[10:00:00]", unix epoch
var timestampRe = regexp.MustCompile(`^\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}|^\d{4}[-/]\d{2}[-/]\d{2}|^\[\d{2}:\d{2}:\d{2}\]|^\d{10,}`)

func tryLogStream(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	tsLines := 0
	for _, l := range lines {
		if timestampRe.MatchString(l) {
			tsLines++
		}
	}
	if float64(tsLines) < float64(len(lines))*0.5 || len(lines) < 10 {
		return "", false
	}

	// Deduplicate: track (message_without_ts → count)
	type entry struct {
		line  string
		count int
	}
	var order []string
	seen := map[string]*entry{}
	for _, l := range lines {
		// Strip timestamp prefix (first word or bracketed prefix)
		msg := timestampRe.ReplaceAllString(l, "")
		msg = strings.TrimSpace(msg)
		if e, ok := seen[msg]; ok {
			e.count++
		} else {
			seen[msg] = &entry{line: l, count: 1}
			order = append(order, msg)
		}
	}

	var sb strings.Builder
	for _, msg := range order {
		e := seen[msg]
		if e.count > 1 {
			fmt.Fprintf(&sb, "%s  [×%d]\n", e.line, e.count)
		} else {
			sb.WriteString(e.line)
			sb.WriteByte('\n')
		}
	}
	out := strings.TrimRight(sb.String(), "\n")
	if len(out) >= len(content) {
		return "", false // dedup didn't help — don't bloat
	}
	return out, true
}

// --- Tabular ---

var columnSepRe = regexp.MustCompile(`\s{2,}|\t`)

func tryTabular(content string, toolArgs map[string]any) (string, bool) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) < 4 {
		return "", false
	}
	// Detect header row: first line with multiple space/tab-separated columns
	header := lines[0]
	cols := columnSepRe.Split(header, -1)
	if len(cols) < 4 {
		return "", false
	}
	// Check that most lines have similar column counts
	aligned := 0
	for _, l := range lines[1:] {
		if len(columnSepRe.Split(l, -1)) >= len(cols)-1 {
			aligned++
		}
	}
	if float64(aligned) < float64(len(lines)-1)*0.6 {
		return "", false
	}

	// Collect referenced column names from tool args
	argStr := argsToString(toolArgs)
	argTokens := strings.Fields(strings.ToLower(argStr))

	// Score each column by presence of its name in args
	keepCols := make([]bool, len(cols))
	keepCols[0] = true // always keep first column
	for i, col := range cols {
		colLower := strings.ToLower(strings.TrimSpace(col))
		for _, tok := range argTokens {
			if strings.Contains(colLower, tok) || strings.Contains(tok, colLower) {
				keepCols[i] = true
				break
			}
		}
	}
	// If no columns selected beyond first, keep all (no useful signal)
	anyExtra := false
	for i := 1; i < len(keepCols); i++ {
		if keepCols[i] {
			anyExtra = true
			break
		}
	}
	if !anyExtra {
		return "", false
	}

	var sb strings.Builder
	for _, l := range lines {
		parts := columnSepRe.Split(l, -1)
		var kept []string
		for i, p := range parts {
			if i < len(keepCols) && keepCols[i] {
				kept = append(kept, p)
			}
		}
		sb.WriteString(strings.Join(kept, "  "))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n"), true
}

func leadingSpaces(s string) int {
	for i, c := range s {
		if c != ' ' && c != '\t' && c != '│' && c != '├' && c != '└' && c != '─' {
			return i
		}
	}
	return len(s)
}

func argsToString(args map[string]any) string {
	var parts []string
	for k, v := range args {
		parts = append(parts, k)
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, " ")
}

func nonEmptyLines(content string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(content), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
