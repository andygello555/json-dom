// Contains all the types/functions to do with running scripts/callbacks as well as a map of all the SupportedLang(s).
//
// How it works
//
// When calling json_map.JsonMapInt.FindScriptFields, each script/callback will be replaced by a value of Code type in
// the jom.Traversal field within jom.JsonMap. The code.Run function can be used to execute a code.Code object of any
// supported type.
package code

import (
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/globals"
	"strings"
)

type ScriptLangType int

// Script language types which are used for printing errors/determining script language type.
//
// Note: This does not mean that all constants are supported by json-dom.
const (
	JS ScriptLangType = iota
	GO ScriptLangType = iota
)

// Wrapper for any "runnable" script/callback.
type Code struct {
	// The script/callback which can be run in either a VM of the language's type/in Go if it is a callback.
	Script 	   interface{}
	// The script's language which determines what it will be run inside.
	ScriptLang ScriptLangType
}

// Uses globals.ScriptErrorFormatString to return a string with both the script and the script language.
func (code *Code) String() string {
	return fmt.Sprintf(globals.ScriptErrorFormatString, code.ScriptLangShebang(), fmt.Sprintf("%v", code.Script))
}

// Gets all the shebang suffixes for the given ScriptLangType.
func (code *Code) ScriptLangShebang() string {
	return map[ScriptLangType]string{
		JS: "js",
		GO: "go",
	}[code.ScriptLang]
}

// Gets the ScriptLangType of the given shebang suffix.
func ShebangScriptLang(shebang string) ScriptLangType {
	return map[string]ScriptLangType{
		"js": JS,
		"go": GO,
	}[shebang]
}

// Creates a new Code object from the given string source code (must include shebang) or a func(json json_map.JsonMapInt).
// If the given value is not one of these an empty Code object will be returned and ok will be false.
// If the given value is a string the following will happen:
//
// • Checking the first line of the string and seeing if it starts with the ShebangPrefix and ends with one of the supported languages.
//
// • Panics if the shebang fits the required length for a shebang but is not a supported script language.
//
// • ok is true if the script does contain a json-dom script, false otherwise.
//
// If the given value is a func(json json_map.JsonMapInt) then there will be no checks as a function callback will be
// run rather than a script in a virtual environment.
func NewFrom(from interface{}) (code Code, ok bool) {
	ok = false
	switch from.(type) {
	case string:
		script := from.(string)
		firstLine := strings.Split(script, "\n")[0]
		firstLen := len(firstLine)

		// First check the bounds of the line so that we won't panic
		if firstLen >= globals.ShebangLen + globals.ShortestSupportedScriptTagLen && firstLen <= globals.ShebangLen + globals.LongestSupportedScriptTagLen {
			shebangPrefix, shebangScriptLang := firstLine[:globals.ShebangLen], firstLine[globals.ShebangLen:]
			if shebangPrefix != globals.ShebangPrefix {
				break
			}

			// Remove the shebang line from the script
			script = strings.Join(strings.Split(script, "\n")[1:], "\n")
			code.Script = script

			if !CheckIfSupported(shebangScriptLang) {
				// We are going to panic here as the script is unsupported
				// NOTE this will only panic when the shebang script is between the shortest and the longest supported lengths
				panic(globals.UnsupportedScriptLang.FillError(shebangScriptLang, fmt.Sprintf(globals.ScriptErrorFormatString, globals.AnonymousScriptPath, script)))
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