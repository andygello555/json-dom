package json_map

// Stores a node within a JsonMapInt
type JsonPathNode struct {
	// The absolute JSON path to the node
	Absolute string
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
	GetAbsolutePaths(absolutePaths *[][]interface{}) (values []interface{}, errs []error)
	JsonPathSelector(jsonPath string) (out []*JsonPathNode, err error)
	Marshal() (out []byte, err error)
	Run()
	Unmarshal(jsonBytes []byte) (err error)
}
