package utils

const (
	ShebangPrefix                 = "#//!"
	ShebangLen					  = 4
	ShortestSupportedScriptTagLen = 2
	LongestSupportedScriptTagLen  = 2
)

// Gets a map of all the supported script shebang suffixes
// An accepted shebang line could look like: #//!js
func GetSupportedScriptTags() map[string]bool {
	return map[string]bool {
		"js": true,
	}
}
