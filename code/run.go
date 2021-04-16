package code

import (
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
)

type SupportedLang struct {
	// The suffix of the shebang
	shebangName string
	// The function that will run the given script in the given scope
	runScript   func(script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)
}

// All the scripting languages currently supported.
var SupportedLangs = make(map[string]*SupportedLang)

// Registers a new SupportedLang to the SupportedLangs map.
// Every supported language package should call this within their init().
func RegisterLang(shebangName string, runScript func(script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)) bool {
	SupportedLangs[shebangName] = &SupportedLang{
		shebangName: shebangName,
		runScript:   runScript,
	}
	return true
}

// Run the given script in the given script environment.
// Returns a json_map.JsonMapInt containing the updated scope, and a non-nil error if an error has occurred, otherwise
// err will be nil.
func Run(scriptLang string, script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error) {
	if supportedLang, ok := SupportedLangs[scriptLang]; ok {
		return supportedLang.runScript(script, jsonMap)
	}
	fmt.Println(SupportedLangs)
	return nil, utils.UnsupportedScriptLang.FillError(scriptLang, fmt.Sprintf(utils.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), script))
}
