package json_map

import "fmt"

// Represents a type of a key within an AbsolutePath
type AbsolutePathKeyType int

// All the key types that can be added to an absolute path
const (
	StringKey       AbsolutePathKeyType = iota
	IndexKey        AbsolutePathKeyType = iota
	Wildcard        AbsolutePathKeyType = iota
	Filter          AbsolutePathKeyType = iota
	First           AbsolutePathKeyType = iota
	Slice           AbsolutePathKeyType = iota
	// NOTE the following should only be used within a Slice AbsolutePathKey's Value
	StartEnd        AbsolutePathKeyType = iota
	RecursiveLookup AbsolutePathKeyType = iota
)

// Map of absolute key type values to their corresponding names
// Used in String method of AbsolutePathKey
var AbsolutePathKeyTypeNames = map[AbsolutePathKeyType]string {
	StringKey:       "StringKey",
	IndexKey:        "IndexKey",
	Wildcard:        "Wildcard",
	Filter:          "Filter",
	First:           "First",
	Slice:           "Slice",
	StartEnd:        "StartEnd",
	RecursiveLookup: "RecursiveLookup",
}

// An absolute path key with a KeyType and a Value
type AbsolutePathKey struct {
	KeyType AbsolutePathKeyType
	Value   interface{}
}

func (apk AbsolutePathKey) String() string {
	return fmt.Sprintf("|%s: %v|", AbsolutePathKeyTypeNames[apk.KeyType], apk.Value)
}

// Type representing a list of absolute paths
// Used as an intermediary for calculating JSON paths
type AbsolutePaths [][]AbsolutePathKey

// Adds the given value to the end of each absolute path in the AbsolutePaths list.
// If check is true then the given jsonMap will be checked if all paths can be reached within the context of the map.
// jsonMap can be nil if check is false.
func (p *AbsolutePaths) AddToAll(jsonMap JsonMapInt, check bool, pathValues... AbsolutePathKey) (errs []error) {
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

// Stores a node within a JsonMapInt
type JsonPathNode struct {
	// The absolute JSON path to the node
	Absolute []AbsolutePathKey
	// The value of the node
	Value    interface{}
}

// Acts as an interface for jom.JsonMap.
// Primarily created to stop cyclic imports
type JsonMapInt interface {
	Clone(clear bool) JsonMapInt
	FindScriptFields() (found bool)
	GetCurrentScopePath() string
	GetInsides() *map[string]interface{}
	GetAbsolutePaths(absolutePaths *AbsolutePaths) (values []*JsonPathNode, errs []error)
	JsonPathSelector(jsonPath string) (out []*JsonPathNode, err error)
	JsonPathSetter(jsonPath string, value interface{}) (err error)
	Markup(jsonPath string, shebangName string, script string) (err error)
	Marshal() (out []byte, err error)
	Run()
	SetAbsolutePaths(absolutePaths *AbsolutePaths, value interface{}) (err error)
	Unmarshal(jsonBytes []byte) (err error)
}
