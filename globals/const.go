// Constants, errors and globals variables used across the module.
package globals

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

// These are global variables that can be changed.
var (
	// The delay time in HaltingDelayUnits after which a running script will panic to stop execution of infinitely executing scripts.
	HaltingDelay = 4
)

// Returns a map of the descriptions for the subcommands that are used in the CLI application.
func CliSubcommandDescriptions() map[string]string {
	return map[string]string{
		"eval": "Evaluates a given hjson input/file(s)",
		"markup": "Mark up the given hjson input/file(s) with the given JSONPath-script pairs",
	}
}
