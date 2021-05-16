package code

import (
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"strings"
)

type ScriptLangType int

const (
	JS ScriptLangType = iota
	GO ScriptLangType = iota
)

type Code struct {
	Script 	   interface{}
	ScriptLang ScriptLangType
}

// Uses utils.ScriptErrorFormatString to return a string with both the script and the script language
func (code *Code) String() string {
	return fmt.Sprintf(utils.ScriptErrorFormatString, code.ScriptLangShebang(), fmt.Sprintf("%v", code.Script))
}

// Gets all the shebang suffixes for the given ScriptLangType
func (code *Code) ScriptLangShebang() string {
	return map[ScriptLangType]string{
		JS: "js",
		GO: "go",
	}[code.ScriptLang]
}

// Gets the ScriptLangType of the given shebang suffix
func ShebangScriptLang(shebang string) ScriptLangType {
	return map[string]ScriptLangType{
		"js": JS,
		"go": GO,
	}[shebang]
}

// Creates a new Code object from the given string source code (must include shebang) or a func(json json_map.JsonMapInt).
// If the given value is not one of these an empty Code object will be returned and ok will be false.
// If the given value is a string the following will happen:
//	 - Checking the first line of the string and seeing if it starts with the ShebangPrefix and ends with one of the supported languages.
//	 - Panics if the shebang fits the required length for a shebang but is not a supported script language.
//	 - ok is true if the script does contain a json-dom script, false otherwise.
// If the given value is a func(json json_map.JsonMapInt) then there will be no checks as a function callback will be
// run rather than a script in a virtual environment
func NewFrom(from interface{}) (code Code, ok bool) {
	ok = false
	switch from.(type) {
	case string:
		script := from.(string)
		firstLine := strings.Split(script, "\n")[0]
		firstLen := len(firstLine)

		// First check the bounds of the line so that we won't panic
		if firstLen >= utils.ShebangLen + utils.ShortestSupportedScriptTagLen && firstLen <= utils.ShebangLen + utils.LongestSupportedScriptTagLen {
			shebangPrefix, shebangScriptLang := firstLine[:utils.ShebangLen], firstLine[utils.ShebangLen:]
			if shebangPrefix != utils.ShebangPrefix {
				break
			}

			// Remove the shebang line from the script
			script = strings.Join(strings.Split(script, "\n")[1:], "\n")
			code.Script = script

			if !CheckIfSupported(shebangScriptLang) {
				// We are going to panic here as the script is unsupported
				// NOTE this will only panic when the shebang script is between the shortest and the longest supported lengths
				panic(utils.UnsupportedScriptLang.FillError(shebangScriptLang, fmt.Sprintf(utils.ScriptErrorFormatString, utils.AnonymousScriptPath, script)))
			}
			code.ScriptLang = ShebangScriptLang(shebangScriptLang)
			ok = true
		}
	case func(json json_map.JsonMapInt):
		code.Script = from.(func(json json_map.JsonMapInt))
		code.ScriptLang = GO
		ok = true
	}
	return code, ok
}