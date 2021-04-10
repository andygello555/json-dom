package utils

import "time"

const (
	ShebangPrefix                 = "#//!"
	ShebangLen                    = len(ShebangPrefix)
	ShortestSupportedScriptTagLen = 2
	LongestSupportedScriptTagLen  = 2
	JOMVariableName               = "json"
	KeyValuePairDelim             = ':'
	HaltingDelayUnits             = time.Second
	ScriptErrorFormatString       = "script <%s>:\n```\n%s\n```"
	AnonymousScriptPath           = "<anonymous>"
	CurrentNodeLiteralVarName     = "__currentNodeLiteral__"
	CurrentNodeValueVarName       = "__currentNode__"
)

// These are global variables that can be changed
var (
	HaltingDelay = 4
)

// Gets a map of all the supported script shebang suffixes
// An accepted shebang line could look like: #//!js
func GetSupportedScriptTags() map[string]bool {
	return map[string]bool {
		"js": true,
	}
}
