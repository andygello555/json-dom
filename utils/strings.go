package utils

import (
	"sort"
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

// Will replace all characters at the given indices with the new string. Returns a new string.
// Indices are all the character indices with which to replace with the new string
func ReplaceCharIndex(old string, new string, indices... int) string {
	if len(indices) > 0 {
		// Lets sort the indices so that we can pop them in ascending order
		sort.Ints(indices)
		// Pop the first element
		var currIdx int
		currIdx, indices = indices[0], indices[1:]

		var b strings.Builder
		for idx, val := range old {
			if currIdx == idx {
				// If we have reached an index to replace then write the new string and pop the new idx
				b.WriteString(new)
				if len(indices) > 0 {
					currIdx, indices = indices[0], indices[1:]
				}
			} else {
				// Otherwise write the current character
				b.WriteString(string(val))
			}
			idx++
		}
		return b.String()
	}
	// If there is nothing to replace then return the old string
	return old
}

// Similar to ReplaceCharIndex but takes multiple index ranges in the form of [start, end]. The length of new strings
// must be less than or equal to the length of the indices slice. The length of indices must also be greater than 0. If
// any of these conditions are not met the old string shall be returned.
func ReplaceCharIndexRange(old string, indices [][]int, new... string) string {
	if len(indices) > 0 && len(new) <= len(indices) {
		// Sort the indices by ascending end values
		sort.SliceStable(indices, func(i, j int) bool {
			return indices[i][1] < indices[j][1]
		})
		// Pop the first element
		var currRange []int
		currRange, indices = indices[0], indices[1:]
		idxCount := 0

		var b strings.Builder
		idx := 0
		for idx < len(old) {
			if idx == currRange[0] {
				// Write the new string if we have just stumbled upon the start of the current range
				b.WriteString(new[idxCount % len(new)])
				idxCount++
				idx += currRange[1] - currRange[0]
				// Pop the new range if we still can
				if len(indices) > 0 {
					currRange, indices = indices[0], indices[1:]
				}
				continue
			}
			b.WriteString(string(old[idx]))
			idx++
		}
		return b.String()
	}
	// If there is nothing to replace then return the old string
	return old
}
