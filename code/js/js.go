package js

import (
	"encoding/json"
	"fmt"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"github.com/robertkrimen/otto"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
)

// Register this language in the code package
func init() {
	code.RegisterLang("js", RunScript)
}

// These can be set when testing to check output
var ExternalConsoleLogStdout io.Writer = os.Stdout
var ExternalConsoleLogStderr io.Writer = os.Stderr

// Used to map a JS Object from Otto into a map so that it can be used
func traverseObject(object *otto.Object) *map[string]interface{} {
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
			objectMapInner := traverseObject(obj)
			realVal = *objectMapInner
		}
		objectMap[key] = realVal
	}
	return &objectMap
}

func toGo(value otto.Value) (out interface{}) {
	var err error
	if value.IsDefined() {
		switch true {
		case value.IsBoolean():
			out, _ = value.ToBoolean()
		case value.IsString():
			out, _ = value.ToString()
		case value.IsNumber():
			out, err = value.ToFloat()
			if err != nil {
				out, err = value.ToInteger()
				if err != nil {
					panic(err)
				}
			}
		case value.IsObject():
			obj := value.Object()
			out = traverseObject(obj)
		case value.IsNull():
			out = nil
		case value.IsFunction():
			fallthrough
		default:
			out = value.Class()
		}
	} else if value.IsUndefined() {
		out = nil
	}
	return out
}

// Composes a string to print from the given otto.FunctionCall
func composePrint(call otto.FunctionCall) *strings.Builder {
	// Print the caller location
	var out strings.Builder
	var callLocation string

	if get, err := call.Otto.Run("json.scopePath"); err != nil {
		callLocation = "json.scopePath not found"
	} else {
		// Check if json.__scopePath__ is not a string (it has been overridden by user)
		if !get.IsString() {
			panic(utils.OverriddenBuiltin.FillError("json.scopePath"))
		}
		callLocation = fmt.Sprintf("<%s>", get.String())
	}
	_, _ = fmt.Fprintln(&out, "call from:", strings.Replace(call.CallerLocation(), utils.AnonymousScriptPath, callLocation, -1))
	var b strings.Builder
	argList := call.ArgumentList

	for i, arg := range argList {
		val := toGo(arg)
		if val == nil {
			_, _ = fmt.Fprint(&b, "undefined")
		} else {
			_, _ = fmt.Fprintf(&b, "%v", val)
		}

		// Add space between args
		if i < len(argList) - 1 {
			_, _ = fmt.Fprint(&b, " ")
		}
	}

	// Tabulate all lines that are being output and write them to out
	for _, line := range strings.Split(b.String(), "\n") {
		_, _ = fmt.Fprintf(&out, "\t%s\n", line)
	}
	return &out
}

// Given a JSON path will return a "NodeSet" object which contains the absolute paths to all values denoted by the JSON
// path as well as getter and setter functions.
//
// 1. Function will stringify json.trail within the VM and unmarshall to a new JsonMap.
// 2. The JSON path will be parsed into json_map.AbsolutePaths and GetAbsolutePaths will be called
// 3. json_map.AbsolutePaths will be converted into JS values
// 4. The returned object will be constructed (_absolutePaths, getValues, setValues)
func jsonPathSelector(call otto.FunctionCall) otto.Value {
	var err error
	vm := call.Otto

	throw := func(message string) {
		panic(vm.MakeCustomError("JSONPathError", message))
	}

	// Check number of arguments and argument types
	if len(call.ArgumentList) > 1 || !call.Argument(0).IsString() {
		throw("jsonPathSelector takes a single string argument")
	}
	jsonPath, _ := call.Argument(0).ToString()

	// We set up a function to retrieve the JsonMap so we can retrieve the most up to date version of json.trail
	getJsonMap := func(vm *otto.Otto) *jom.JsonMap {
		// Stringify the json.trail object
		var trailStringValue otto.Value
		trailStringValue, err = vm.Run(fmt.Sprintf("JSON.stringify(%s.trail)", utils.JOMVariableName))
		if err != nil || trailStringValue.IsUndefined() || trailStringValue.IsNull() || !trailStringValue.IsString() {
			if err != nil {
				throw(err.Error())
			}
			throw(fmt.Sprintf("\"%s.trail\" is not JSON stringifiable. It is \"%v\".", utils.JOMVariableName, trailStringValue))
		}
		// Marshall the JSON string into a JsonMap
		trailString, _ := trailStringValue.ToString()
		jMap := jom.New()
		err = jMap.Unmarshal([]byte(trailString))
		if err != nil {
			throw(fmt.Sprintf("cannot Unmarshall \"%s\" into a JsonMap", trailString))
		}
		return jMap
	}

	getAbsPaths := func(absolutePaths *json_map.AbsolutePaths, jMap *jom.JsonMap) []*json_map.JsonPathNode {
		values, errs := jMap.GetAbsolutePaths(absolutePaths)
		if errs != nil {
			throw(utils.JsonPathError.FillFromErrors(errs).Error())
		}
		return values
	}

	// Then we will parse the JSON path
	var absolutePaths json_map.AbsolutePaths
	absolutePaths, err = utils.ParseJsonPath(jsonPath)
	if err != nil {
		throw(err.Error())
	}

	// Set up a temporary vm which we will use to construct the NodeSet
	vmTemp := otto.New()

	// Temp function to push a value to an array stored in the given variable name.
	// This will first marshall the value into a JSON value in Go then parse the []byte to the VM which will use
	// JSON.parse to parse the value and then push it to the array of the given name.
	pushValue := func(arrName string, value interface{}) {
		var valueStr []byte
		valueStr, err = json.Marshal(value)
		if err != nil {
			throw(fmt.Sprintf("cannot Marshal value: \"%v\"", value))
		}
		// Then push the value to the nodeSet array within the VM
		currentName := fmt.Sprintf("%sCurrStr", arrName)
		_ = vmTemp.Set(currentName, string(valueStr))
		_, err = vmTemp.Run(fmt.Sprintf("%s.push(JSON.parse(%s))", arrName, currentName))
		if err != nil {
			throw(fmt.Sprintf("could not push current node: \"%v\", to %s", string(valueStr), arrName))
		}
	}

	var setupKeyObject func(path json_map.AbsolutePathKey) map[string]interface{}
	setupKeyObject = func(path json_map.AbsolutePathKey) map[string]interface{} {
		absolutePathKeyMap := make(map[string]interface{})
		absolutePathKeyMap["typeId"]   = path.KeyType
		absolutePathKeyMap["typeName"] = json_map.AbsolutePathKeyTypeNames[path.KeyType]
		switch path.KeyType {
		case json_map.Slice:
			// In cases of slices we have to setup a new array
			_, _ = vmTemp.Run("sliceArray = []")
			for _, slice := range path.Value.([]json_map.AbsolutePathKey) {
				pushValue("sliceArray", setupKeyObject(slice))
			}
			absolutePathKeyMap["key"], _ = vmTemp.Get("sliceArray")
		default:
			absolutePathKeyMap["key"] = path.Value
		}
		return absolutePathKeyMap
	}

	// Create a node set map which will store the object we need to return
	nodeSetMap := make(map[string]interface{})

	_, _ = vmTemp.Run("absoluteValues = []")
	for _, paths := range absolutePaths {
		_, _ = vmTemp.Run("currentPath = []")
		for _, path := range paths {
			pathMap := setupKeyObject(path)
			var value otto.Value
			value, err = vmTemp.ToValue(pathMap)
			if err != nil {
				throw(err.Error())
			}
			_ = vmTemp.Set("currentKey", value)
			// Append to array
			_, _ = vmTemp.Run("currentPath.push(currentKey)")
		}
		// Append the current path to the absolute paths array
		_, _ = vmTemp.Run("absoluteValues.push(currentPath)")
	}
	nodeSetMap["_absolutePaths"], _ = vmTemp.Get("absoluteValues")

	// Set getter and setter funcs
	nodeSetMap["getValues"] = func(call otto.FunctionCall) otto.Value {
		// Get the most "up to date" json map from json.trail
		jsonMap := getJsonMap(call.Otto)
		_, _ = vmTemp.Run("nodeValues = []")
		nodes := getAbsPaths(&absolutePaths, jsonMap)

		// Expand the first element if we only have one element and its an array
		if len(nodes) == 1 {
			switch nodes[0].Value.(type) {
			case []interface{}:
				expandedNodes := make([]*json_map.JsonPathNode, 0)
				for _, node := range nodes[0].Value.([]interface{}) {
					expandedNodes = append(expandedNodes, &json_map.JsonPathNode{
						Absolute: nodes[0].Absolute,
						Value:    node,
					})
				}
				nodes = expandedNodes
			}
		}

		for _, value := range nodes {
			pushValue("nodeValues", value.Value)
		}
		nodeValues, _ := vmTemp.Get("nodeValues")
		return nodeValues
	}

	nodeSetMap["setValues"] = func(call otto.FunctionCall) otto.Value {
		// Get the most "up to date" json map from json.trail
		jsonMap := getJsonMap(call.Otto)
		if len(call.ArgumentList) == 0 || len(call.ArgumentList) > 1 {
			throw("setValue takes a single argument")
		}
		value := call.Argument(0)
		valueGo := toGo(value)

		// Because the value returned by toGo can indeed by a pointer we need to do some suave reflect method calls to
		// get the indirect value of the pointer if the Kind of value returned is indeed a pointer
		if reflect.ValueOf(valueGo).Kind() == reflect.Ptr {
			valueGo = reflect.Indirect(reflect.ValueOf(valueGo)).Interface()
		}

		// Then we call SetAbsolutePaths
		// NOTE: this is referencing the absolute paths from outside the scope of this function but this doesn't matter
		//       because if the user has changed the JSON structure for the worse then its kinda their fault for using
		//       an out of date NodeSet object
		err = jsonMap.SetAbsolutePaths(&absolutePaths, valueGo)
		if err != nil {
			throw(err.Error())
		}

		// Then we update the current json.trail object with the createJom function which will recreate the jom
		var trail otto.Value
		trail, err = createJom(jsonMap)
		if err != nil {
			throw("Could not JOM-ify modified JsonMap")
		}

		_ = call.Otto.Set(utils.ModifiedTrailValueVarName, trail)
		_, err = call.Otto.Run(fmt.Sprintf("json[\"trail\"] = %s", utils.ModifiedTrailValueVarName))
		if err != nil {
			throw(err.Error() + "gello")
		}
		return otto.NullValue()
	}

	var nodeSet otto.Value
	nodeSet, err = vm.ToValue(nodeSetMap)
	if err != nil {
		throw(fmt.Sprintf("could not convert \"%v\" to otto value", nodeSetMap))
	}
	return nodeSet
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
	// printlnExternal is a legacy version of the console.log
	{"printlnExternal", func(call otto.FunctionCall) otto.Value {
		_, _ = fmt.Fprintf(ExternalConsoleLogStdout, "Print %s", composePrint(call))
		return otto.NullValue()
	}},
}

var builtinVars = []BuiltinVar{
	// Construct the main JOM object
	{utils.JOMVariableName, func(i ...interface{}) interface{} {
		runtime := i[0].(*otto.Otto)
		jsonMap := i[1].(json_map.JsonMapInt)
		trail, err := createJom(jsonMap)
		if err != nil {
			panic(utils.BuiltinGetterError.FillError("json.trail", "Could not JOM-ify"))
		}
		jom := map[string]interface{} {
			"trail": trail,
			"jsonPathSelector": jsonPathSelector,
			"scopePath": jsonMap.GetCurrentScopePath(),
		}
		if val, err := runtime.ToValue(jom); err != nil {
			panic(utils.BuiltinGetterError.FillError("json", "Could not convert JOM into otto.Value"))
		} else {
			return val
		}
	}},
	{"console", func(i ...interface{}) interface{} {
		// Sets up the console object
		runtime := i[0].(*otto.Otto)
		consoleObj := map[string]interface{} {
			"log": func(call otto.FunctionCall)otto.Value {
				_, _ = fmt.Fprintf(ExternalConsoleLogStdout, "Print %s", composePrint(call))
				return otto.NullValue()
			},
			"error": func(call otto.FunctionCall)otto.Value {
				// Redirect to stderr
				_, _ = fmt.Fprintf(ExternalConsoleLogStderr, "Error %s", composePrint(call))
				return otto.NullValue()
			},
		}
		if val, err := runtime.ToValue(consoleObj); err != nil {
			panic(utils.BuiltinGetterError.FillError("console", "Could not convert console obj to otto.Value"))
		} else {
			return val
		}
	}},
}

// Create the JOM within a Javascript VM, assign all necessary functions and retrieve the variable from within the VM.
// This will create a JOM for the scope of the given json map.
// NOTE this needs to be used to correctly parse Go arrays ([]interface{}) as JS arrays and not JS objects
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
	// NOTE JSON.stringify will strip keys that are functions out from the object
	run, err := env.Run(fmt.Sprintf("JSON.stringify(%s.trail)", utils.JOMVariableName))
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
	// Create the VM and register all builtins
	vm := otto.New()
	// Register all builtins
	for _, builtin := range builtinFuncs {
		if err := vm.Set(builtin.name, builtin.function); err != nil {
			panic(err)
		}
	}
	for _, builtin := range builtinVars {
		var err error
		switch builtin.name {
		case utils.JOMVariableName:
			err = vm.Set(builtin.name, builtin.getter(vm, jsonMap))
		case "console":
			err = vm.Set(builtin.name, builtin.getter(vm))
		default:
			err = vm.Set(builtin.name, builtin.getter())
		}
		// Panic if there was an error
		if err != nil {
			panic(err)
		}
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
					fmt.Sprintf(utils.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), script),
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
		time.Sleep(time.Duration(utils.HaltingDelay) * utils.HaltingDelayUnits)
		vm.Interrupt <- func() {
			panic(utils.HaltingProblem)
		}
	}()
	// Run the script
	_, err = vm.Run(script)
	if err != nil {
		// Re-wrap the error as a ScriptError
		return nil, utils.ScriptError.FillError(err.Error(), fmt.Sprintf(utils.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), script))
	}

	// De-JOM-ify the environment and return the json_map.JsonMapInt
	data, err = deJomIfy(jsonMap, vm)
	if err != nil {
		return nil, err
	}
	return data, nil
}
