package utils

import (
	"fmt"
	"github.com/go-test/deep"
	"sort"
	"strings"
	"testing"
)

// Generates an integer array with indices from start to end with the given step value.
// Returns an empty array if step is less than or equal to 0 or end is less than start.
func Range(start, end, step int) []int {
	if step <= 0 || end < start {
		return []int{}
	}
	s := make([]int, 0, 1+(end-start)/step)
	for start <= end {
		s = append(s, start)
		start += step
	}
	return s
}

// Removes the elements at the given indices in the given interface slice and returns a new slice.
func RemoveElems(slice []interface{}, indices... int) []interface{} {
	out := make([]interface{}, 0)
	// Simple priority queue structure
	sort.Ints(indices)
	var currIdx int
	currIdx, indices = indices[0], indices[1:]

	for i, elem := range slice {
		if i == currIdx {
			if len(indices) > 0 {
				currIdx, indices = indices[0], indices[1:]
			}
			continue
		}
		out = append(out, elem)
	}
	return out
}

// Clones a map deeply using recursion.
func CopyMap(m map[string]interface{}) map[string]interface{} {
	cp := make(map[string]interface{})
	for k, v := range m {
		vm, ok := v.(map[string]interface{})
		if ok {
			cp[k] = CopyMap(vm)
		} else {
			cp[k] = v
		}
	}

	return cp
}

// Used in tests to check equality between two interface{}s.
// NOTE this takes into account orderings.
func JsonMapEqualTest(t *testing.T, actual, expected interface{}, forString string) {
	if diff := deep.Equal(actual, expected); diff != nil {
		var errB strings.Builder
		errB.WriteString(fmt.Sprintf("Difference between actual and expected for %s (Left = Expected, Right = Actual)\n", forString))
		for _, d := range diff {
			errB.WriteString(fmt.Sprintf("\t%s\n", d))
		}
		t.Error(errB.String())
	}
}
