package code

import (
	"fmt"
	"github.com/robertkrimen/otto"
	"strings"
)

// Used to map a JS Object from Otto into a map so that it can be used
func TraverseObject(object *otto.Object) *map[string]interface{} {
	objectMap := make(map[string]interface{})

	for _, key := range object.Keys() {
		val, err := object.Get(key)
		if err != nil {
			panic(err)
		}

		var realVal interface{} = val
		if val.IsNumber() {
			float, err := val.ToFloat()
			if err != nil {
				integer, err := val.ToInteger()
				if err != nil {
					panic(err)
				}
				realVal = integer
			}
			realVal = float
		} else if val.IsString() {
			str, _ := val.ToString()
			realVal = str
		} else if val.IsBoolean() {
			boolean, _ := val.ToBoolean()
			realVal = boolean
		} else if val.IsObject() {
			obj := val.Object()
			objectMapInner := TraverseObject(obj)
			realVal = *objectMapInner
		}
		objectMap[key] = realVal
	}
	return &objectMap
}

// Callback for printing within the JS environment
func PrintlnExternal(call otto.FunctionCall) otto.Value {
	// Print the caller location
	fmt.Println("PrintlnExternal call from:", call.CallerLocation())
	var b strings.Builder
	argList := call.ArgumentList

	for i, arg := range argList {
		if arg.IsDefined() {
			if arg.IsBoolean() {
				boolean, _ := arg.ToBoolean()
				_, _ = fmt.Fprintf(&b, "%t", boolean)
			} else if arg.IsString() {
				str, _ := arg.ToString()
				_, _ = fmt.Fprintf(&b, "%s", str)
			} else if arg.IsNumber() {
				float, err := arg.ToFloat()
				if err != nil {
					integer, err := arg.ToInteger()
					if err != nil {
						panic(err)
					}
					_, _ = fmt.Fprintf(&b, "%d", integer)
				}
				_, _ = fmt.Fprintf(&b, "%g", float)
			} else if arg.IsObject() {
				obj := arg.Object()
				objMap := TraverseObject(obj)
				_, _ = fmt.Fprintf(&b, "%v", *objMap)
			} else {
				class := arg.Class()
				_, _ = fmt.Fprint(&b, class)
			}
		} else {
			_, _ = fmt.Fprint(&b, "undefined")
		}

		// Add space between args
		if i < len(argList) - 1 {
			_, _ = fmt.Fprint(&b, " ")
		}
	}
	fmt.Println("\t", b.String())
	return otto.Value{}
}

type Builtin struct {
	name string
	function func(call otto.FunctionCall)otto.Value
}

// Construct a list of all the builtin functions to register when creating the environment
var Builtins = []Builtin {
	{"printlnExternal", PrintlnExternal},
}

// Creates a new VM with some debug functions bound to JS functions
func NewVM() *otto.Otto {
	vm := otto.New()

	// Register all builtins
	// NOTE this does not register the JOM
	for _, builtin := range Builtins {
		if err := vm.Set(builtin.name, builtin.function); err != nil {
			panic(err)
		}
	}
	return vm
}
