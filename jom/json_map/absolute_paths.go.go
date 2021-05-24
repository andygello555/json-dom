package json_map

import (
	"fmt"
)

// Represents a type of a key within an AbsolutePath.
type AbsolutePathKeyType int

// All the key types that can be added to an absolute path.
const (
	// Represents descent down a key in a map.
	StringKey AbsolutePathKeyType = iota
	// Represents descent down an index in an array.
	IndexKey AbsolutePathKeyType = iota
	// Represents descent down all key-value pairs in a map or elements in an array.
	Wildcard AbsolutePathKeyType = iota
	// Represents a filter expression on a map or an array.
	Filter AbsolutePathKeyType = iota
	// Represents a lexicographically first descent.
	First AbsolutePathKeyType = iota
	// Represents list slice notation (range of elements within a list).
	Slice AbsolutePathKeyType = iota
	// Represents a start or an end within a slice.
	//
	// Note: The following should only be used within a Slice AbsolutePathKey's Value.
	StartEnd AbsolutePathKeyType = iota
	// Represents a recursive lookup for a given property.
	RecursiveLookup AbsolutePathKeyType = iota
)

// Map of absolute key type values to their corresponding names.
//
// Used in String method of AbsolutePathKey.
var AbsolutePathKeyTypeNames = map[AbsolutePathKeyType]string{
	StringKey:       "StringKey",
	IndexKey:        "IndexKey",
	Wildcard:        "Wildcard",
	Filter:          "Filter",
	First:           "First",
	Slice:           "Slice",
	StartEnd:        "StartEnd",
	RecursiveLookup: "RecursiveLookup",
}

// An absolute path key with a KeyType and a Value.
type AbsolutePathKey struct {
	// An AbsolutePathKeyType which represents what type of descent is occurring.
	KeyType AbsolutePathKeyType
	// An associated value.
	//
	// Non-nil for StringKey, IndexKey, Filter, Slice and RecursiveLookup. Types can be inferred from the KeyType.
	Value interface{}
}

func (apk AbsolutePathKey) String() string {
	return fmt.Sprintf("|%s: %v|", AbsolutePathKeyTypeNames[apk.KeyType], apk.Value)
}

// Type representing a list of absolute paths.
//
// Used as an intermediary for calculating JSON paths.
type AbsolutePaths [][]AbsolutePathKey

// Adds the given value to the end of each absolute path in the AbsolutePaths list.
//
// If check is true then the given jsonMap will be checked if all paths can be reached within the context of the map.
// jsonMap can be nil if check is false.
func (p *AbsolutePaths) AddToAll(jsonMap JsonMapInt, check bool, pathValues ...AbsolutePathKey) (errs []error) {
	if len(*p) > 0 {
		// If our AbsolutePaths array contains the following paths...
		// {property1, 0}
		// {property1, 1}
		// And we want to add the new paths: [0, 1, 2]
		// Then we would end up with...
		// {property1, 0, 0}
		// {property1, 0, 1}
		// {property1, 0, 2}
		// {property1, 1, 0}
		// {property1, 1, 1}
		// {property1, 1, 2}
		if len(pathValues) > 1 {
			newAbsolutePaths := make(AbsolutePaths, 0)
			for _, absolutePath := range *p {
				for _, val := range pathValues {
					// Create a clone of the absolute path slice
					newAbsolutePath := make([]AbsolutePathKey, len(absolutePath))
					copy(newAbsolutePath, absolutePath)
					// Append the value onto the new absolute path slice
					newAbsolutePath = append(newAbsolutePath, val)
					// Then append the new path into the new array
					newAbsolutePaths = append(newAbsolutePaths, newAbsolutePath)
				}
			}
			// Set the current referer to equal the new paths
			*p = newAbsolutePaths
		} else if len(pathValues) == 1 {
			// Do this to save memory as no new absolute paths are created therefore no paths need to be cloned
			for i, absolutePath := range *p {
				(*p)[i] = append(absolutePath, pathValues[0])
			}
		}
	} else {
		// Create the start of the absolute paths
		for _, val := range pathValues {
			*p = append(*p, []AbsolutePathKey{val})
		}
	}

	// Check if there is a way to all of those paths (only if any new paths were added and check is true)
	if len(pathValues) > 0 && check {
		_, errs = jsonMap.GetAbsolutePaths(p)
		if errs != nil {
			return errs
		}
	}
	return nil
}

// Stores a node within a JsonMapInt.
type JsonPathNode struct {
	// The absolute path to the node.
	Absolute []AbsolutePathKey
	// The value of the node.
	Value interface{}
}
