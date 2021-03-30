package jom

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"github.com/hjson/hjson-go"
	"strings"
)

// Traversal object which is composed within JsonMap. Holds some info about the current traversal.
type Traversal struct {
	scopePath *strings.Builder
	script    map[string]interface{}
	nonScript map[string]interface{}
}

// Creates a new Traversal object (used within JsonMap)
func newTraversal() *Traversal {
	return &Traversal{
		// The scopePath to the current scope of traversal (JSONPath)
		scopePath: &strings.Builder{},
		// Contains all fields (key-values) that are scripts within insides
		script:    make(map[string]interface{}),
		// Contains all fields (key-values) that are not scripts within insides
		nonScript: make(map[string]interface{}),
	}
}

// Wrapper for map[string]interface{} that can be easily extensible with more functionality
type JsonMap struct {
	// The inner workings, aka. a map.
	insides   map[string]interface{}
	// Used for certain traversal logic
	traversal *Traversal
	// Whether or not the JSON has an Array at its root
	Array bool
}

// Returns the current scopes JSON Path to itself.
// This just uses the string builder within the traversal field
func (jsonMap *JsonMap) GetCurrentScopePath() string {
	return jsonMap.traversal.scopePath.String()
}

// Getter for insides.
// Useful when using json_map.JsonMapInt
func (jsonMap *JsonMap) GetInsides() *map[string]interface{} {
	return &jsonMap.insides
}

// Return a clone of the JsonMap. If clear is given then a New will be called and returned.
// NOTE this is primarily used when using json_map.JsonMapInt to return a new JsonMap to avoid cyclic imports
func (jsonMap *JsonMap) Clone(clear bool) json_map.JsonMapInt {
	if !clear {
		return &JsonMap{
			insides:   jsonMap.insides,
			traversal: jsonMap.traversal,
		}
	}
	cleared := New()
	// The cleared Array still inherits the Array field from the Cloned Array
	cleared.Array = jsonMap.Array
	return cleared
}

// Construct a new empty JsonMap.
// Returns a pointer to a JsonMap.
func New() *JsonMap {
	return &JsonMap{
		insides:   make(map[string]interface{}),
		traversal: newTraversal(),
		Array:     false,
	}
}

// Constructs a new JsonMap from the given string->interface{} map
// Returns a pointer to a JsonMap
func NewFromMap(jsonMap map[string]interface{}) *JsonMap {
	return &JsonMap{
		insides:   jsonMap,
		traversal: newTraversal(),
		Array:     false,
	}
}

// Check if the given string contains a json-dom script.
// This is done by checking the first line of the string and seeing if it starts with the ShebangPrefix and ends with
// one of the supported languages.
// Panics if the shebang fits the required length for a shebang but is not a supported script language.
// Returns true if the script does contain a json-dom script, false otherwise. Along with the retrieved script language.
func CheckIfScript(script string) (isScript bool, shebangScriptLang string) {
	firstLine := strings.Split(script, "\n")[0]
	firstLen := len(firstLine)

	// First check the bounds of the line so that we won't panic
	if firstLen >= utils.ShebangLen + utils.ShortestSupportedScriptTagLen && firstLen <= utils.ShebangLen + utils.LongestSupportedScriptTagLen {
		shebangPrefix, shebangScriptLang := firstLine[:utils.ShebangLen], firstLine[utils.ShebangLen:]
		if shebangPrefix != utils.ShebangPrefix {
			return false, shebangScriptLang
		}
		if !utils.GetSupportedScriptTags()[shebangScriptLang] {
			// We are going to panic here as the script is unsupported
			// NOTE this will only panic when the shebang script is between the shorted and the longest supported lengths
			panic(utils.UnsupportedScriptLang.FillError(shebangScriptLang, fmt.Sprintf(utils.ScriptErrorFormatString, utils.AnonymousScriptPath, script)))
		}
		return true, shebangScriptLang
	}
	return false, shebangScriptLang
}

// Finds all the script and non-script fields within a JsonMap.
// Updates the script and nonScript fields within the JsonMap's traversal object.
func (jsonMap *JsonMap) FindScriptFields() (found bool) {
	// Map to keep the script key values and map to keep all key values apart from the script fields

	// Indicates whether a script tag has been found at the current depth or a nested depth. Used to indicate when to
	// join a scriptFields subtree to its parent tree.
	found = false

	for key, element := range (*jsonMap).insides {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse down the inner map
			innerMap := NewFromMap(element.(map[string]interface{}))
			foundInner := innerMap.FindScriptFields()
			// Join the two trees if there was something found
			if foundInner {
				// Also set found to true as we've found something deeper down
				found = true
				jsonMap.traversal.script[key] = innerMap.traversal.script
			}
			// Always join the nonScriptFieldsInner back into the main tree (nonScriptFields)
			jsonMap.traversal.nonScript[key] = innerMap.traversal.nonScript
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
					innerMap := NewFromMap(inner.(map[string]interface{}))
					foundInnerInner := innerMap.FindScriptFields()
					if foundInnerInner {
						foundInner = true
						scriptArrayInner[i] = innerMap.traversal.script
					}
					// Always join nonScriptFieldsInner back into main array (nonScriptArrayInner)
					nonScriptArrayInner[i] = innerMap.traversal.nonScript
				default:
					// Fill current element with nil in the scriptArray to indicate that there is no script here
					scriptArrayInner[i] = nil
					nonScriptArrayInner[i] = inner
				}
			}

			// If any scripts were found in the scope of the array then assign the array to the current key
			if foundInner {
				jsonMap.traversal.script[key] = scriptArrayInner
			}
			// Always join nonScriptArrayInner back into the main tree (nonScriptFields)
			jsonMap.traversal.nonScript[key] = nonScriptArrayInner
		case string:
			// Check if the element contains a script
			if isScript, _ := CheckIfScript(element.(string)); isScript {
				// If it is then add the key to the scriptFields map and set found to true
				found = true
				jsonMap.traversal.script[key] = element
			} else {
				// Add the field to the nonScriptFields map
				jsonMap.traversal.nonScript[key] = element
			}
		default:
			// Add the field to the nonScriptFields map
			jsonMap.traversal.nonScript[key] = element
		}
	}

	return found
}

// Given a JsonMap this will traverse it and execute all scripts. Will update the given JsonMap in place.
// - All scripts will be run and removed from the JsonMap
// - In cases where there are more than one script tag on a level: scripts will be evaluated in lexicographical script-key order
func (jsonMap *JsonMap) Run() {
	// At every level of the json map
	// 1. Create a script priority queue of all the script tags at that level
	// 2. While the script queue isn't empty ->
	// 		1. Run the script in the script lang's environment using code.Run -> new scope JsonMap
	//		2. Delete the script from the new De-JOM-ified JsonMap
	//		3. Set the current scope to the De-JOM-ified JsonMap
	// 3. Iterate over each key in the new updated scope
	// 		1. If the element at the key is an array:
	//			- Iterate over array and recurse whenever there is an object (remember to update the traversal scopePath)
	//			- Shouldn't need to be joined back into main tree as it should have been done by step 2 (pointers)
	//		2. If the element at the key is an object:
	//			- Remember to update the traversal scopePath
	//			- Recurse into the object
	//		3. Default just passes
	// Find all the script fields
	jsonMap.FindScriptFields()
	// Set up path
	if jsonMap.traversal.scopePath.Len() == 0 {
		_, _ = fmt.Fprint(jsonMap.traversal.scopePath, "$")
	}

	// Get all script keys at the current level
	scriptQueue := make(utils.StringHeap, 0)
	for k, e := range jsonMap.traversal.script {
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

		// Get the script language by CheckIfScript
		script := jsonMap.traversal.script[scriptKey].(string)
		_, scriptLang := CheckIfScript(script)

		// Run the script for the script's language. This will...
		// 1. Create the JOM object, setup any builtin functions and insert the JOM into the script environment
		// 2. Setup any interrupts for the halting problem
		// 3. Extract and decode the JOM from the environment and return it
		// Any errors that occur have to be panicked as they can effect the entire runtime
		newScope, err := code.Run(scriptLang, script, jsonMap)
		if err != nil {
			panic(err)
		}

		// Delete the script key from the newScope
		delete(*newScope.GetInsides(), scriptKey)
		// Set the current scope to the new scope
		(*jsonMap).insides = *newScope.GetInsides()
	}

	// Iterate over each key within the new scope (or the same scope if no scripts were run)
	for key, element := range (*jsonMap).insides {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse when there is a nested object
			jsonInnerMap := NewFromMap(element.(map[string]interface{}))
			// Remember to update the scope path of the new JsonMap
			_, _ = fmt.Fprintf(jsonInnerMap.traversal.scopePath, "%s.%s", jsonMap.traversal.scopePath.String(), key)
			jsonInnerMap.Run()
			// Join the subtree back into the main tree
			jsonMap.insides[key] = jsonInnerMap.insides
		case []interface{}:
			elementArray := element.([]interface{})
			// Iterate over array and recurse on all objects that may be inside the array
			for i, inner := range elementArray {
				switch inner.(type) {
				case map[string]interface{}:
					jsonInnerInnerMap := NewFromMap(inner.(map[string]interface{}))
					// Remember to update the scope path of the new JsonMap
					_, _ = fmt.Fprintf(jsonInnerInnerMap.traversal.scopePath, "%s.%s.[%d]", jsonMap.traversal.scopePath.String(), key, i)
					jsonInnerInnerMap.Run()
					// Join the subtree back into the array
					elementArray[i] = jsonInnerInnerMap.insides
				}
			}
			// Join array back into the main tree
			jsonMap.insides[key] = elementArray
		}
	}
}

// Unmarshal a hjson byte string and package it as a JsonMap.
func (jsonMap *JsonMap) Unmarshal(jsonBytes []byte) (err error) {
	// Decode and a check for errors.
	if err = hjson.Unmarshal(jsonBytes, &jsonMap.insides); err != nil {
		// FIXME Find a better way of handling JSON with an array at their root
		if strings.Contains(err.Error(), "value of type []interface {} is not assignable to type map[string]interface {}") {
			jsonMap.Array = true
			// Create an "array" key within jsonMap.insides that will contain the unmarshalled array
			// When Marshalling into JSON check if this hack was used and deal with it accordingly
			var rootArray []interface{}
			if err = hjson.Unmarshal(jsonBytes, &rootArray); err == nil {
				jsonMap.insides["array"] = rootArray
			}
		}
		if !jsonMap.Array {
			return err
		}
	}
	return nil
}

// Marshal a JsonMap back into JSON.
func (jsonMap *JsonMap) Marshal() (out []byte, err error) {
	// Marshal the output JSON
	if !jsonMap.Array {
		out, err = json.Marshal(jsonMap.insides)
	} else {
		// Handle the root array hack
		out, err = json.Marshal(jsonMap.insides["array"])
	}
	return out, err
}

// Evaluates the scripts within a given hjson byte array.
// Should really only be called from within CLI main.
// Returns the evaluated JSON as a byte array and nil if everything is good. Otherwise an empty byte array and an error
// will be returned if an error occurs.
func Eval(jsonBytes []byte, verbose bool) (out []byte, err error) {
	// Create map to keep decoded data
	jsonMap := New()

	// Unmarshal into the JsonMap
	err = jsonMap.Unmarshal(jsonBytes)
	if err != nil {
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
	jsonMap.Run()

	if verbose {
		fmt.Println("\nJomMap:", jsonMap.insides)
	}

	// Marshal the output JSON
	out, err = jsonMap.Marshal()
	if err != nil {
		return out, err
	}
	return out, nil
}
