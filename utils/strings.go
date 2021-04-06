package utils

import (
	"strings"
	"unicode"
)

// StringHeap priority queue used when evaluating script
type StringHeap []string

func (spq StringHeap) Len() int { return len(spq) }

func (spq StringHeap) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return strings.Compare(spq[i], spq[j]) <= 0
}

func (spq StringHeap) Swap(i, j int) { spq[i], spq[j] = spq[j], spq[i] }

func (spq *StringHeap) Push(x interface{}) { *spq = append(*spq, x.(string)) }

func (spq *StringHeap) Pop() interface{} {
	old := *spq
	n := len(old)
	str := old[n-1]
	*spq = old[0 : n-1]
	return str
}

// Given a string, will strip all whitespace from it and return a new string without any whitespace
func StripWhitespace(str string) string {
	var b strings.Builder
	b.Grow(len(str))
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
