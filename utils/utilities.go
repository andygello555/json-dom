package utils

import (
	"fmt"
	"github.com/go-test/deep"
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
