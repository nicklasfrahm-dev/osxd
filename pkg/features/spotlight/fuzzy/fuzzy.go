// Package fuzzy scores how well a short query matches a piece of text.
package fuzzy

// Score returns how well pattern matches text as an in-order subsequence, or 0
// if it doesn't. Both should already be lowercase. It finds the best
// alignment: matches at the start of a word and runs of consecutive letters
// score higher, and gaps cost a little.
func Score(pattern, text string) int {
	p, t := []rune(pattern), []rune(text)
	if len(p) == 0 || len(p) > len(t) || !subsequence(p, t) {
		return 0
	}
	const (
		wordStart   = 6
		consecutive = 5
		gapCost     = 1
		maxLead     = 3
	)
	isSep := func(r rune) bool { return r == ' ' || r == '-' || r == '_' || r == '.' || r == ';' || r == '/' }
	bonus := func(j int) int { // j indexes t
		b := 1
		if j == 0 || isSep(t[j-1]) {
			b += wordStart
		}
		return b
	}

	// prev[j]: best score with the previous pattern letter matched at t[j].
	const none = -1 << 30
	prev := make([]int, len(t))
	for j := range t {
		prev[j] = none
		if t[j] == p[0] {
			lead := j
			if lead > maxLead {
				lead = maxLead
			}
			prev[j] = bonus(j) - lead*gapCost
		}
	}
	for i := 1; i < len(p); i++ {
		cur := make([]int, len(t))
		for j := range t {
			cur[j] = none
			if t[j] != p[i] {
				continue
			}
			for k := 0; k < j; k++ {
				if prev[k] == none {
					continue
				}
				s := prev[k] + bonus(j)
				if k == j-1 {
					s += consecutive
				} else {
					s -= (j - k - 1) * gapCost
				}
				if s > cur[j] {
					cur[j] = s
				}
			}
		}
		prev = cur
	}
	best := none
	for _, s := range prev {
		if s > best {
			best = s
		}
	}
	if best == none {
		return 0
	}
	if best < 1 {
		best = 1
	}
	return best
}

// subsequence is a cheap pre-check so that non-matches, the vast majority
// when searching many files, skip the full alignment.
func subsequence(p, t []rune) bool {
	i := 0
	for _, r := range t {
		if r == p[i] {
			i++
			if i == len(p) {
				return true
			}
		}
	}
	return false
}
