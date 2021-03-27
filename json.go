package main

import (
	"encoding/json"
	"fmt"
	"github.com/andygello555/json-dom/utils"
	"github.com/hjson/hjson-go"
	"github.com/robertkrimen/otto"
	"strings"
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
func FindScriptFields(json map[string]interface{}) (map[string]interface{}, map[string]interface{}, bool) {
	// Map to keep the script key values
	scriptFields := make(map[string]interface{})
	// Map to keep all key values apart from the script fields
	nonScriptFields := make(map[string]interface{})

	// Indicates whether a script tag has been found at the current depth or a nested depth. Used to indicate when to
	// join a scriptFields subtree to its parent tree.
	found := false

	for key, element := range json {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse down the inner map
			scriptFieldsInner, nonScriptFieldsInner, foundInner := FindScriptFields(element.(map[string]interface{}))
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
					scriptFieldsInner, nonScriptFieldsInner, foundInnerInner := FindScriptFields(inner.(map[string]interface{}))
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
// This will create a JOM for the scope of the given json map. To generate the necessary JOM for all script scopes then
// a traversal must be done.
// Returns an otto.Value which can be plugged into the VM which will run the scripts. If an error occurs at any point
// then an otto.NullValue and the error are returned.
func CreateJom(jsonMap map[string]interface{}) (otto.Value, error) {
	// Convert the map to json
	jsonDataBytes, err := json.Marshal(jsonMap)
	if err != nil {
		return otto.NullValue(), err
	}
	jsonData := string(jsonDataBytes)

	// Create a VM, parse the json string and get the value out of the VM
	vm := otto.New()
	if err := vm.Set("jsonString", jsonData); err != nil {
		return otto.NullValue(), err
	}
	run, err := vm.Run("JSON.parse(jsonString)")
	if err != nil {
		return otto.NullValue(), err
	}

	// At some point introduce some helpful functions and helpers to the JOM

	return run, nil
}

// The traverse function that is used by TraverseJsonMap
type TraverseJsonMapFunc func(...*map[string]interface{})

func TraverseJsonMap(scriptMap *map[string]interface{}, nonScriptMap *map[string]interface{}, extraMap *map[string]interface{}, mapFunc TraverseJsonMapFunc) {
	// Iterate over script fields
	for key, element := range *scriptMap {
		switch element.(type) {
		case string:
			// When there is a script then call the mapFunc
			mapFunc(nonScriptMap, extraMap)
		case map[string]interface{}:
			// Recurse when there is a nested object
			scriptMapInner, nonScriptMapInner := element.(map[string]interface{}), (*nonScriptMap)[key].(map[string]interface{})
			scope := make(map[string]interface{})
			(*extraMap)[key] = scope
			TraverseJsonMap(&scriptMapInner, &nonScriptMapInner, &scope, mapFunc)
		case []interface{}:
			// Allocate a matching array
			innerArray := element.([]interface{})
			extraArrayInner := make([]interface{}, len(innerArray))

			// Iterate over array and find the non-nil elements and recurse into them to fill out scopes
			for i, inner := range innerArray {
				if inner != nil {
					// Recurse into scope
					scriptMapInner, nonScriptMapInner := inner.(map[string]interface{}), (*nonScriptMap)[key].([]interface{})[i].(map[string]interface{})
					scope := make(map[string]interface{})
					TraverseJsonMap(&scriptMapInner, &nonScriptMapInner, &scope, mapFunc)
					extraArrayInner[i] = scope
				} else {
					// Otherwise assign current element to nil
					extraArrayInner[i] = nil
				}
			}
			// Assign array subtree back onto main subtree
			(*extraMap)[key] = extraArrayInner
		}
	}
}

// Create a JOM for all scopes in the scriptMap.
// Cross reference each script "path" in the scriptMap with the nonScriptMap.
// Return a map of the JOM otto.Values to
func CreateJomScopes(scriptMap map[string]interface{}, nonScriptMap map[string]interface{}) {

}

func Eval(jsonBytes []byte) (map[string]interface{}, error) {
	// Create map to keep decoded data
	var jsonMap map[string]interface{}

	// Decode and a check for errors.
	if err := hjson.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return jsonMap, err
	}

	// Find script fields
	scripts, nonScript, _ := FindScriptFields(jsonMap)
	fmt.Println()
	fmt.Println("Eval script fields:", scripts)
	fmt.Println()
	fmt.Println("Eval non-script fields:", nonScript)

	// Generate JOM for each scope in the scripts map
	jomMap := make(map[string]interface{})
	TraverseJsonMap(&scripts, &nonScript, &jomMap, func(maps ...*map[string]interface{}) {
		// Check if there is already a JOM for this scope
		if _, ok := (*maps[1])["JOM"]; !ok {
			// Generate the JOM for the current scope
			jom, err := CreateJom(*maps[0])
			if err != nil {
				panic(err)
			}
			// Assign the JOM to the JOM key of the correct scope
			(*maps[1])["JOM"] = jom
		}
	})
	fmt.Println()
	fmt.Println("JomMap:", jomMap)
	return jsonMap, nil
}
