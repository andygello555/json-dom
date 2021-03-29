package js

import (
	"encoding/json"
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"github.com/robertkrimen/otto"
	"log"
	"strings"
	"time"
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

// Struct representing a builtin function that can be called from within the JS environment
type BuiltinFunc struct {
	name     string
	function func(call otto.FunctionCall)otto.Value
}

// Struct representing a variable that can be accessed from within the JS environment
type BuiltinVar struct {
	name   string
	getter func(...interface{}) interface{}
}

// Construct a list of all the builtin functions to register when creating the environment
var builtinFuncs = []BuiltinFunc{
	{"printlnExternal", PrintlnExternal},
}

var builtinVars = []BuiltinVar{
	{"__scopePath__", func(i ...interface{}) interface{} {
		if jsonMap, ok := i[0].(json_map.JsonMapInt); ok {
			return jsonMap.GetCurrentScopePath()
		}
		// Panic if we can't assert the given arg into a JsonMapInt
		panic(utils.BuiltinGetterError.FillError("__scopePath__", fmt.Sprintf("Could not assert %v into JsonMapInt", i[0])))
	}},
}

// Create the JOM within a Javascript VM, assign all necessary functions and retrieve the variable from within the VM.
// This will create a JOM for the scope of the given json map.
// Returns an otto.Value which can be plugged into the VM which will run the scripts. If an error occurs at any point
// then an otto.NullValue and the error are returned.
func createJom(jsonMap json_map.JsonMapInt) (run otto.Value, err error) {
	// Convert the map to json
	jsonDataBytes, err := json.Marshal(jsonMap.GetInsides())
	if err != nil {
		return otto.NullValue(), err
	}
	jsonData := string(jsonDataBytes)

	// Create a VM, parse the json string and get the value out of the VM
	vm := otto.New()
	if err := vm.Set("jsonString", jsonData); err != nil {
		return otto.NullValue(), err
	}
	run, err = vm.Run("JSON.parse(jsonString)")
	if err != nil {
		return otto.NullValue(), err
	}

	// TODO At some point introduce some helpful functions and helpers to the JOM

	return run, nil
}

// Given a JS environment, retrieve the JOM and generate the json_map.JsonMapInt for the object
// Returns the json_map.JsonMapInt of the converted JOM and any errors (if there are any)
func deJomIfy(jsonMap json_map.JsonMapInt, env *otto.Otto) (data json_map.JsonMapInt, err error) {
	// TODO this will need to change when the CreateJom function changes. Such as when new helper functions are introduced
	data = jsonMap.Clone(true)

	// Stringify and return the JOM (as a string)
	run, err := env.Run("JSON.stringify(json)")
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON string to convert it into a map
	if err := json.Unmarshal([]byte(run.String()), data.GetInsides()); err != nil {
		return nil, err
	}
	return data, nil
}

// Run the given script, with the given json_map.JsonMapInt and return the new json_map.JsonMapInt for the scope
// The order of which things are executed
// 1. The JOM is created
// 2. The VM is created
// 3. The builtins and the JOM is passed into the environment
// 4. Interrupt for the halting problem is setup
// 5. The script is run
// 6. The environment is De-JOM-ified
// 7. The new json_map.JsonMapInt is returned
func RunScript(script string, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error) {
	// Create the JOM createJom
	jomValue, err := createJom(jsonMap)
	if err != nil {
		return nil, err
	}

	// Create the VM and register all builtins
	vm := otto.New()
	// Register all builtins
	for _, builtin := range builtinFuncs {
		if err := vm.Set(builtin.name, builtin.function); err != nil {
			panic(err)
		}
	}
	for _, builtin := range builtinVars {
		switch builtin.name {
		case "__scopePath__":
			if err := vm.Set(builtin.name, builtin.getter(jsonMap)); err != nil {
				panic(err)
			}
		default:
			log.Fatal("Could not assign builtin variable: " + builtin.name)
		}
	}

	// Pass the JOM into the environment
	if err = vm.Set(utils.JOMVariableName, jomValue); err != nil {
		return nil, err
	}
	// Remove the shebang line from the script
	script = strings.Join(strings.Split(script, "\n")[1:], "\n")

	// To stop infinite loops start a timer which will panic once the timer stops and be caught in a deferred func
	start := time.Now()
	// This will catch any panics thrown by running the script/the timer
	defer func() {
		duration := time.Since(start)
		if caught := recover(); caught != nil {
			// If the caught error is the HaltingProblem var then package it up using FillError and set the outer error
			if caught == utils.HaltingProblem {
				err = utils.HaltingProblem.FillError(
					duration.String(),
					fmt.Sprintf(utils.ScriptErrorFormatString, script),
				)
				return
			}
			// Another error that we should panic for
			panic(caught)
		}
	}()

	vm.Interrupt = make(chan func(), 1)

	// Start the timer
	go func() {
		time.Sleep(utils.HaltingDelay * utils.HaltingDelayUnits)
		vm.Interrupt <- func() {
			panic(utils.HaltingProblem)
		}
	}()
	// Run the script
	_, err = vm.Run(script)
	if err != nil {
		return nil, err
	}

	// De-JOM-ify the environment and return the json_map.JsonMapInt
	data, err = deJomIfy(jsonMap, vm)
	if err != nil {
		return nil, err
	}
	return data, nil
}
