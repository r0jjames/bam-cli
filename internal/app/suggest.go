package app

import "strings"

// Closest returns up to n candidates similar to input: substring matches
// first, then names within a small edit distance.
func Closest(input string, candidates []string, n int) []string {
	lower := strings.ToLower(input)
	var out []string
	seen := map[string]bool{}
	for _, c := range candidates {
		if len(out) == n {
			return out
		}
		if strings.Contains(strings.ToLower(c), lower) {
			out = append(out, c)
			seen[c] = true
		}
	}
	limit := len(input) / 3
	if limit < 2 {
		limit = 2
	}
	for _, c := range candidates {
		if len(out) == n {
			break
		}
		if !seen[c] && levenshtein(lower, strings.ToLower(c)) <= limit {
			out = append(out, c)
		}
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
