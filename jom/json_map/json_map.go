// Contains the JsonMapInt interface which should be used in signatures as well as constants and types relating to
// AbsolutePaths and JSON path parsing to AbsolutePaths.
package json_map

// Acts as an interface for jom.JsonMap.
//
// Primarily created to stop cyclic imports.
type JsonMapInt interface {
	// Return a clone of the JsonMap. If clear is given then New will be called but "Array" field will be inherited.
	Clone(clear bool) JsonMapInt
	// Finds all the script and non-script fields within a JsonMap.
	FindScriptFields() (found bool)
	// Returns the current scopes JSON Path to itself.
	GetCurrentScopePath() string
	// Getter for insides.
	GetInsides() *map[string]interface{}
	// Given the list of absolute paths for a JsonMap, will return the list of values that said paths lead to.
	GetAbsolutePaths(absolutePaths *AbsolutePaths) (values []*JsonPathNode, errs []error)
	// Given a valid JSON path will return the list of pointers to json_map.JsonPathNode(s) that satisfies the JSON path.
	JsonPathSelector(jsonPath string) (out []*JsonPathNode, err error)
	// Given a valid JSON path: will set the values pointed to by the JSON path to be the value given.
	JsonPathSetter(jsonPath string, value interface{}) (err error)
	// Adds the given script of the given shebangName (must be a supported language) at the path pointed to by the given jsonPath.
	MarkupCode(jsonPath string, shebangName string, script string) (err error)
	// Marshal a JsonMap back into JSON.
	Marshal() (out []byte, err error)
	// A wrapper for MustSet(jsonPath, nil).
	MustDelete(jsonPath string)
	// Like JsonPathSelector, only it panics when an error occurs and returns an []interface{} instead of []json_map.JsonPathNode.
	MustGet(jsonPath string) (out []interface{})
	// Pops from an []interface{} indicated by the given JSON path at the given indices and panics if any errors occur.
	MustPop(jsonPath string, indices... int) (popped []interface{})
	// Pushes to an []interface{} indicated by the given JSON path at the given indices and panics if any errors occur.
	MustPush(jsonPath string, value interface{}, indices... int)
	// Like JsonPathSetter, only it panics when an error occurs.
	MustSet(jsonPath string, value interface{})
	// Given a JsonMap this will traverse it and execute all scripts. Will update the given JsonMap in place.
	Run()
	// Given the list of absolute paths for a JsonMap: will set the values pointed to by the given JSON path to be the given value.
	SetAbsolutePaths(absolutePaths *AbsolutePaths, value interface{}) (err error)
	// Strips any script key-value pairs found within the JsonMap and updates it in place.
	Strip()
	// Marshals the JsonMap into hjson and returns the stringified byte array.
	String() string
	// Unmarshal a hjson byte string and package it as a JsonMap.
	Unmarshal(jsonBytes []byte) (err error)
}
