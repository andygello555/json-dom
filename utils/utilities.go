// Contains errors, constants and other datastructures and helpful functions used throughout the codebase.
package utils

import (
	"fmt"
	"github.com/go-test/deep"
	"sort"
	"strings"
	"testing"
)

// In-Out infinite channels that don't block when written to.
//
// This is from https://medium.com/capital-one-tech/building-an-unbounded-channel-in-go-789e175cd2cd.
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
//
// Returns an empty array if step is equal to 0 or end is less than start and step is a positive number.
func Range(start, end, step int) []int {
	if step == 0 || (end < start && step > 0) {
		return []int{}
	}

	// Gets the absolute of an integer.
	abs := func(x int) int {
		if x < 0 {
			return x * -1
		}
		return x
	}

	// For checking if the iteration still holds.
	keepGoing := func(s, e int) bool {
		if step < 0 {
			return e <= s
		} else {
			return s <= e
		}
	}

	s := make([]int, 0, 1+abs(end-start)/abs(step))
	for keepGoing(start, end) {
		s = append(s, start)
		start += step
	}
	return s
}

const minInt = -int((^uint(0)) >> 1) - 1

// Returns the maximum of all the given integers.
func Max(numbers... int) (max int) {
	max = minInt
	for _, n := range numbers {
		if n > max {
			max = n
		}
	}
	return max
}

// Remove duplicates and sort an array of integers in place.
func RemoveDuplicatesAndSort(indices *[]int) {
	actualIndices := make([]int, 0)
	indexSet := make(map[int]struct{})

	for _, index := range *indices {
		// Check if the index is already in the array of actual indices. If not then we can add it
		if _, exists := indexSet[index]; !exists {
			actualIndices = append(actualIndices, index)
			indexSet[index] = struct{}{}
		}
	}

	// Sort the indices
	sort.Ints(actualIndices)
	*indices = actualIndices
}

// Adds the given value at the given indices.
//
// If there is an index which exceeds the length of the given slice plus the number of unique indices given then this
// will result in an new array that's the length of the maximum index in indices. If this happens then any "empty"
// space will be filled by default by "nil".
func AddElems(slice []interface{}, value interface{}, indices... int) []interface{} {
	RemoveDuplicatesAndSort(&indices)
	// Find the bounds of the new array which will contain the appended value. This is either:
	// 1. The maximum index: when it exceeds the limits of the new array which will be the length of the slice plus the number of indices
	// 2. The length of the slice plus the number of indices: otherwise
	var high int
	if indices[len(indices) - 1] + 1 > len(slice) + len(indices) {
		high = indices[len(indices) - 1] + 1
	} else {
		high = len(slice) + len(indices)
	}
	// Construct a new array from the specifications above
	newArr := make([]interface{}, high)
	offset := 0

	var currIdx int
	currIdx, indices = indices[0], indices[1:]

	// Iterate from 0 to high inserting a value at each index to insert into
	for i := 0; i < high; i++ {
		if currIdx == i {
			if len(indices) > 0 {
				currIdx, indices = indices[0], indices[1:]
			}
			newArr[i] = value
			offset += 1
			continue
		}
		if i - offset < len(slice) {
			newArr[i] = slice[i - offset]
		}
	}
	return newArr
}

// Removes the elements at the given indices in the given interface slice and returns a new slice.
//
// The new array will have a length which is the difference between the length of the given slice and the length of the
// given indices as a unique set.
func RemoveElems(slice []interface{}, indices... int) []interface{} {
	RemoveDuplicatesAndSort(&indices)
	out := make([]interface{}, 0)
	// Simple priority queue structure
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
//
// This takes into account orderings of slices.
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
