package main

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/utils"
	"github.com/hjson/hjson-go"
	"github.com/robertkrimen/otto"
	"strings"
	"time"
)


// Check if the given string contains a json-dom script.
// This is done by checking the first line of the string and seeing if it starts with the ShebangPrefix and ends with
// one of the supported languages.
// Returns true if the script does contain a json-dom script, false otherwise.
func CheckIfScript(script string) bool {
	firstLine := strings.Split(script, "\n")[0]
	firstLen := len(firstLine)
	// First check the bounds of the line so that we won't panic
	if firstLen >= utils.ShebangLen + utils.ShortestSupportedScriptTagLen && firstLen <= utils.ShebangLen + utils.LongestSupportedScriptTagLen {
		return firstLine[:utils.ShebangLen] == utils.ShebangPrefix && utils.GetSupportedScriptTags()[firstLine[utils.ShebangLen:]]
	}
	return false
}

// Find all the fields within the JSON that contain a script header.
// Returns a map of all the fields that contain a script header to the value of that script field (the script itself).
// Along with a copy of the JSON without any of the script tags. Along with a boolean which indicates whether any
// scripts were found.
func FindScriptFields(json *map[string]interface{}) (map[string]interface{}, map[string]interface{}, bool) {
	// Map to keep the script key values
	scriptFields := make(map[string]interface{})
	// Map to keep all key values apart from the script fields
	nonScriptFields := make(map[string]interface{})

	// Indicates whether a script tag has been found at the current depth or a nested depth. Used to indicate when to
	// join a scriptFields subtree to its parent tree.
	found := false

	for key, element := range *json {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse down the inner map
			innerMap := element.(map[string]interface{})
			scriptFieldsInner, nonScriptFieldsInner, foundInner := FindScriptFields(&innerMap)
			// Join the two trees if there was something found
			if foundInner {
				// Also set found to true as we've found something deeper down
				found = true
				scriptFields[key] = scriptFieldsInner
			}
			// Always join the nonScriptFieldsInner back into the main tree (nonScriptFields)
			nonScriptFields[key] = nonScriptFieldsInner
		case []interface{}:
			// Allocate a matching array
			arrayLen := len(element.([]interface{}))
			scriptArrayInner := make([]interface{}, arrayLen)
			nonScriptArrayInner := make([]interface{}, arrayLen)
			foundInner := false

			for i, inner := range element.([]interface{}) {
				switch inner.(type) {
				case map[string]interface{}:
					// Recurse over all objects
					innerMap := inner.(map[string]interface{})
					scriptFieldsInner, nonScriptFieldsInner, foundInnerInner := FindScriptFields(&innerMap)
					if foundInnerInner {
						foundInner = true
						scriptArrayInner[i] = scriptFieldsInner
					}
					// Always join nonScriptFieldsInner back into main array (nonScriptArrayInner)
					nonScriptArrayInner[i] = nonScriptFieldsInner
				default:
					// Fill current element with nil in the scriptArray to indicate that there is no script here
					scriptArrayInner[i] = nil
					nonScriptArrayInner[i] = inner
				}
			}

			// If any scripts were found in the scope of the array then assign the array to the current key
			if foundInner {
				scriptFields[key] = scriptArrayInner
			}
			// Always join nonScriptArrayInner back into the main tree (nonScriptFields)
			nonScriptFields[key] = nonScriptArrayInner
		case string:
			// Check if the element contains a script
			if CheckIfScript(element.(string)) {
				// If it is then add the key to the scriptFields map and set found to true
				found = true
				scriptFields[key] = element
			} else {
				// Add the field to the nonScriptFields map
				nonScriptFields[key] = element
			}
		default:
			// Add the field to the nonScriptFields map
			nonScriptFields[key] = element
		}
	}

	return scriptFields, nonScriptFields, found
}

// Create the JOM within a Javascript VM, assign all necessary functions and retrieve the variable from within the VM.
// This will create a JOM for the scope of the given json map.
// Returns an otto.Value which can be plugged into the VM which will run the scripts. If an error occurs at any point
// then an otto.NullValue and the error are returned.
func CreateJom(jsonMap *map[string]interface{}) (otto.Value, error) {
	// Convert the map to json
	jsonDataBytes, err := json.Marshal(*jsonMap)
	if err != nil {
		return otto.NullValue(), err
	}
	jsonData := string(jsonDataBytes)

	// Create a VM, parse the json string and get the value out of the VM
	vm := code.NewVM()
	if err := vm.Set("jsonString", jsonData); err != nil {
		return otto.NullValue(), err
	}
	run, err := vm.Run("JSON.parse(jsonString)")
	if err != nil {
		return otto.NullValue(), err
	}

	// TODO At some point introduce some helpful functions and helpers to the JOM

	return run, nil
}

// Given a JS environment, retrieve the JOM and generate the JsonMap for the object
// Returns the JsonMap of the converted JOM and any errors (if there are any)
func DeJomIfy(env *otto.Otto) (map[string]interface{}, error) {
	// TODO this will need to change when the CreateJom function changes. Such as when new helper functions are introduced
	data := make(map[string]interface{})

	// Stringify and return the JOM (as a string)
	run, err := env.Run("JSON.stringify(json)")
	if err != nil {
		return data, err
	}

	// Unmarshal the JSON string to convert it into a map
	if err := json.Unmarshal([]byte(run.String()), &data); err != nil {
		return data, err
	}
	return data, nil
}

// Run the given script, with the given JOM and return the Otto environment
func RunScript(script string, jom otto.Value) (vm *otto.Otto, err error) {
	// Create the VM
	vm = code.NewVM()
	// Pass the JOM into the environment
	if err = vm.Set(utils.JOMVariableName, jom); err != nil {
		return vm, err
	}
	// Remove the shebang line from the script
	script = strings.Join(strings.Split(script, "\n")[1:], "\n")

	// To stop infinite loops start a timer which will panic once the timer stops
	start := time.Now()
	// This will catch any panics thrown by running the script/the timer
	defer func() {
		duration := time.Since(start)
		if caught := recover(); caught != nil {
			// If the caught error is the HaltingProblem var then package it up using FillError and set the outer error
			if caught == utils.HaltingProblem {
				err = utils.HaltingProblem.FillError(
					duration.String(),
					fmt.Sprintf("script: %s", script),
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
		return vm, err
	}
	return vm, err
}

func RunScripts(jsonMap *map[string]interface{}) {
	// At every level of the json map
	// 1. Find all the script tags at that level and below
	// 2. Create a script queue of all the script tags at that level
	// 3. If there is a script tag at that level ->
	// 		1. Generate the JOM for that scope (using JsonMap with scripts) using CreateJom
	//		2. Insert JOM into VM
	//		3. Run current script
	//		4. De-JOM-ify the "json" object from the VM -> JsonMap
	//		5. Delete the script from the new De-JOM-ified JsonMap
	//		6. Set the current scope to the De-JOM-ified JsonMap
	// 4. Iterate over each key in the new updated scope
	// 		1. If the element at the key is an array:
	//			- Iterate over array and recurse whenever there is an object
	//			- Shouldn't need to be joined back into main tree as it should have been done by step 2 (pointers)
	//		2. If the element at the key is an object:
	//			- Recurse into the object
	//		3. Default just passes
	scriptMap, _, _ := FindScriptFields(jsonMap)

	// Get all script keys at the current level
	scriptQueue := make(utils.StringHeap, 0)
	for k, e := range scriptMap {
		switch e.(type) {
		case string:
			scriptQueue = append(scriptQueue, k)
		default:
			continue
		}
	}
	// Initialise the heap so that all script tags can be dequeued in lexicographical order
	heap.Init(&scriptQueue)

	// Iterate over all scripts
	for scriptQueue.Len() > 0 {
		// Dequeue the scriptKey from the scriptQueue
		scriptKey := heap.Pop(&scriptQueue).(string)

		// Create the JOM for the current scope
		jom, err := CreateJom(jsonMap)
		if err != nil {
			panic(err)
		}
		// Run the script (inserting the JOM as a var)
		vm, err := RunScript(scriptMap[scriptKey].(string), jom)
		if err != nil {
			panic(err)
		}
		// De-JOM-ify the object
		newScope, err := DeJomIfy(vm)
		if err != nil {
			panic(err)
		}
		// Delete the script key from the newScope
		delete(newScope, scriptKey)
		// Set the current scope to the new scope
		*jsonMap = newScope
	}

	// Iterate over each key within the new scope (or the same scope if no scripts were run)
	for key, element := range *jsonMap {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse when there is a nested object
			jsonInnerMap := element.(map[string]interface{})
			RunScripts(&jsonInnerMap)
			// Join the subtree back into the main tree
			(*jsonMap)[key] = jsonInnerMap
		case []interface{}:
			elementArray := element.([]interface{})
			// Iterate over array and recurse on all objects that may be inside the array
			for i, inner := range elementArray {
				switch inner.(type) {
				case map[string]interface{}:
					jsonInnerInnerMap := inner.(map[string]interface{})
					RunScripts(&jsonInnerInnerMap)
					// Join the subtree back into the array
					elementArray[i] = jsonInnerInnerMap
				}
			}
			// Join array back into the main tree
			(*jsonMap)[key] = elementArray
		}
	}
}

func Eval(jsonBytes []byte, verbose bool) (out []byte, err error) {
	// Create map to keep decoded data
	var jsonMap map[string]interface{}

	// Decode and a check for errors.
	if err = hjson.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return out, err
	}

	// Catch any panics that might happen when running scripts
	defer func() {
		if p := recover(); p != nil {
			// Set the error so that it is returned
			err = errors.New(fmt.Sprintf("Error occured while evaluating JSON-DOM: %v", p))
			return
		}
	}()

	// Run the scripts within each scope of the JsonMap
	RunScripts(&jsonMap)

	if verbose {
		fmt.Println()
		fmt.Println("JomMap:", jsonMap)
	}

	// Marshal the output JSON
	out, err = json.Marshal(jsonMap)
	if err != nil {
		return out, err
	}
	return out, nil
}
