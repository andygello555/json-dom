package json_map

import "fmt"

// Parsing a JSON path string into an AbsolutePaths type.
func ExampleParseJsonPath() {
	path, _ := ParseJsonPath("$..property[0, 1, 2].name")
	fmt.Println(path)
	// Output:
	// [[|RecursiveLookup: property| |IndexKey: 0| |StringKey: name|] [|RecursiveLookup: property| |IndexKey: 1| |StringKey: name|] [|RecursiveLookup: property| |IndexKey: 2| |StringKey: name|]]
}

// Constructing an AbsolutePaths value using AddToAll.
func ExampleAbsolutePaths() {
	absolutePaths := make(AbsolutePaths, 0)

	// We add a RecursiveLookup key, this will initialise the root of the AbsolutePaths.
	absolutePaths.AddToAll(nil, false,
		AbsolutePathKey{
			KeyType: RecursiveLookup,
			Value:   "property",
		},
	)

	// Then we add the 0, 1 and 2 indices.
	// Note: We have to add them using separate calls. This is because we want to create a new path for each index added.
	absolutePaths.AddToAll(nil, false,
		AbsolutePathKey{
			KeyType: IndexKey,
			Value:   0,
		},
		AbsolutePathKey{
			KeyType: IndexKey,
			Value:   1,
		},
		AbsolutePathKey{
			KeyType: IndexKey,
			Value:   2,
		},
	)

	// Then we add the string key.
	absolutePaths.AddToAll(nil, false,
		AbsolutePathKey{
			KeyType: StringKey,
			Value:   "name",
		},
	)

	fmt.Println(absolutePaths)
	// Output:
	// [[|RecursiveLookup: property| |IndexKey: 0| |StringKey: name|] [|RecursiveLookup: property| |IndexKey: 1| |StringKey: name|] [|RecursiveLookup: property| |IndexKey: 2| |StringKey: name|]]
}