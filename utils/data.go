package utils

import (
	"strings"
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
