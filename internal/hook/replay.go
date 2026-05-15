package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mmalta/token-crunch/internal/compress"
	"github.com/mmalta/token-crunch/internal/session"
)

// ReplayEntry is a single line in a session log file.
type ReplayEntry struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Content   string         `json:"content"`
}

type ReplayReport struct {
	Entries []ReplayResult `json:"entries"`
	Total   ReplayTotal    `json:"total"`
}

type ReplayResult struct {
	Line       int      `json:"line"`
	ToolName   string   `json:"tool_name"`
	Original   int      `json:"original"`
	Final      int      `json:"final"`
	Saved      int      `json:"saved"`
	Strategies []string `json:"strategies"`
}

type ReplayTotal struct {
	Entries  int     `json:"entries"`
	Original int     `json:"original"`
	Final    int     `json:"final"`
	Saved    int     `json:"saved"`
	Ratio    float64 `json:"ratio"`
}

// Replay replays a session log file with the current compression settings,
// printing a before/after comparison for each entry.
func Replay(logPath string) error {
	report, err := collectReplay(logPath)
	if err != nil {
		return err
	}
	for _, entry := range report.Entries {
		strategies := strings.Join(entry.Strategies, "+")
		if strategies == "" {
			strategies = "none"
		}
		fmt.Printf("entry %d  tool=%-20s  orig=%6d  final=%6d  saved=%6d  strategy=%s\n",
			entry.Line, entry.ToolName, entry.Original, entry.Final, entry.Saved, strategies)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("TOTAL  entries=%d  orig=%d  final=%d  saved=%d  ratio=%.1f%%\n",
		report.Total.Entries, report.Total.Original, report.Total.Final, report.Total.Saved, report.Total.Ratio)
	return nil
}

func ReplayJSON(logPath string) error {
	report, err := collectReplay(logPath)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func collectReplay(logPath string) (ReplayReport, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return ReplayReport{}, err
	}
	defer func() { _ = f.Close() }()

	store := newEphemeralStore()

	lineNum := 0
	report := ReplayReport{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4<<20), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lineNum++

		var entry ReplayEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Fprintf(os.Stderr, "line %d: skipping malformed entry: %v\n", lineNum, err)
			continue
		}

		result := compress.Run(store, entry.ToolName, entry.ToolInput, entry.Content, compress.DefaultTokenBudget)
		savings := result.OrigSize - result.FinalSize
		report.Entries = append(report.Entries, ReplayResult{
			Line:       lineNum,
			ToolName:   entry.ToolName,
			Original:   result.OrigSize,
			Final:      result.FinalSize,
			Saved:      savings,
			Strategies: result.Strategies,
		})
		report.Total.Original += result.OrigSize
		report.Total.Final += result.FinalSize
	}
	if err := scanner.Err(); err != nil {
		return ReplayReport{}, err
	}

	report.Total.Entries = lineNum
	report.Total.Saved = report.Total.Original - report.Total.Final
	if report.Total.Original > 0 {
		report.Total.Ratio = float64(report.Total.Saved) / float64(report.Total.Original) * 100
	}
	return report, nil
}

// newEphemeralStore returns a zero-value in-memory store (no disk backing).
func newEphemeralStore() *session.Store {
	return session.NewEphemeral()
}
