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
	ModifiedTrailValueVarName     = "__modifiedTrail__"
)

// These are global variables that can be changed
var (
	HaltingDelay = 4
)

func CliSubcommandDescriptions() map[string]string {
	return map[string]string{
		"eval": "Evaluates a given hjson input/file(s)",
		"markup": "Mark up the given hjson input/file(s) with the given JSONPath-script pairs",
	}
}
