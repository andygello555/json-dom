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
	"github.com/robertkrimen/otto"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
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

func pathFinder(path []json_map.AbsolutePathKey, jsonMap map[string]interface{}, errChan chan<- error, valChan chan<- json_map.JsonPathNode, wg *sync.WaitGroup) {
	defer wg.Done()
	var currValue interface{} = jsonMap
	var err error = nil

	// Iterate through the absolute path keys
	for _, key := range path {
		// StartEnd KeyTypes must be within a Slice key type so throw an error if so
		if key.KeyType == json_map.StartEnd {
			err = utils.JsonPathError.FillError("Cannot have a start/end key type outside a slice key type")
			break
		}

		// Check the type of the current value and take the according value
		switch currValue.(type) {
		case map[string]interface{}:
			var ok bool
			m := currValue.(map[string]interface{})
			switch key.KeyType {
			case json_map.StringKey:
				if currValue, ok = m[key.Value.(string)]; !ok {
					// If the key does not exist then push to the error channel and return
					err = utils.JsonPathError.FillError(fmt.Sprintf("Key '%v' does not exist in map", key.Value))
					break
				}
			case json_map.IndexKey | json_map.Slice:
				err = utils.JsonPathError.FillError(fmt.Sprintf("Cannot access map %v with numerical key %v", currValue, key.Value))
				break
			case json_map.Wildcard:
				// For wildcards return all the values of each key within the map
				// To ensure that the same key isn't pulled twice when evaluating paths like this:
				// {"person", "friends", 0, *, 0}
				// {"person", "friends", 0, *, 1}
				// We have to first push all the keys to a heap and then pop them off so that we have the same order
				// each time we evaluate this map
				keyQueue := make(utils.StringHeap, 0)
				for k := range m {
					keyQueue = append(keyQueue, k)
				}
				heap.Init(&keyQueue)

				// Add the values of each key to a slice then set that slice to be the current value
				currValueArr := make([]interface{}, 0)
				for keyQueue.Len() > 0 {
					currValueArr = append(currValueArr, m[heap.Pop(&keyQueue).(string)])
				}
				currValue = currValueArr
			case json_map.First:
				// Similar as with the wildcards we sort the keys alphabetically then set the value of the first, THAT
				// IS A MAP, as the current value
				keys := make([]string, 0)
				for k := range m {
					switch m[k].(type) {
					case map[string]interface{}, []interface{}:
						keys = append(keys, k)
					default:
						continue
					}
				}
				// If there is nothing to recurse down then throw error
				if len(keys) == 0 {
					err = utils.JsonPathError.FillError(fmt.Sprintf("There are no maps to recurse down in %v", m))
					break
				}
				// Otherwise sort the strings and take the value of the first key as the new current value
				sort.Strings(keys)
				currValue = m[keys[0]]
			default:
				err = utils.JsonPathError.FillError(fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised", key.KeyType))
				break
			}
		case []interface{}:
			arr := currValue.([]interface{})
			switch key.KeyType {
			case json_map.StringKey:
				err = utils.JsonPathError.FillError(fmt.Sprintf("Cannot access array %v with string index %v", arr, key.Value))
				break
			case json_map.IndexKey:
				i := key.Value.(int)
				if i >= len(arr) || i < 0 {
					err = utils.JsonPathError.FillError(fmt.Sprintf("Index (%d) is out of bounds for array of length %d", i, len(arr)))
					break
				}
				fmt.Println("Getting index:", i, "from", arr, "=", arr[i])
				currValue = arr[i]
			case json_map.Wildcard:
				// If a wildcard then just set the current value to be equal to the array
				currValue = arr
			case json_map.Filter:
				// For Filters we first have to replace all all @ chars with the current node that has been Marshalled
				// into JSON then JSON.parse-d. And we have to also replace all the JSON paths with calculated literals
				stringLiterals := regexp.MustCompile("['\"]([^\\\\\"']|\\\\.)*['\"]")
				jsonPathRegex := regexp.MustCompile("\\$[.\\[][^'\"\\n\\r\\s]+")
				filterExp := []byte(key.Value.(string))

				// Get the locations of the start and end of all string literals
				stringLiteralLocs := stringLiterals.FindAllIndex(filterExp, -1)
				fmt.Println("string literal locations:", stringLiteralLocs)


				// We can replace all occurrences of any JSON path expressions within the filter expression with the
				// literal values to which they evaluate to straight away so that they don't interfere with current node
				// replacement
				jsonPathLocs := jsonPathRegex.FindAllIndex(filterExp, -1)
				// Stores the locations of each JSON path that occurs within the filter expression
				jsonPathIndices := make([][]int, 0)
				// Stores the marshalled literal values
				jsonPathLiterals := make([]string, 0)

				if len(jsonPathLocs) > 0 {
					myJson := NewFromMap(jsonMap)
					// Then we do a similar thing for JSON path expressions within the filter expression
					for _, jsonPathLoc := range jsonPathLocs {
						within := false
						for _, stringLiteralLoc := range stringLiteralLocs {
							if jsonPathLoc[1] - jsonPathLoc[0] <= stringLiteralLoc[1] - stringLiteralLoc[0] - 2 {
								if jsonPathLoc[0] >= stringLiteralLoc[0] && jsonPathLoc[1] <= stringLiteralLoc[1] {
									within = true
									break
								}
							}
						}

						// If the json path is not within any string literals then evaluate the path to find the values
						// and unwrap the returned values and wrap them back up in an index struct
						if !within {
							// We can find the JSON path straight away and append its location and values to the slice
							var values []*json_map.JsonPathNode
							fmt.Println("evaluating JSON path:", string(filterExp)[jsonPathLoc[0]:jsonPathLoc[1]])
							values, err = myJson.JsonPathSelector(string(filterExp)[jsonPathLoc[0]:jsonPathLoc[1]])
							if err != nil {
								break
							}

							var valueRaw interface{}
							// If we only have one node then we can just set that as the value
							if len(values) == 1 {
								valueRaw = values[0].Value
							} else {
								// Unwrap the values into an interface{} slice and set the index struct value to the unwrapped slice
								valueArr := make([]interface{}, 0)
								for _, node := range values {
									valueArr = append(valueArr, node.Value)
								}
								valueRaw = valueArr
							}

							// Then we marshall the value into a JS datatype using json.Marshall
							var literal []byte
							literal, err = json.Marshal(valueRaw)
							if err != nil {
								break
							}

							// We then add the location of the JSON path expression to a slice as well as the literal
							// byte slice to another array that we will evaluate after this
							jsonPathIndices = append(jsonPathIndices, jsonPathLoc)
							jsonPathLiterals = append(jsonPathLiterals, string(literal))
						}
					}
				}

				// Break out if any errors have occurred
				if err != nil {
					err = utils.JsonPathError.FillError(err.Error())
					break
				}

				// Replace all the occurrences of any JSON path expression within the filter expression with the
				// literal evaluation of each JSON path calculated above
				if len(jsonPathIndices) > 0 {
					filterExp = []byte(utils.ReplaceCharIndexRange(string(filterExp), jsonPathIndices, jsonPathLiterals...))
					// All string literal locations also have to be recalculated as there were changes made to the
					// filter expression
					stringLiteralLocs = stringLiterals.FindAllIndex(filterExp, -1)
					fmt.Println("after replacing JSON path expressions with literal evals:", string(filterExp))
				}

				currentNodeIndices := make([]int, 0)
				// Find all current node indices that lie outside the string literal matches
				for i, char := range string(filterExp) {
					if char == '@' {
						within := false
						for _, stringLiteralLoc := range stringLiteralLocs {
							// If the @ lies inside a string literal then skip it
							if i >= stringLiteralLoc[0] && i <= stringLiteralLoc[1] {
								within = true
								break
							}
						}
						// Only add it if its not within a string lit.
						if !within {
							currentNodeIndices = append(currentNodeIndices, i)
						}
					}
				}

				// Set up the slice which stores all the nodes where the expression evaluates to true as well as the VM
				// to run everything inside
				truers := make([]interface{}, 0)
				vm := otto.New()
				for _, node := range arr {
					currentExpression := make([]byte, len(filterExp))
					copy(currentExpression, filterExp)
					if len(currentNodeIndices) != 0 {
						// Then we want to marshal the current node and replace all occurrences with that unmarshalled literal
						var literal []byte
						literal, err = json.Marshal(node)
						if err != nil {
							break
						}

						// Finally replace the @ characters in the expression with the current node
						currentExpression = []byte(utils.ReplaceCharIndex(string(currentExpression), string(literal), currentNodeIndices...))
					}

					// Evaluate the expression within the VM
					var expressionReturn otto.Value
					// Wrap the execution in an anonymous function so we can handle the halting problem
					func() {
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
										string(filterExp),
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
							time.Sleep(1 * utils.HaltingDelayUnits)
							vm.Interrupt <- func() {
								panic(utils.HaltingProblem)
							}
						}()
						// NOTE: how we wrap the expression in !!() this is to try to convert to boolean
						expressionReturn, err = vm.Run(fmt.Sprintf("!!(%s)", string(currentExpression)))
					}()
					if err != nil {
						break
					}
					// Break out with an error if the returned value is not a boolean
					if !expressionReturn.IsBoolean() {
						err = errors.New("filter does not return boolean")
						break
					}
					truer, _ := expressionReturn.ToBoolean()
					fmt.Println("expression at node", node, "is", string(currentExpression), "=", truer)
					// Otherwise add the node to the truers slice if the returned value is true
					if truer {
						truers = append(truers, node)
					}
				}
				if err != nil {
					err = utils.JsonPathError.FillError(fmt.Sprintf("An error has occurred while evaluating the filter expression \"%s\": %v", string(filterExp), err))
					break
				}
				// Set current value to be the nodes which evaluated to true when filter expression was parsed
				currValue = truers
			case json_map.First:
				err = utils.JsonPathError.FillError("Cannot recurse into an array")
				break
			case json_map.Slice:
				// For slices we'll see if we can do the slice natively in go ([start:end], [:end], [start:])
				// Or for negative slices ([-1:], [:-1]) which are not supported natively so need to be converted
				// Replace StartEnd key types with either the 0th index or the last
				slice := key.Value.([]json_map.AbsolutePathKey)
				sliceIndices := make([]int, 2)
				if slice[0].KeyType == json_map.StartEnd {
					sliceIndices[0] = 0
				} else {
					sliceIndices[0] = slice[0].Value.(int)
				}
				if slice[1].KeyType == json_map.StartEnd {
					sliceIndices[1] = len(arr)
				} else {
					sliceIndices[1] = slice[1].Value.(int)
				}

				// Then we check for negatives
				for i, idx := range sliceIndices {
					if idx < 0 {
						sliceIndices[i] = len(arr) + idx
					}
				}
				fmt.Println("sliceIndices", sliceIndices)

				// Wrap next bit inside anon func so we can catch any panics that occur
				func() {
					defer func() {
						if caught := recover(); caught != nil {
							switch caught.(type) {
							case error:
								// Wrap the error as a JsonPathError if the error has something to do with slices
								if strings.Contains(caught.(error).Error(), "slice") {
									err = utils.JsonPathError.FillError(fmt.Sprintf("Slice error occured: %v", caught))
								} else {
									// Otherwise something more awful has gone wrong
									panic(caught)
								}
							default:
								panic(caught)
							}
						}
					}()
					// Then set the current value to the slice
					// NOTE this might panic so we set up a recovery function above so we can re-wrap any slice errors that occur
					currValue = arr[sliceIndices[0]:sliceIndices[1]]
				}()

				// Break from the loop if an error has occurred
				if err != nil {
					break
				}
			default:
				err = utils.JsonPathError.FillError(fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised", key.KeyType))
				break
			}
		default:
			err = utils.JsonPathError.FillError(fmt.Sprintf("Cannot access key %v of type %s", key, reflect.TypeOf(currValue).Name()))
			break
		}
	}

	if err != nil {
		// Push the error to the error channel if one has occurred
		errChan <- err
	} else {
		// Push the value into the value channel
		valChan <- json_map.JsonPathNode{
			Absolute: path,
			Value:    currValue,
		}
	}
}

// Given the list of absolute paths for a JsonMap, will return the list of values that said paths lead to
// An absolute path is an array of strings, which represent map keys, and integers, which represent array indices.
func (jsonMap *JsonMap) GetAbsolutePaths(absolutePaths *json_map.AbsolutePaths) (values []*json_map.JsonPathNode, errs []error) {
	// Create a wait group which all Finders will be added to
	var wg sync.WaitGroup

	// Both the channels can be buffered to be the length of the array of absolute paths to be evaluated
	// Create a channel of errors which records all the errors that happen within the Finders
	errsChan := make(chan error, len(*absolutePaths))
	// Also create a channel for the return values found by the Finders
	valuesChan := make(chan json_map.JsonPathNode, len(*absolutePaths))

	// Start the finders
	wg.Add(len(*absolutePaths))
	for _, absolutePath := range *absolutePaths {
		go pathFinder(absolutePath, jsonMap.insides, errsChan, valuesChan, &wg)
	}

	// Wait for all Finders and then close the errors channel
	wg.Wait()
	close(errsChan)
	close(valuesChan)

	if len(errsChan) > 0 {
		errs = make([]error, 0)
		// Consume all the errors in the channel and append them to the error return array
		for err := range errsChan {
			errs = append(errs, err)
		}
		return values, errs
	}

	// Fill out the values array by consuming from the values channel
	values = make([]*json_map.JsonPathNode, 0)
	for value := range valuesChan {
		fmt.Println("Appending", value)
		values = append(values, &json_map.JsonPathNode{
			Absolute: value.Absolute,
			Value:    value.Value,
		})
	}
	return values, nil
}

// Given a valid JSON path will return the list of pointers to json_map.JsonPathNode(s) that satisfy the JSON path
//
// This function supports the following JSON path syntax
// - Property selection: .property BUT NOT ['property']
// - Element selection: [n], [x, y, z]
// - First descent: ..property (different to JSON path spec .. descends down the alphabetically first map/array)
// - Wildcards: .property.*, [*]
// - List slicing: [start:end], [start:], [-start:], [:end], [:-end]
// - Filter expressions: [?(expression)]
// - Current node syntax: @
//
// If a filter expression can be evaluated in JS and returns a boolean value then it counts as a valid filter expression
func (jsonMap *JsonMap) JsonPathSelector(jsonPath string) (out []*json_map.JsonPathNode, err error) {
	out = make([]*json_map.JsonPathNode, 0)
	paths, err := utils.ParseJsonPath(jsonPath, jsonMap)
	if err != nil {
		return nil, err
	}
	values, errs := jsonMap.GetAbsolutePaths(&paths)

	// Handle errors
	if errs != nil {
		return nil, utils.JsonPathError.FillFromErrors(errs)
	}
	return values, nil
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
