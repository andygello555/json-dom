package main

import (
	"fmt"
	"github.com/andygello555/json-dom/utils"
	"github.com/hjson/hjson-go"
	"strings"
)

// Check if the given string contains a json-dom script
// This is done by checking the first line of the string and seeing if it starts with the ShebangPrefix and ends with
// one of the supported languages
// Returns true if the script does contain a json-dom script, false otherwise
func CheckIfScript(script string) bool {
	firstLine := strings.Split(script, "\n")[0]
	firstLen := len(firstLine)
	// First check the bounds of the line so that we won't panic
	if firstLen >= utils.ShebangLen + utils.ShortestSupportedScriptTagLen && firstLen <= utils.ShebangLen + utils.LongestSupportedScriptTagLen {
		return firstLine[:utils.ShebangLen] == utils.ShebangPrefix && utils.GetSupportedScriptTags()[firstLine[utils.ShebangLen:]]
	}
	return false
}

// Find all the fields within the JSON that contain a script header
// Return a map of all the fields that contain a script header to the value of that script field (the script itself).
// Along with a copy of the JSON without any of the script tags. Along with a boolean which indicates whether any
// scripts were found
func FindScriptFields(json map[string]interface{}) (map[string]interface{}, bool) {
	scriptFields := make(map[string]interface{})
	// Indicates whether a script tag has been found at the current depth or a nested depth. Used to indicate when to
	// join a scriptFields subtree to its parent tree.
	found := false

	for key, element := range json {
		switch element.(type) {
		case map[string]interface{}:
			// Recurse down the inner map
			scriptFieldsInner, foundInner := FindScriptFields(element.(map[string]interface{}))
			// Join the two trees if there was something found
			if foundInner {
				// Also set found to true as we've found something deeper down
				found = true
				scriptFields[key] = scriptFieldsInner
			}
		case []interface{}:
			// Allocate a matching array
			arrayInner := make([]interface{}, len(element.([]interface{})))
			foundInner := false

			for i, inner := range element.([]interface{}) {
				switch inner.(type) {
				case map[string]interface{}:
					// Recurse over all objects
					scriptFieldsInner, foundInnerInner := FindScriptFields(inner.(map[string]interface{}))
					if foundInnerInner {
						foundInner = true
						arrayInner[i] = scriptFieldsInner
					}
				default:
					// Fill current element with nil to indicate that there is nothing here
					arrayInner[i] = nil
				}
			}

			// If any scripts were found in the scope of the array then assign the array to the current key
			if foundInner {
				scriptFields[key] = arrayInner
			}
		case string:
			// Check if the element contains a script
			if CheckIfScript(element.(string)) {
				// If it is then add the key to the scriptFields map and set found to true
				found = true
				scriptFields[key] = element
			}
		}
	}

	return scriptFields, found
}

func Eval(jsonBytes []byte) (map[string]interface{}, error) {
	// Create map to keep decoded data
	var json map[string]interface{}

	// Decode and a check for errors.
	if err := hjson.Unmarshal(jsonBytes, &json); err != nil {
		return json, err
	}

	// Find script fields
	scripts, _ := FindScriptFields(json)
	fmt.Println()
	fmt.Println("Eval:", scripts)
	return json, nil
}
