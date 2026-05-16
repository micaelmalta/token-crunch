package session

import (
	"fmt"
	"strings"
)

// DiffRatio returns a similarity score in [0,1] between two strings using
// a token-level approximation of the Myers diff ratio.
// 1.0 = identical, 0.0 = nothing in common.
func DiffRatio(a, b string) float64 {
	tokA := tokenise(a)
	tokB := tokenise(b)
	if len(tokA) == 0 && len(tokB) == 0 {
		return 1.0
	}
	common := lcsLen(tokA, tokB)
	return 2.0 * float64(common) / float64(len(tokA)+len(tokB))
}

// NearMatchThreshold is the minimum similarity to treat two outputs as near-matches.
const NearMatchThreshold = 0.85

func tokenise(s string) []string {
	return strings.Fields(s)
}

// lcsLen computes the length of the longest common subsequence of two token slices.
// Uses O(min(m,n)) space.
func lcsLen(a, b []string) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	m := len(a)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for _, tb := range b {
		for i, ta := range a {
			if ta == tb {
				curr[i+1] = prev[i] + 1
			} else if prev[i+1] > curr[i] {
				curr[i+1] = prev[i+1]
			} else {
				curr[i+1] = curr[i]
			}
		}
		prev, curr = curr, prev
		for i := range curr {
			curr[i] = 0
		}
	}
	return prev[m]
}

const diffContext = 3 // lines of context around each change

// UnifiedDiff returns a compact +/- diff between oldContent and newContent.
// Only changed lines and up to diffContext surrounding lines are emitted;
// unchanged runs are replaced with "@@ N lines unchanged @@" markers.
func UnifiedDiff(oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	ses := shortestEditScript(oldLines, newLines)

	type line struct {
		op   byte
		text string
	}
	lines := make([]line, 0, len(ses))
	oIdx, nIdx := 0, 0
	for _, op := range ses {
		switch op {
		case 'k':
			lines = append(lines, line{'k', oldLines[oIdx]})
			oIdx++
			nIdx++
		case 'd':
			lines = append(lines, line{'d', oldLines[oIdx]})
			oIdx++
		case 'i':
			lines = append(lines, line{'i', newLines[nIdx]})
			nIdx++
		}
	}

	// Expand a diffContext window around each changed line.
	n := len(lines)
	emit := make([]bool, n)
	for i, l := range lines {
		if l.op != 'k' {
			for j := max(0, i-diffContext); j <= min(n-1, i+diffContext); j++ {
				emit[j] = true
			}
		}
	}

	var sb strings.Builder
	skipped := 0
	for i, l := range lines {
		if !emit[i] {
			skipped++
			continue
		}
		if skipped > 0 {
			fmt.Fprintf(&sb, "@@ %d lines unchanged @@\n", skipped)
			skipped = 0
		}
		switch l.op {
		case 'k':
			sb.WriteString("  ")
		case 'd':
			sb.WriteString("- ")
		case 'i':
			sb.WriteString("+ ")
		}
		sb.WriteString(l.text)
		sb.WriteByte('\n')
	}
	if skipped > 0 {
		fmt.Fprintf(&sb, "@@ %d lines unchanged @@\n", skipped)
	}
	return sb.String()
}

// maxDPCells caps the (m+1)*(n+1) DP table to avoid large heap allocations on
// big tool outputs; inputs above this threshold use greedyEditScript instead.
const maxDPCells = 500_000

// shortestEditScript returns a sequence of 'k' (keep), 'd' (delete), 'i' (insert)
// operations using a simple DP edit script (not full Myers, but correct).
// When the input pair would exceed maxDPCells cells it falls back to
// greedyEditScript which uses O(m+n) space at the cost of a non-optimal (but
// valid) edit script.
func shortestEditScript(a, b []string) []byte {
	m, n := len(a), len(b)
	if int64(m+1)*int64(n+1) > maxDPCells {
		return greedyEditScript(a, b)
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				d, ins, del := dp[i-1][j-1]+1, dp[i][j-1]+1, dp[i-1][j]+1
				dp[i][j] = min3(d, ins, del)
			}
		}
	}
	ops := make([]byte, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			ops = append(ops, 'k')
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] <= dp[i-1][j]):
			ops = append(ops, 'i')
			j--
		default:
			ops = append(ops, 'd')
			i--
		}
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// greedyEditScript produces a valid (though not minimum) edit script in O(m+n)
// time and space by matching lines via position indices on both slices.
// Used when the two inputs are too large for the full DP table.
func greedyEditScript(a, b []string) []byte {
	indexB := make(map[string][]int, len(b))
	for j, line := range b {
		indexB[line] = append(indexB[line], j)
	}
	indexA := make(map[string][]int, len(a))
	for i, line := range a {
		indexA[line] = append(indexA[line], i)
	}

	ops := make([]byte, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			ops = append(ops, 'k')
			i++
			j++
			continue
		}
		// Find the next position of a[i] in b (at or after j).
		aInB := -1
		for _, p := range indexB[a[i]] {
			if p >= j {
				aInB = p
				break
			}
		}
		// Find the next position of b[j] in a (after i).
		bInA := -1
		for _, p := range indexA[b[j]] {
			if p > i {
				bInA = p
				break
			}
		}

		switch {
		case aInB == -1 && bInA == -1:
			ops = append(ops, 'd')
			i++
			ops = append(ops, 'i')
			j++
		case bInA == -1 || (aInB != -1 && (aInB-j) <= (bInA-i)):
			ops = append(ops, 'i')
			j++
		default:
			ops = append(ops, 'd')
			i++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, 'd')
	}
	for ; j < len(b); j++ {
		ops = append(ops, 'i')
	}
	return ops
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
