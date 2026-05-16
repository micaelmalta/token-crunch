package compress

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/micaelmalta/token-crunch/internal/config"
)

// DefaultTokenBudget is the max token estimate before relevance trimming kicks in.
const DefaultTokenBudget = 2000

// TrimByRelevance scores each line of content against queryTerms and drops
// low-scoring lines when content exceeds tokenBudget tokens.
// Always preserves: first line, last line, any line containing a query term verbatim.
// Returns (trimmed, wasTrimmed).
func TrimByRelevance(content string, queryTerms []string, tokenBudget int) (string, bool) {
	return trimByRelevance(content, queryTerms, tokenBudget, config.Load().MinRelevanceLines)
}

func trimByRelevance(content string, queryTerms []string, tokenBudget int, minLines int) (string, bool) {
	if tokenBudget <= 0 {
		tokenBudget = DefaultTokenBudget
	}
	lines := strings.Split(content, "\n")
	nonEmpty := 0
	for _, l := range lines {
		if l != "" {
			nonEmpty++
		}
	}
	if minLines <= 0 {
		minLines = 10
	}
	if estimateTokens(content) <= tokenBudget || nonEmpty < minLines {
		return content, false
	}

	// Build IDF from the corpus of lines
	idf := computeIDF(lines)
	querySet := normaliseTerms(queryTerms)

	type scoredLine struct {
		idx   int
		score float64
		line  string
		keep  bool // forced keep
	}
	scored := make([]scoredLine, len(lines))
	for i, l := range lines {
		forced := i == 0 || i == len(lines)-1 || containsAny(l, querySet)
		scored[i] = scoredLine{
			idx:   i,
			line:  l,
			score: tfIDF(l, querySet, idf),
			keep:  forced,
		}
	}

	// Sort by score descending, keep top lines until budget is met
	byScore := make([]*scoredLine, len(scored))
	for i := range scored {
		byScore[i] = &scored[i]
	}
	sort.Slice(byScore, func(a, b int) bool {
		return byScore[a].score > byScore[b].score
	})

	budget := tokenBudget
	for _, s := range byScore {
		if s.keep {
			budget -= estimateTokens(s.line)
			continue
		}
		if budget > 0 {
			s.keep = true
			budget -= estimateTokens(s.line)
		}
	}

	// Reconstruct in original order
	var out []string
	dropped := 0
	for i, s := range scored {
		if s.keep {
			if dropped > 0 {
				out = append(out, strings.Repeat(" ", indent(lines[i]))+
					"[… "+itoa(dropped)+" lines omitted …]")
				dropped = 0
			}
			out = append(out, s.line)
		} else {
			dropped++
		}
	}
	if dropped > 0 {
		out = append(out, "[… "+itoa(dropped)+" lines omitted …]")
	}

	return strings.Join(out, "\n"), true
}

// ExtractQueryTerms flattens tool_input values into a token list.
func ExtractQueryTerms(toolInput map[string]any) []string {
	var terms []string
	for _, v := range toolInput {
		terms = append(terms, strings.Fields(fmt.Sprintf("%v", v))...)
	}
	return terms
}

func computeIDF(lines []string) map[string]float64 {
	df := map[string]int{}
	n := len(lines)
	for _, l := range lines {
		seen := map[string]bool{}
		for _, tok := range tokeniseWords(l) {
			if !seen[tok] {
				df[tok]++
				seen[tok] = true
			}
		}
	}
	idf := make(map[string]float64, len(df))
	for term, freq := range df {
		idf[term] = math.Log(float64(n+1) / float64(freq+1))
	}
	return idf
}

func tfIDF(line string, querySet map[string]bool, idf map[string]float64) float64 {
	tokens := tokeniseWords(line)
	if len(tokens) == 0 {
		return 0
	}
	freq := map[string]int{}
	for _, t := range tokens {
		freq[t]++
	}
	var score float64
	for term := range querySet {
		if f, ok := freq[term]; ok {
			tf := float64(f) / float64(len(tokens))
			score += tf * idf[term]
		}
	}
	return score
}

func normaliseTerms(terms []string) map[string]bool {
	set := map[string]bool{}
	for _, t := range terms {
		set[strings.ToLower(t)] = true
	}
	return set
}

func containsAny(line string, terms map[string]bool) bool {
	lower := strings.ToLower(line)
	for t := range terms {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func tokeniseWords(s string) []string {
	s = strings.ToLower(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		return ('a' > r || r > 'z') && ('0' > r || r > '9') && r != '_'
	})
}

// EstimateTokens approximates the Claude token count for s.
// Plain prose averages ~0.75 words/token.  Dense technical content (paths,
// stack frames, symbol-heavy identifiers) tokenises more finely, so we apply a
// lower words-per-token ratio for those to avoid under-counting.
func EstimateTokens(s string) int {
	words := strings.Fields(s)
	if len(words) == 0 {
		return 0
	}
	ratio := technicalRatio(words)
	return int(math.Ceil(float64(len(words)) / ratio))
}

func estimateTokens(s string) int { return EstimateTokens(s) }

// technicalRatio returns the estimated words-per-token ratio for the token
// slice.  Interpolated between 0.60 (fully technical) and 0.75 (plain prose)
// based on the fraction of words that carry path separators, colons, parens,
// uppercase, or long underscored identifiers.
func technicalRatio(words []string) float64 {
	const (
		proseRatio     = 0.75
		technicalRatio = 0.60
		sampleMax      = 40
	)
	sample := words
	if len(sample) > sampleMax {
		sample = sample[:sampleMax]
	}
	technical := 0
	for _, w := range sample {
		if isTechnicalWord(w) {
			technical++
		}
	}
	frac := float64(technical) / float64(len(sample))
	return proseRatio - frac*(proseRatio-technicalRatio)
}

// isTechnicalWord reports whether w looks like a dense technical token.
func isTechnicalWord(w string) bool {
	for _, c := range w {
		switch {
		case c == '/' || c == '\\' || c == '.':
			return true
		case c == ':' || c == '(' || c == ')':
			return true
		case 'A' <= c && c <= 'Z':
			return true
		case c == '_' && len(w) > 4:
			return true
		}
	}
	return false
}

func indent(line string) int {
	n := 0
	for _, c := range line {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
