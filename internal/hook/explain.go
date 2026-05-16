package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/micaelmalta/token-crunch/internal/compress"
	"github.com/micaelmalta/token-crunch/internal/session"
)

type ExplainReport struct {
	ToolName   string   `json:"tool_name"`
	Original   int      `json:"original"`
	Final      int      `json:"final"`
	Modified   bool     `json:"modified"`
	Strategies []string `json:"strategies"`
	Trace      []string `json:"trace"`
	Output     string   `json:"output"`
}

func Explain(path string) error {
	data, err := readInput(path)
	if err != nil {
		return err
	}

	var post postInput
	if err := json.Unmarshal(data, &post); err == nil && len(post.ToolResponse) > 0 {
		content := toolText(post.ToolResponse)
		if content == "" {
			return fmt.Errorf("no extractable tool response text")
		}
		return printExplain(post.ToolName, post.ToolInput, content)
	}

	var replay ReplayEntry
	if err := json.Unmarshal(data, &replay); err != nil {
		return err
	}
	if replay.Content == "" {
		return fmt.Errorf("no content field found")
	}
	return printExplain(replay.ToolName, replay.ToolInput, replay.Content)
}

func readInput(path string) ([]byte, error) {
	if path == "-" || strings.TrimSpace(path) == "" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(path)
}

func printExplain(toolName string, toolInput map[string]any, content string) error {
	result := compress.Run(session.NewEphemeral(), toolName, toolInput, content, compress.DefaultTokenBudget)
	report := ExplainReport{
		ToolName:   toolName,
		Original:   result.OrigSize,
		Final:      result.FinalSize,
		Modified:   result.WasModified,
		Strategies: result.Strategies,
		Trace:      result.Trace,
		Output:     result.Output,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
