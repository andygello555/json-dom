package utils

import (
	"fmt"
	"sync"
)

// Add the given element at the given indices.
func ExampleAddElems() {
	arr := []interface{}{1, 2, 3}
	fmt.Println("Before:", arr)

	// All duplicate indices will be removed.
	// Here the new length of the array will be 7 as it is greater than len(arr) + len(unique indices).
	arr = AddElems(arr, 0, 0, 0, 3, 7, 1)
	fmt.Println("After:", arr)
	// Output:
	// Before: [1 2 3]
	// After: [0 0 1 0 2 3 <nil> 0]
}

// Remove the given indices from an array.
func ExampleRemoveElems() {
	arr := []interface{}{1, 2, 3, 4, 5}
	fmt.Println("Before:", arr)

	// All duplicate indices will be removed.
	arr = RemoveElems(arr, 4, 4, 2, 1, 2)
	fmt.Println("After:", arr)
	// Output:
	// Before: [1 2 3 4 5]
	// After: [1 4]
}

// Counting down from 10 in intervals of 2.
//
// The Range function is similar to the range function in Python.
func ExampleRange() {
	r := Range(10, 0, -2)
	fmt.Println(r)
	// Output:
	// [10 8 6 4 2 0]
}

// Sends values 0 through 9 into "in" channel and makes sure that they all come out in the correct order from the
// "out" channel.
//
// Example and implementation are from: https://medium.com/capital-one-tech/building-an-unbounded-channel-in-go-789e175cd2cd.
func ExampleInOut() {
	in, out := InOut()
	lastVal := -1
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		for v := range out {
			vi := v.(int)
			fmt.Println("Reading:", vi)
			if lastVal + 1 != vi {
				panic("sequence is out of order")
			}
			lastVal = vi
		}
		wg.Done()
		fmt.Println("Finished reading!")
	}()

	for i := 0; i < 10; i++ {
		fmt.Println("Writing:", i)
		in <- i
	}

	close(in)
	fmt.Println("Finished writing!")
	wg.Wait()

	if lastVal != 9 {
		panic("last value isn't 99")
	}
	// Unordered output:
	// Writing: 0
	// Writing: 1
	// Writing: 2
	// Writing: 3
	// Writing: 4
	// Writing: 5
	// Writing: 6
	// Writing: 7
	// Writing: 8
	// Writing: 9
	// Finished writing!
	// Reading: 0
	// Reading: 1
	// Reading: 2
	// Reading: 3
	// Reading: 4
	// Reading: 5
	// Reading: 6
	// Reading: 7
	// Reading: 8
	// Reading: 9
	// Finished reading!
}