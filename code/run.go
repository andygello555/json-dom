package code

import (
	"fmt"
	"github.com/andygello555/json-dom/code/js"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
)

type SupportedLang struct {
	// The suffix of the shebang
	shebangName string
	// The function that will run the given script in the given scope
	runScript   func(script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)
}

func New(shebangName string, runScript func(script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)) *SupportedLang {
	return &SupportedLang{
		shebangName: shebangName,
		runScript:   runScript,
	}
}

// All the scripting languages currently supported.
var SupportedLangs = map[string]*SupportedLang{
	"js": New("js", js.RunScript),
}

// Run the given script in the given script environment.
// Returns a json_map.JsonMapInt containing the updated scope, and a non-nil error if an error has occurred, otherwise
// err will be nil.
func Run(scriptLang string, script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error) {
	if supportedLang, ok := SupportedLangs[scriptLang]; ok {
		return supportedLang.runScript(script, jsonMap)
	}
	return nil, utils.UnsupportedScriptLang.FillError(scriptLang, fmt.Sprintf(utils.ScriptErrorFormatString, script))
}
