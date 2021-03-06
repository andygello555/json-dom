// Implementation of json_map.JsonMapInt along with some other helper functions which relate to the JOM.
//
// JsonMap should not be used directly, json_map.JsonMapInt should be used instead for variable declaration, then jom.New,
// Unmarshal, etc. for creating a JsonMap.
package jom

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/andygello555/gotils/concurrency"
	"github.com/andygello555/gotils/ints"
	"github.com/andygello555/gotils/maps"
	"github.com/andygello555/gotils/slices"
	str "github.com/andygello555/gotils/strings"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/globals"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/hjson/hjson-go"
	"github.com/robertkrimen/otto"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Holds some info about the current traversal of the JOM.
//
// Used when evaluating a JsonMap.
type Traversal struct {
	scopePath *strings.Builder
	script    map[string]interface{}
	nonScript map[string]interface{}
}

// Creates a new Traversal object (used within JsonMap).
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

// Wrapper for map[string]interface{} that can be easily extensible with more functionality.
type JsonMap struct {
	// The inner workings, aka. a map.
	insides   map[string]interface{}
	// Used for certain traversal logic
	traversal *Traversal
	// Whether or not the JSON has an Array at its root
	Array bool
}

// Construct a new empty JsonMap.
//
// Returns a pointer to a JsonMap.
func New() *JsonMap {
	return &JsonMap{
		insides:   make(map[string]interface{}),
		traversal: newTraversal(),
		Array:     false,
	}
}

// Constructs a new JsonMap from the given string->interface{} map.
//
// Returns a pointer to a JsonMap.
func NewFromMap(jsonMap map[string]interface{}) *JsonMap {
	return &JsonMap{
		insides:   jsonMap,
		traversal: newTraversal(),
		Array:     false,
	}
}

// Return a clone of the JsonMap. If clear is given then New will be called but "Array" field will be inherited.
//
// Note: This is primarily used when using json_map.JsonMapInt to return a new JsonMap to avoid cyclic imports.
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
//
// This just uses the string builder within the traversal field.
func (jsonMap *JsonMap) GetCurrentScopePath() string {
	return jsonMap.traversal.scopePath.String()
}

// Getter for insides.
// Useful when using json_map.JsonMapInt
func (jsonMap *JsonMap) GetInsides() *map[string]interface{} {
	return &jsonMap.insides
}

// Evaluates the given JSON path filter expression on the given obj on the given json map. Returns a list of values of
// all nodes which are satisfied by the given filter expression. Filter expressions are evaluated using the otto JS
// interpreter so (pretty much) any valid javascript can be written within them as long as they return a boolean value.
// If the returnIndices flag is true then the function will return the slice of indices (string/int) where the true
// values (as decided by the filter exp) occur.
func filterRunner(obj interface{}, filterExp []byte, jsonMap map[string]interface{}, mapType bool, returnIndices bool) (truers interface{}, err error) {
	// For Filters we first have to replace all all @ chars with the current node that has been Marshalled
	// into JSON then JSON.parse-d. And we have to also replace all the JSON paths with calculated literals
	stringLiterals := regexp.MustCompile("['\"]([^\\\\\"']|\\\\.)*['\"]")
	jsonPathRegex := regexp.MustCompile("\\$[.\\[][^'\"\\n\\r\\s]+")

	// Get the locations of the start and end of all string literals
	stringLiteralLocs := stringLiterals.FindAllIndex(filterExp, -1)
	//fmt.Println("string literal locations:", stringLiteralLocs)

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
				//fmt.Println("evaluating JSON path:", string(filterExp)[jsonPathLoc[0]:jsonPathLoc[1]])
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

	// Return out if any errors have occurred
	if err != nil {
		return truers, globals.JsonPathError.FillError(err.Error())
	}

	// Replace all the occurrences of any JSON path expression within the filter expression with the
	// literal evaluation of each JSON path calculated above
	if len(jsonPathIndices) > 0 {
		filterExp = []byte(str.ReplaceCharIndexRange(string(filterExp), jsonPathIndices, jsonPathLiterals...))
		// All string literal locations also have to be recalculated as there were changes made to the
		// filter expression
		stringLiteralLocs = stringLiterals.FindAllIndex(filterExp, -1)
		//fmt.Println("after replacing JSON path expressions with literal evals:", string(filterExp))
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
			// Only add it if its not within a string literal
			if !within {
				currentNodeIndices = append(currentNodeIndices, i)
			}
		}
	}

	// Set up the truers slice to store all truthy values within the map/arr and setup the VM to run everything inside
	if !returnIndices {
		truers = make([]interface{}, 0)
	} else {
		// If we are returning indices then we will create the array according to whether we are going to return string
		// keys or numerical indices
		if mapType {
			truers = make([]string, 0)
		} else {
			truers = make([]int, 0)
		}
	}
	vm := otto.New()

	// Setup up an anonymous function which will make up our for loop body which iterates over our obj
	loopBody := func(nodeIdx interface{}, node interface{}) (err error) {
		// The current expression with all the @s replaced with the literal of the current node
		currentExpression := string(filterExp)
		if len(currentNodeIndices) != 0 {
			// Then we want to marshal the current node and replace all occurrences with that unmarshalled literal
			var literal []byte
			literal, err = json.Marshal(node)
			if err != nil {
				return err
			}
			// Here we are just passing the literal string into the vm and parsing it to a JS value using JSON.parse
			// Then setting the currentNode variable to be that value
			err = vm.Set(globals.CurrentNodeLiteralVarName, string(literal))
			if err != nil {
				return err
			}
			var currentNodeValue otto.Value
			currentNodeValue, err = vm.Run(fmt.Sprintf("JSON.parse(%s)", globals.CurrentNodeLiteralVarName))
			if err != nil {
				return err
			}
			err = vm.Set(globals.CurrentNodeValueVarName, currentNodeValue)
			if err != nil {
				return err
			}

			// Finally replace all @s with the variable name "currentNode"
			currentExpression = str.ReplaceCharIndex(currentExpression, currentNodeIndices, globals.CurrentNodeValueVarName)
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
					if caught == globals.HaltingProblem {
						err = globals.HaltingProblem.FillError(
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
				time.Sleep(1 * globals.HaltingDelayUnits)
				vm.Interrupt <- func() {
					panic(globals.HaltingProblem)
				}
			}()
			// NOTE: how we wrap the expression in !!() this is to try to convert to boolean
			currentExpression = fmt.Sprintf("!!(%s)", currentExpression)
			expressionReturn, err = vm.Run(currentExpression)
		}()
		if err != nil {
			return err
		}
		// Break out with an error if the returned value is not a boolean
		if !expressionReturn.IsBoolean() {
			err = errors.New("filter does not return boolean")
			return err
		}
		truer, _ := expressionReturn.ToBoolean()
		//fmt.Println("expression at node", node, "is", currentExpression, "=", truer)
		// Otherwise add the node to the truers slice if the returned value is true
		if truer {
			switch truers.(type) {
			case []interface{}:
				truers = append(truers.([]interface{}), node)
			case []string:
				truers = append(truers.([]string), nodeIdx.(string))
			case []int:
				truers = append(truers.([]int), nodeIdx.(int))
			}
		}
		return nil
	}

	// Start our for loop depending on whether our obj is a map[string]interface{} or a []interface{}
	//fmt.Println("\nRunning filter: \"", string(filterExp), "\" on", obj)
	if mapType {
		for k, node := range obj.(map[string]interface{}) {
			err = loopBody(k, node)
			if err != nil {
				break
			}
		}
	} else {
		for i, node := range obj.([]interface{}) {
			err = loopBody(i, node)
			if err != nil {
				break
			}
		}
	}

	if err != nil {
		return nil, globals.JsonPathError.FillError(fmt.Sprintf("An error has occurred while evaluating the filter expression \"%s\": %v", string(filterExp), err))
	}

	return truers, nil
}

// Run by GetAbsolutePaths in parallel for each absolute path in an json_map.AbsolutePaths array to find the requested values.
//
// Each found value is pushed to valChan and each error is pushed to errChan. Once it has complete it calls Done on the wait group.
func pathFinder(path []json_map.AbsolutePathKey, jsonMap map[string]interface{}, errChan chan<- error, valChan chan<- json_map.JsonPathNode, wg *sync.WaitGroup) {
	defer wg.Done()
	var currValue interface{} = jsonMap
	var err error = nil

	// Temp helper function for recursive lookups
	recursiveLookup := func(key json_map.AbsolutePathKey, arrOrMap interface{}) []interface{} {
		// We'll have to spin up additional finders for every key within this map
		// Create a wait group which all Sub-Finders will be added to
		var subWg sync.WaitGroup
		// Create an in and out channel using the InOut data structure
		inFound, outFound := concurrency.InOut()
		toFind := key.Value.(string)
		foundValues := make([]interface{}, 0)

		// Set up a temp function for the RecursiveLookup finders
		var subFinder func(subtree interface{}, subWg *sync.WaitGroup, toFind string, foundlings chan<- interface{})
		subFinder = func(subtree interface{}, subWg *sync.WaitGroup, toFind string, foundlings chan<- interface{}) {
			// Only defer done when a wait group is given
			if subWg != nil {
				defer subWg.Done()
			}

			switch subtree.(type) {
			case map[string]interface{}:
				subM := subtree.(map[string]interface{})
				for subSubKey, subSubtree := range subM {
					// Recurse into all the keys within the map checking if the key of the current subtree is equal to
					// the key we are meant to be finding
					if subSubKey == toFind {
						// If so we add the subtree to the values channel
						foundlings <- subSubtree
					}
					// ... we still traverse in order to explore everything
					subFinder(subSubtree, nil, toFind, foundlings)
				}
			case []interface{}:
				// Since an array doesn't have any keys to search for we will just recurse down
				for _, subSubtree := range subtree.([]interface{}) {
					subFinder(subSubtree, nil, toFind, foundlings)
				}
			default:
				// Base case so we'll break and return
				break
			}
			return
		}

		// Start the sub-finders for each sub-tree of depth one
		// We do a type switch here to work out whether we are iterating over a map or on array
		switch arrOrMap.(type) {
		case map[string]interface{}:
			m := arrOrMap.(map[string]interface{})
			subWg.Add(len(m))
			for _, value := range m {
				go subFinder(value, &subWg, toFind, inFound)
			}
		case []interface{}:
			arr := arrOrMap.([]interface{})
			subWg.Add(len(arr))
			for _, value := range arr {
				go subFinder(value, &subWg, toFind, inFound)
			}
		}

		// Wait for all Finders and then close the input channel
		subWg.Wait()
		close(inFound)

		// Finally we read all the values from the out channel and append them to the foundValues array
		for v := range outFound {
			// If the value added was an array then we will "unwrap" it
			switch v.(type) {
			case []interface{}:
				for _, av := range v.([]interface{}) {
					foundValues = append(foundValues, av)
				}
			default:
				foundValues = append(foundValues, v)
			}
		}
		return foundValues
	}


	// Iterate through the absolute path keys
	for _, key := range path {
		// StartEnd KeyTypes must be within a Slice key type so throw an error if so
		if key.KeyType == json_map.StartEnd {
			err = globals.JsonPathError.FillError("Cannot have a start/end key type outside a slice key type")
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
					err = globals.JsonPathError.FillError(fmt.Sprintf("Key '%v' does not exist in map", key.Value))
					break
				}
			case json_map.IndexKey | json_map.Slice:
				err = globals.JsonPathError.FillError(fmt.Sprintf("Cannot access map %v with numerical key %v", currValue, key.Value))
				break
			case json_map.Wildcard:
				// For wildcards return all the values of each key within the map
				// To ensure that the same key isn't pulled twice when evaluating paths like this:
				// {"person", "friends", 0, *, 0}
				// {"person", "friends", 0, *, 1}
				// We have to first push all the keys to a heap and then pop them off so that we have the same order
				// each time we evaluate this map
				keyQueue := make(str.StringHeap, 0)
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
			case json_map.Filter:
				// Using the filterRunner function we can run the filter on the values of each key in the map
				filterExp := []byte(key.Value.(string))
				currValue, err = filterRunner(m, filterExp, jsonMap, true, false)
				if err != nil {
					break
				}
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
					err = globals.JsonPathError.FillError(fmt.Sprintf("There are no maps to recurse down in %v", m))
					break
				}
				// Otherwise sort the strings and take the value of the first key as the new current value
				sort.Strings(keys)
				currValue = m[keys[0]]
			case json_map.RecursiveLookup:
				// We set the current value to be all found values
				currValue = recursiveLookup(key, m)
				break
			default:
				err = globals.JsonPathError.FillError(fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised for type \"%s\"", key.KeyType, str.TypeName(currValue)))
				break
			}
		case []interface{}:
			arr := currValue.([]interface{})
			switch key.KeyType {
			case json_map.StringKey:
				// When given a string key we will iterate over all elements seeing if we have a map which we can test
				// if it contains the required StringKey
				newArr := make([]interface{}, 0)
				for _, item := range arr {
					switch item.(type) {
					case map[string]interface{}:
						if match, ok := item.(map[string]interface{})[key.Value.(string)]; ok {
							newArr = append(newArr, match)
						}
					default:
						continue
					}
				}
				currValue = newArr
			case json_map.IndexKey:
				i := key.Value.(int)
				if i >= len(arr) || i < 0 {
					err = globals.JsonPathError.FillError(fmt.Sprintf("Index (%d) is out of bounds for array of length %d", i, len(arr)))
					break
				}
				//fmt.Println("Getting index:", i, "from", arr, "=", arr[i])
				currValue = arr[i]
			case json_map.Wildcard:
				// If a wildcard then just set the current value to be equal to the array
				currValue = arr
			case json_map.Filter:
				// Using the filterRunner function we can run the filter on the elements of the array
				filterExp := []byte(key.Value.(string))
				currValue, err = filterRunner(arr, filterExp, jsonMap, false, false)
				if err != nil {
					break
				}
			case json_map.First:
				err = globals.JsonPathError.FillError("Cannot recurse into an array")
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
				//fmt.Println("sliceIndices", sliceIndices)

				// Wrap next bit inside anon func so we can catch any panics that occur
				func() {
					defer func() {
						if caught := recover(); caught != nil {
							switch caught.(type) {
							case error:
								// Wrap the error as a JsonPathError if the error has something to do with slices
								if strings.Contains(caught.(error).Error(), "slice") {
									err = globals.JsonPathError.FillError(fmt.Sprintf("Slice error occured: %v", caught))
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
			case json_map.RecursiveLookup:
				// We set the current value to be all found values from the recursive lookup helper
				currValue = recursiveLookup(key, arr)
				break
			default:
				err = globals.JsonPathError.FillError(fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised for type \"%s\"", key.KeyType, str.TypeName(currValue)))
				break
			}
		default:
			err = globals.JsonPathError.FillError(fmt.Sprintf("Cannot access key %v of type \"%s\"", key, str.TypeName(currValue)))
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

// Given the list of absolute paths for a JsonMap, will return the list of values that said paths lead to.
//
// An absolute path is an array of json_map.AbsolutePathKey(s), each of which represent a descent down the JsonMap.
// Will start a goroutine for each absolute path slice in the given json_map.AbsolutePaths struct meaning that lookup
// is pretty fast.
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
		//fmt.Println("Appending", value)
		switch value.Value.(type) {
		case []interface{}:
			// We unwrap any arrays returned from the finders
			for i, v := range value.Value.([]interface{}) {
				// We have to amend the absolute paths for the nodes by adding on an IndexKey at each iteration
				amendedAbsolute := make([]json_map.AbsolutePathKey, len(value.Absolute) + 1)
				copy(amendedAbsolute, value.Absolute)
				amendedAbsolute = append(amendedAbsolute, json_map.AbsolutePathKey{
					KeyType: json_map.IndexKey,
					Value:   i,
				})
				values = append(values, &json_map.JsonPathNode{
					Absolute: amendedAbsolute,
					Value:    v,
				})
			}
		default:
			// Otherwise we just append normally
			values = append(values, &json_map.JsonPathNode{
				Absolute: value.Absolute,
				Value:    value.Value,
			})
		}
	}
	return values, nil
}

// Given the list of absolute paths for a JsonMap: will set the values pointed to by the given JSON path to be the
// given value.
//
// If a value of nil is given the structures pointed to by the absolute paths will be deleted.
// To avoid race conditions this routine runs single threaded which means this operation can be significantly slower
// than getting values. It's important to bear this in mind.
func (jsonMap *JsonMap) SetAbsolutePaths(absolutePaths *json_map.AbsolutePaths, value interface{}) (err error) {
	// Create a type for errors which will be used in discerning caught panics later on
	type recursionError struct {
		Message string
	}

	// Set up the recursive function which will be run on all absolute paths
	var recursiveTraversal func(remainingPath []json_map.AbsolutePathKey, currTree interface{}) interface{}
	recursiveTraversal = func(remainingPath []json_map.AbsolutePathKey, currTree interface{}) interface{} {
		if len(remainingPath) > 0 {
			// Pop the next path key
			var key json_map.AbsolutePathKey
			//fmt.Print("value:", value, ", remaining:", remainingPath)
			key, remainingPath = remainingPath[0], remainingPath[1:]
			//fmt.Print(", up next:", key)

			// Some precomputed flags for readability
			lastKey := len(remainingPath) == 0   // Whether we are on the last key in the path and should set the value
			deleteVal := value == nil && lastKey // Whether we are on the last key AND value is nil so we should delete
			//fmt.Println(" lastKey, deleteVal =", lastKey, deleteVal)

			// A simple setter function which returns the value needed for the given index depending on whether we are
			// on the last key or not
			setter := func(obj interface{}, index interface{}) (newValue interface{}) {
				if !lastKey {
					switch obj.(type) {
					case map[string]interface{}:
						newValue = recursiveTraversal(remainingPath, obj.(map[string]interface{})[index.(string)])
					case []interface{}:
						newValue = recursiveTraversal(remainingPath, obj.([]interface{})[index.(int)])
					default:
						// Otherwise we cannot continue down the tree any further
						panic(recursionError{fmt.Sprintf("Cannot recurse down subtree of type \"%s\"", str.TypeName(obj))})
					}
					return newValue
				}
				return value
			}

			// Specific setters for maps and arrays. Takes a reference to a map/array and modifies in place
			setterMap := func(mRef *map[string]interface{}, key string) {
				if deleteVal {
					// Delete the key using the delete function
					delete(*mRef, key)
				} else {
					(*mRef)[key] = setter(*mRef, key)
				}
			}
			// Takes an array of indices so that multiple indices can be deleted at once so that indices aren't messed
			// up between deletions
			setterArr := func(arrRef *[]interface{}, indices... int) {
				// If the length of indices is 0 then we assume that the caller wants all the indices
				if len(indices) == 0 {
					indices = ints.Range(0, len(*arrRef) - 1, 1)
				}

				if deleteVal {
					// Delete indices using the RemoveElems from gotils
					*arrRef = slices.RemoveElems(*arrRef, indices...)
				} else {
					// We have to iterate through all indices and set the according values
					for _, idx := range indices {
						(*arrRef)[idx] = setter(*arrRef, idx)
					}
				}
			}

			// Temp helper function for recursive lookups
			recursiveLookup := func(key json_map.AbsolutePathKey, arrOrMap interface{}) (newTree interface{}) {
				// Extract the key to find from the AbsolutePathKey
				toFind := key.Value.(string)

				// Set up a temp function for the RecursiveLookup finders
				var subFinder func(subtree interface{}, toFind string) (newSubtree interface{})
				subFinder = func(subtree interface{}, toFind string) (newSubtree interface{}) {
					switch subtree.(type) {
					case map[string]interface{}:
						subM := subtree.(map[string]interface{})
						// Create a copy of the map to store the updated subtree in
						newSubtreeM := maps.CopyMap(subM)

						// Recurse into all the other keys within the map and set the new subtrees returned in the
						// clone of the current subtree (newSubtreeM)
						for subSubKey, subSubtree := range subM {
							// If the current key is equal to the toFind key we continue the recursiveTraversal
							// subroutine from here
							if subSubKey == toFind {
								// Then we can use the setterMap function to set, delete from or recurse down the map
								// This is so that if we still have path remaining we will continue the search at this subtree
								setterMap(&newSubtreeM, toFind)
								//fmt.Println("found:", toFind, "within:", subM, "updated subtree to:", newSubtreeM)
							} else {
								// Otherwise we continue our search for the toFind key
								newSubtreeM[subSubKey] = subFinder(subSubtree, toFind)
							}
						}
						// Set the new subtree return value
						newSubtree = newSubtreeM
					case []interface{}:
						subArr := subtree.([]interface{})
						// Allocate memory for a new array of the same size
						newSubtreeArr := make([]interface{}, len(subArr))
						// Since an array doesn't have any keys to search for we will just recurse down
						for subSubIdx, subSubtree := range subArr {
							newSubtreeArr[subSubIdx] = subFinder(subSubtree, toFind)
							//fmt.Println("recursing down", subArr[subSubIdx], "in", subArr, "setting to", newSubtreeArr[subSubIdx])
						}
						// Set the new subtree return value
						newSubtree = newSubtreeArr
					default:
						// Base case so we just return the subtree without recursing down it
						newSubtree = subtree
					}
					return newSubtree
				}

				// RECURSE DOWN EACH SUBTREE IN THE MAP/ARRAY
				// We do a type switch here to work out whether we are iterating over a map or on array
				switch arrOrMap.(type) {
				case map[string]interface{}:
					m := arrOrMap.(map[string]interface{})
					mCopy := maps.CopyMap(m)
					for k, v := range m {
						mCopy[k] = subFinder(v, toFind)
					}
					newTree = mCopy
				case []interface{}:
					arr := arrOrMap.([]interface{})
					arrCopy := make([]interface{}, len(arr))
					for i, v := range arr {
						arrCopy[i] = subFinder(v, toFind)
					}
					newTree = arrCopy
				}
				return newTree
			}

			// StartEnd KeyTypes must be within a Slice key type so throw an error if so
			// NOTE all errors will panic as it makes dealing with them easier due to the recursive nature of the lookup
			if key.KeyType == json_map.StartEnd {
				panic(recursionError{"cannot have a start/end key type outside a slice key type"})
			}

			// Check the type of the current value and take the according value
			switch currTree.(type) {
			case map[string]interface{}:
				// A "frozen" copy of the current value as a map which will not be modified
				m := currTree.(map[string]interface{})
				switch key.KeyType {
				case json_map.StringKey:
					if _, ok := m[key.Value.(string)]; !ok && !lastKey {
						// If the key does not exist and we are not on the last key in the path then we cannot continue so we throw an error
						panic(recursionError{fmt.Sprintf("Key '%v' does not exist in map", key.Value)})
					}
					setterMap(&m, key.Value.(string))
				case json_map.IndexKey | json_map.Slice:
					panic(recursionError{fmt.Sprintf("Cannot access map %v with numerical key %v", currTree, key.Value)})
				case json_map.Wildcard:
					// We always iterate through a copy of the map as we might delete a key-value pair from the original map
					for k := range currTree.(map[string]interface{}) {
						setterMap(&m, k)
					}
				case json_map.Filter:
					var newSubtreeIndices interface{}
					// Using the filterRunner function we can run the filter on the values of each key in the map
					filterExp := []byte(key.Value.(string))
					newSubtreeIndices, err = filterRunner(m, filterExp, jsonMap.insides, true, true)
					if err != nil {
						panic(recursionError{err.Error()})
					}
					// Then we iterate through all truthy indices and recurse down their values
					for _, k := range newSubtreeIndices.([]string) {
						setterMap(&m, k)
					}
				case json_map.First:
					// We sort the keys alphabetically then set the value of the first
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
						panic(recursionError{fmt.Sprintf("There are no maps to recurse down in %v", m)})
					}
					// Otherwise sort the strings and take the value of the first key as the new current value
					sort.Strings(keys)
					setterMap(&m, keys[0])
				case json_map.RecursiveLookup:
					// NOTE: The RecursiveLookup case is a special scenario where the traversal is continued within the
					// recursive lookup function. This means after the function returns we can empty the path queue so
					// we stop in the recursiveLookup function
					m = recursiveLookup(key, m).(map[string]interface{})
					remainingPath = []json_map.AbsolutePathKey{}
				default:
					panic(recursionError{fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised for type \"%s\"", key.KeyType, str.TypeName(currTree))})
				}
				// Finally set the current tree to the copy of the value as a map
				currTree = m
			case []interface{}:
				arr := currTree.([]interface{})
				switch key.KeyType {
				case json_map.StringKey:
					// When given a string key we will iterate over all elements seeing if we have a map which we can test
					// if it contains the required StringKey
					for i, item := range currTree.([]interface{}) {
						switch item.(type) {
						case map[string]interface{}:
							mapItem := item.(map[string]interface{})
							if _, ok := mapItem[key.Value.(string)]; ok {
								setterMap(&mapItem, key.Value.(string))
								arr[i] = mapItem
							}
						default:
							continue
						}
					}
				case json_map.IndexKey:
					i := key.Value.(int)
					if i >= len(arr) || i < 0 {
						panic(recursionError{fmt.Sprintf("Index (%d) is out of bounds for array of length %d", i, len(arr))})
					}
					//fmt.Println("Getting index:", i, "from", arr, "=", arr[i])
					setterArr(&arr, i)
				case json_map.Wildcard:
					// If a wildcard then we need to iterate over array and recurse down each element
					setterArr(&arr)
				case json_map.Filter:
					var newSubtreeIndices interface{}
					// Using the filterRunner function we can run the filter on the elements of the array
					filterExp := []byte(key.Value.(string))
					newSubtreeIndices, err = filterRunner(arr, filterExp, jsonMap.insides, false, true)
					if err != nil {
						panic(recursionError{err.Error()})
					}
					// Then we iterate through all truthy indices and recurse down their values only if filter actually
					// returned anything
					if len(newSubtreeIndices.([]int)) > 0 {
						setterArr(&arr, newSubtreeIndices.([]int)...)
					}
				case json_map.First:
					panic(recursionError{"cannot recurse into an array"})
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
					//fmt.Println("sliceIndices", sliceIndices)

					if sliceIndices[0] < 0 || sliceIndices[1] > len(arr) {
						panic(recursionError{fmt.Sprintf("Slice: [%v:%v] (translated to %v), is out of range", slice[0], slice[1], sliceIndices)})
					}

					// Panic if an error has occurred
					if err != nil {
						panic(recursionError{err.Error()})
					}

					// Then we iterate through a range of slice indices
					setterArr(&arr, ints.Range(sliceIndices[0], sliceIndices[1] - 1, 1)...)
				case json_map.RecursiveLookup:
					// NOTE: The RecursiveLookup case is a special scenario where the traversal is continued within the
					// recursive lookup function. This means after the function returns we can empty the path queue so
					// we stop in the recursiveLookup function
					arr = recursiveLookup(key, arr).([]interface{})
					remainingPath = []json_map.AbsolutePathKey{}
				default:
					panic(recursionError{fmt.Sprintf("AbsolutePathKey of type: %v is unrecognised for type \"%s\"", key.KeyType, str.TypeName(currTree))})
				}
				// Current tree set to the modified array
				currTree = arr
			default:
				panic(recursionError{fmt.Sprintf("Cannot access key %v within type \"%s\"", key, str.TypeName(currTree))})
			}
		}
		return currTree
	}

	// Iterate through all paths and start the recursiveTraversal function for each
	err = nil
	for _, path := range *absolutePaths {
		func() {
			// PANIC HANDLING
			defer func() {
				// Handle any errors that occur within the recursive traversal
				if caught := recover(); caught != nil {
					// If the caught error is of type recursionError (defined above) then we want to wrap into an error
					switch caught.(type) {
					case recursionError:
						// Here we fill out the JsonPathError from the recursionError
						err = globals.JsonPathError.FillError(caught.(recursionError).Message)
					default:
						// Another error that we should panic for
						panic(caught)
					}
					return
				}
			}()

			// Run the recursive traversal function for the current path
			newInsides := recursiveTraversal(path, jsonMap.insides)
			// Depending what type was returned from the function take the appropriate action
			switch newInsides.(type) {
			case map[string]interface{}:
				jsonMap.insides = newInsides.(map[string]interface{})
			case []interface{}:
				// FIXME: The root array hack
				jsonMap.insides["array"] = newInsides.([]interface{})
				jsonMap.Array = true
			default:
				err = errors.New(fmt.Sprintf("a JSON object cannot have a \"%s\" at its root", str.TypeName(newInsides)))
			}
		}()

		// Break out the loop if an error has occurred
		if err != nil {
			break
		}
	}

	return err
}

// Given a valid JSON path will return the list of pointers to json_map.JsonPathNode(s) that satisfies the JSON path.
//
// A wrapper for json_map.ParseJsonPath and GetAbsolutePaths.
// Note: The JSON path syntax is very particular. If a JSON path is not being parsed make sure it looks like the below examples.
//
// This function supports the following JSON path syntax:
//
// Property selection
//
// Selects a property from a map.
//  .property
//  // BUT NOT
//  ['property']
//
// Element selection
//
// Selects an element from an array. If a comma separated list is given this will give a new list of the elements at the
// given indices.
//  [n]
//  // OR
//  [x, y, z]
//
// First descent
//
// "...+" descends down the alphabetically first map/array. Any dots following "..." will indicate another first ascent.
//  ...property
// For example:
//  .....property
// Will perform first ascent three times.
//
// Recursive lookup
//
// Takes precedence over First descent. Searches for all the properties of the given name.
//  ..property
//
// Wildcards
//
// Selects all values/elements from a map or an array respectively.
//  // Key-values
//  .property.*
//  // Elements
//  property[*]
//
// List slicing
//
// Slice a list from start to finish. Supports one missing side and negative indices. Similar to python list slicing.
//  // From <start> to <end>
//  [start:end]
//  // From <start> to the end of the array
//  [start:]
//  // From the last <start> elements to the end of the array
//  [-start:]
//  // From the start of the array to <end> element
//  [:end]
//  // The entire array excluding the last <end> elements
//  [:-end]
//  // NOT SUPPORTED
//  [:]
//
// Filter expressions
//
// A filter expression will be run against every value within a map and every element in an array. The resulting array
// it will produce will be all the values on which the filter expression evaluated to true. Any JS expression which can
// be processed by the Otto interpreter can be run as a filter expression. Therefore the expression must be a boolean
// expression or one which can be cast to boolean using "!!".
//  [?(expression)]
// The current node can be referred to using the '@' character.
//  $.property[?(@.name)]
// Any JSON paths used within an expression will also be evaluated from the current scope.
//  $.property[?($[0].name == @.name)]
func (jsonMap *JsonMap) JsonPathSelector(jsonPath string) (out []*json_map.JsonPathNode, err error) {
	out = make([]*json_map.JsonPathNode, 0)
	paths, err := json_map.ParseJsonPath(jsonPath)
	if err != nil {
		return nil, err
	}
	values, errs := jsonMap.GetAbsolutePaths(&paths)

	// Handle errors
	if errs != nil {
		return nil, globals.JsonPathError.FillFromErrors(errs)
	}
	return values, nil
}

// Given a valid JSON path: will set the values pointed to by the JSON path to be the value given.
//
// If nil is given as the value then the pointed to elements will be deleted.
// A wrapper for json_map.ParseJsonPath -> SetAbsolutePaths.
func (jsonMap *JsonMap) JsonPathSetter(jsonPath string, value interface{}) (err error) {
	var paths json_map.AbsolutePaths
	paths, err = json_map.ParseJsonPath(jsonPath)
	if err != nil {
		return err
	}
	err = jsonMap.SetAbsolutePaths(&paths, value)
	return err
}

// Adds the given script of the given shebangName (must be a supported language) at the path pointed to by the given
// jsonPath.
//
// This just validates the given shebangName, constructs the script with the appropriate shebang and runs JsonPathSetter
// on the given jsonPath with the constructed script as a value.
func (jsonMap *JsonMap) MarkupCode(jsonPath string, shebangName string, script string) (err error)  {
	// Check if the shebangName is a valid and supported shebangName
	if !code.CheckIfSupported(shebangName) {
		return globals.UnsupportedScriptLang.FillError(shebangName)
	}
	// Construct the script
	constructedScript := fmt.Sprintf("%s%s\n%s", globals.ShebangPrefix, shebangName, script)
	// Run JsonPathSetter
	err = jsonMap.JsonPathSetter(jsonPath, constructedScript)
	return err
}

// Finds all the script and non-script fields within a JsonMap.
//
// Updates the script and nonScript fields within the JsonMap's traversal object. Scripts will be replaced by a code.Code
// value which contains the runnable.
func (jsonMap *JsonMap) FindScriptFields() (found bool) {
	// Indicates whether a script tag has been found at the current depth or a nested depth. Used to indicate when to
	// join a scriptFields subtree to its parent tree.
	found = false

	for key, element := range *(jsonMap.GetInsides()) {
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
		case func(json json_map.JsonMapInt), string:
			// Check if the element contains a script
			if runnable, ok := code.NewFrom(element); ok {
				// If it is then add the key to the script map as a Code object and set found to true
				found = true
				jsonMap.traversal.script[key] = runnable
			} else {
				// Add the field to the nonScriptFields map
				jsonMap.traversal.nonScript[key] = element
			}
		default:
			// Add the field to the nonScriptFields map
			jsonMap.traversal.nonScript[key] = element
		}
		// FIXME: I don't know why this needs to be here but apparently it does otherwise go callback scripts (func(json json_map.JsonMapInt)) at a depth greater than 1 will be deleted?
		(*jsonMap).insides[key] = element
	}

	return found
}

// Strips any script key-value pairs found within the JsonMap and updates it in place.
//
// This is essentially just a wrapper for FindScriptFields which just sets insides to be traversal.nonScript.
func (jsonMap *JsonMap) Strip() {
	if jsonMap.FindScriptFields() {
		jsonMap.insides = jsonMap.traversal.nonScript
	}
}

// Given a JsonMap this will traverse it and execute all scripts. Will update the given JsonMap in place.
//
// ??? All scripts will be run and removed from the JsonMap.
//
// ??? In cases where there are more than one script tag on a level: scripts will be evaluated in lexicographical script-key order.
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
	scriptQueue := make(str.StringHeap, 0)
	for k, e := range jsonMap.traversal.script {
		switch e.(type) {
		case code.Code:
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
		script := jsonMap.traversal.script[scriptKey].(code.Code)

		// Run the script for the script's language. This will...
		// 1. Create the JOM object, setup any builtin functions and insert the JOM into the script environment
		// 2. Setup any interrupts for the halting problem
		// 3. Extract and decode the JOM from the environment and return it
		// Any errors that occur have to be panicked as they can effect the entire runtime
		newScope, err := code.Run(script, jsonMap)
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
//
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
		fmt.Println("\ngo map:", jsonMap.insides)
	}

	// Marshal the output JSON
	out, err = jsonMap.Marshal()
	if err != nil {
		return out, err
	}
	return out, nil
}

// Like JsonPathSelector, only it panics when an error occurs and returns an []interface{} instead of []json_map.JsonPathNode.
//
// The type of out depends on how many results were returned:
//
// ??? If no == 0 then out == nil.
//
// ??? If no > 0 then out == []interface{}.
func (jsonMap *JsonMap) MustGet(jsonPath string) (out []interface{}) {
	selector, err := jsonMap.JsonPathSelector(jsonPath)
	if err != nil {
		panic(err)
	}
	if len(selector) == 0 {
		out = nil
	} else {
		out = make([]interface{}, len(selector))
		for i, v := range selector {
			out[i] = v.Value
		}
	}
	return out
}

// Like JsonPathSetter, only it panics when an error occurs.
func (jsonMap *JsonMap) MustSet(jsonPath string, value interface{}) {
	err := jsonMap.JsonPathSetter(jsonPath, value)
	if err != nil {
		panic(err)
	}
}

// A wrapper for MustSet(jsonPath, nil).
func (jsonMap *JsonMap) MustDelete(jsonPath string) {
	jsonMap.MustSet(jsonPath, nil)
}

// Pushes to an []interface{} indicated by the given JSON path at the given indices and panics if any errors occur.
//
// Note: This assumes that the JSON path points to an array and will use JsonPathSetter to set the JSON path to be the array selected + the pushed values.
//
// Indices conditions:
//
// ??? If no indices are given insert value at the end of the array (len(arr)).
//
// ??? If duplicates occur then they will be ignored/removed.
//
// ??? Can be unsorted (will be sorted within function).
//
// ??? If a given index is greater than the length of the array to insert to, all empty spaces will be filled with nil.
func (jsonMap *JsonMap) MustPush(jsonPath string, value interface{}, indices... int) {
	nodes := jsonMap.MustGet(jsonPath)
	if nodes == nil {
		panic(errors.New(fmt.Sprintf("no node at \"%s\" to push to", jsonPath)))
	}

	// If there are no indices given push to the back of the array
	if len(indices) == 0 {
		indices = append(indices, len(nodes))
	}
	// Use the AddElems utility to add the value at the given indices
	newArr := slices.AddElems(nodes, value, indices...)

	// Finally we set the same path we are given to be the newArr
	jsonMap.MustSet(jsonPath, newArr)
}

// Pops from an []interface{} indicated by the given JSON path at the given indices and panics if any errors occur.
//
// NOTE: this assumes that the JSON path points to an array and will use JsonPathSetter to set the JSON path to be the array selected + the pushed values.
//
// Indices conditions:
//
// ??? If no indices are given delete value at the start of the array.
//
// ??? If duplicates occur then they will be ignored/removed.
//
// ??? Can be unsorted (will be sorted within function).
//
// ??? Indices that are not within the range of the array will be ignored.
func (jsonMap *JsonMap) MustPop(jsonPath string, indices... int) (popped []interface{}) {
	nodes := jsonMap.MustGet(jsonPath)
	if nodes == nil {
		panic(errors.New(fmt.Sprintf("no node at \"%s\" to pop from", jsonPath)))
	}

	// If there are no indices given pop from the head of the array
	if len(indices) == 0 {
		indices = append(indices, 0)
	}

	// We get the popped elements and the new array at the some time
	slices.RemoveDuplicatesAndSort(&indices)
	popped = make([]interface{}, 0)
	newArr := make([]interface{}, 0)

	var currIdx int
	currIdx, indices = indices[0], indices[1:]

	for i, node := range nodes {
		if currIdx == i {
			popped = append(popped, node)
			if len(indices) > 0 {
				currIdx, indices = indices[0], indices[1:]
			}
			continue
		}
		newArr = append(newArr, node)
	}

	// Then we use MustSet to set the new array
	jsonMap.MustSet(jsonPath, newArr)
	return popped
}

// Marshals the JsonMap into hjson and returns the stringified byte array.
func (jsonMap *JsonMap) String() string {
	var err error
	var out []byte
	if !jsonMap.Array {
		out, err = hjson.Marshal(jsonMap.insides)
	} else {
		// FIXME: Handle the root array hack
		out, err = hjson.Marshal(jsonMap.insides["array"])
	}
	if err != nil {
		panic(err)
	}
	return string(out)
}

// Checks whether the JsonMap is an array at its root.
func (jsonMap *JsonMap) IsArray() bool {
	return jsonMap.Array
}
