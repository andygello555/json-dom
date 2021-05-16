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
	runCode     func(code Code, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)
}

// All the scripting languages currently supported.
var supportedLangs = make(map[string]*SupportedLang)

// Checks if the given
func CheckIfSupported(scriptLang string) bool {
	_, ok := supportedLangs[scriptLang]
	return ok
}

// Registers a new SupportedLang to the supportedLangs map.
// Every supported language package should call this within their init().
func RegisterLang(shebangName string, runCode func(code Code, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error)) bool {
	supportedLangs[shebangName] = &SupportedLang{
		shebangName: shebangName,
		runCode:     runCode,
	}
	return true
}

// Run the given Code in the given Code environment.
// Returns a json_map.JsonMapInt containing the updated scope, and a non-nil error if an error has occurred, otherwise
// err will be nil.
func Run(code Code, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error) {
	if supportedLang, ok := supportedLangs[code.ScriptLangShebang()]; ok {
		return supportedLang.runCode(code, jsonMap)
	}
	//fmt.Println(supportedLangs)
	return nil, utils.UnsupportedScriptLang.FillError(code.ScriptLangShebang(), fmt.Sprintf(utils.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), "func(json json_map.JsonMapInt)"))
}
