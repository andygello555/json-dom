package utils

import (
	"fmt"
	"github.com/go-test/deep"
	"sort"
	"strings"
	"testing"
)

// In-Out infinite channels that don't block when written to
// This is from https://medium.com/capital-one-tech/building-an-unbounded-channel-in-go-789e175cd2cd
func InOut() (chan<- interface{}, <-chan interface{}) {
	in := make(chan interface{})
	out := make(chan interface{})

	go func() {
		var inQueue []interface{}

		// Temp function which returns the out channel to write to
		// This is done to avoid writing nils to the out channel
		outCh := func() chan interface{} {
			if len(inQueue) == 0 {
				return nil
			}
			return out
		}

		// Returns the head of the input queue if the queue is not empty otherwise it returns nil
		curVal := func() interface {} {
			if len(inQueue) == 0 {
				return nil
			}
			return inQueue[0]
		}

		for len(inQueue) > 0 || in != nil {
			select {
			// Read from input channel if we can
			case v, ok := <-in:
				if !ok {
					// If input channel is empty then we set input to a nil channel so we don't read anything more
					in = nil
				} else {
					// We append the input to the queue to be written to out
					inQueue = append(inQueue, v)
				}
			// If there is a value in the queue to write to out then write
			case outCh() <- curVal():
				// We pop off the head of the queue
				inQueue = inQueue[1:]
			}
		}
		close(out)
	}()
	return in, out
}

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
		errB.WriteString(fmt.Sprintf("Difference between actual and expected for %s (Left = Actual, Right = Expected)\n", forString))
		for _, d := range diff {
			errB.WriteString(fmt.Sprintf("\t%s\n", d))
		}
		t.Error(errB.String())
	}
}
