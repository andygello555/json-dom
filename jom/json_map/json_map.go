package json_map

// Acts as an interface for jom.JsonMap.
// Primarily created to stop cyclic imports
type JsonMapInt interface {
	Clone(clear bool) JsonMapInt
	FindScriptFields() (found bool)
	GetCurrentScopePath() string
	GetInsides() *map[string]interface{}
	Run()
}
