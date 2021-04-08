package utils

import (
	"bufio"
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Represents a state in the finite state machine used to tokenize JSON paths
type state struct {
	// name used in error messages
	name       string
	// The regular expression that will match the state
	tokenRegex *regexp.Regexp
	// Given the JsonMap, the current absolute paths, the remaining path expression and the token, will validate whether
	// the new path exists in the JsonMap.
	// If error is nil then the path is valid, otherwise the path is not valid
	validator  func(token []byte, togo []byte, jsonMap json_map.JsonMapInt) (absolutePathKeys []json_map.AbsolutePathKey, errs []error)
}

var (
	property = state{
		name:       "Property (property)",
		tokenRegex: regexp.MustCompile("([a-zA-Z_]+([a-zA-Z0-9_-]*)|\\*)"),
		validator:  func(token []byte, togo []byte, jsonMap json_map.JsonMapInt) (absolutePathKeys []json_map.AbsolutePathKey, errs []error) {
			absolutePathKeys = make([]json_map.AbsolutePathKey, 0)
			switch {
			case regexp.MustCompile("\\*").Match(token):
				// If the property is a wildcard then append an AbsolutePathKey of type wildcard to the end of all paths
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
					KeyType: json_map.Wildcard,
					Value:   nil,
				})
			default:
				// Otherwise just append the property as a StringKey
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
					KeyType: json_map.StringKey,
					Value:   string(token),
				})
			}
			return absolutePathKeys, nil
		},
	}
	filter = state{
		name:       "Filter Expression ([?(...)])",
		// We allow anything to be written as a filter expression as it will be passed to otto which will parse the
		// expression and throw up an error if it's incorrect
		tokenRegex: regexp.MustCompile("\\[\\?\\(.*\\)]"),
		validator: func(token []byte, togo []byte, jsonMap json_map.JsonMapInt) (absolutePathKeys []json_map.AbsolutePathKey, errs []error) {
			// We just add the expression body to the absolute path keys
			absolutePathKeys = []json_map.AbsolutePathKey{{
				KeyType: json_map.Filter,
				Value:   string(regexp.MustCompile("(\\[\\?\\(|\\)])").ReplaceAll(token, []byte(""))),
			}}
			return absolutePathKeys, nil
		},
	}
	index = state{
		name:       "Array Index ([n])",
		tokenRegex: regexp.MustCompile("\\[(-?\\d+:?|:?-?\\d+|\\d+:\\d+|\\*|\\d+(,\\s*\\d+)*)]"),
		// The validator for index needs to check if its slice notation [start:end], [start:], [:end], [-start:], [:-end]
		// or if just a normal array index: [n]
		validator:  func(token []byte, togo []byte, jsonMap json_map.JsonMapInt) (absolutePathKeys []json_map.AbsolutePathKey, errs []error) {
			// Function to remove square braces and whitespace then split at the given separator
			stripSplitIndex := func(token []byte, separator string) []string {
				return strings.Split(StripWhitespace(string(regexp.MustCompile("[\\[\\]]").ReplaceAll(token, []byte("")))), separator)
			}

			// Create an array of AbsolutePathKeys which will be added to the AbsolutePaths at the end
			absolutePathKeys = make([]json_map.AbsolutePathKey, 0)
			switch {
			case regexp.MustCompile("\\[\\d+]").Match(token):
				// Normal array index
				n, err := strconv.Atoi(string(regexp.MustCompile("\\d+").Find(token)))
				if err != nil {
					return absolutePathKeys, []error{JsonPathError.FillError(fmt.Sprintf("Could not convert index %s into an integer", string(token)))}
				}
				// Add n to the end of all absolute paths
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{KeyType: json_map.IndexKey, Value: n})
			case regexp.MustCompile("\\[\\d+(,\\s*\\d+)*]").Match(token):
				// Comma separated list of indexes
				// 1. Remove open/close brackets
				// 2. Strip all whitespace
				// 3. Split at each comma
				for _, index := range stripSplitIndex(token, ",") {
					indexInt, err := strconv.Atoi(index)
					if err != nil {
						return absolutePathKeys, []error{JsonPathError.FillError(fmt.Sprintf("Could not convert index %v in token %s into an integer", index, string(token)))}
					}
					absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
						KeyType: json_map.IndexKey,
						Value:   indexInt,
					})
				}
			case regexp.MustCompile("\\[\\*]").Match(token):
				// For wildcards we just add a AbsoluteKeyPath of the Wildcard type
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
					KeyType: json_map.Wildcard,
					Value:   nil,
				})
			case regexp.MustCompile(":").Match(token):
				// Array slices are parsed into an array [start, end]. This is of type []AbsolutePathKey to accommodate
				// empty start and end slices using the StartEnd AbsolutePathKeyType
				slice := make([]json_map.AbsolutePathKey, 0)
				sanitised := stripSplitIndex(token, ":")
				blank := false
				for _, index := range sanitised {
					if index != "" {
						indexInt, err := strconv.Atoi(index)
						if err != nil {
							return absolutePathKeys, []error{JsonPathError.FillError(fmt.Sprintf("Could not convert index %v in token %s into an integer", index, string(token)))}
						}
						slice = append(slice, json_map.AbsolutePathKey{
							KeyType: json_map.IndexKey,
							Value:   indexInt,
						})
					} else {
						// If the blank flag has already been set then throw an error
						// NOTE we don't allow [:] syntax as we already have [*]
						if blank {
							return absolutePathKeys, []error{JsonPathError.FillError("Syntax '[:]' is not supported, use '[*]' instead")}
						}
						// Append a StartEnd token
						slice = append(slice, json_map.AbsolutePathKey{
							KeyType: json_map.StartEnd,
							Value:   nil,
						})
						blank = true
					}
				}
				fmt.Println(slice)
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
					KeyType: json_map.Slice,
					Value:   slice,
				})
			}
			return absolutePathKeys, nil
		},
	}
	dot = state{
		name:           "Dot (.)",
		tokenRegex:     regexp.MustCompile("\\."),
		// The validator for a dot needs to check if it is a recursive descent (the next token is a dot) if it is then
		// append zero to all absolute paths and test whether the
		validator: func(token []byte, togo []byte, jsonMap json_map.JsonMapInt) (absolutePathKeys []json_map.AbsolutePathKey, errs []error) {
			absolutePathKeys = make([]json_map.AbsolutePathKey, 0)
			// Check if recursive descent, aka. the next token is a dot
			if togo[1] == '.' {
				// Add the First Key type to all paths
				absolutePathKeys = append(absolutePathKeys, json_map.AbsolutePathKey{
					KeyType: json_map.First,
					Value:   nil,
				})
			}
			return absolutePathKeys, nil
		},
	}
	root = state{
		name:           "Root node ($)",
		tokenRegex:     regexp.MustCompile("\\$"),
		// The root does not need a validator as it is the first state
		validator:      nil,
	}
	// A dummy state used for error messages
	start = state{
		name:       "Start",
		tokenRegex: nil,
		validator:  nil,
	}
	end = state{
		name:       "End",
		tokenRegex: nil,
		validator:  nil,
	}
)

// A table of each state name to all the states that can precede the state of that name
var fromStateToStates = map[string][]*state {
	"Start": {},
	"Root node ($)": {&dot, &index, &filter},
	"Dot (.)": {&dot, &index, &filter, &property},
	"Array Index ([n])": {&index, &dot, &filter},
	"Property (property)": {&dot, &index, &filter},
	"Filter Expression ([?(...)])": {&dot, &index, &filter},
}

// Will decide the next state given a list of possible states and call the validator for that next state
func (s *state) handler(togo []byte, jsonMap json_map.JsonMapInt, absolutePaths *json_map.AbsolutePaths) (next *state, err error) {
	fmt.Println("togo:", string(togo), "togo len:", len(togo))
	next = nil
	var token []byte

	if len(strings.TrimSpace(string(togo))) != 0 {
		possibleStates := fromStateToStates[s.name]
		fmt.Println("posssible states:", possibleStates)

		// Find which state is next by checking the regex of each possible state on the characters to go
		for _, possibleState := range possibleStates {
			// If possible state's regex matches the start of the characters to go then that is the next state
			if occurrences := possibleState.tokenRegex.FindIndex(togo); len(occurrences) != 0 && occurrences[0] == 0 {
				next = possibleState
				token = togo[occurrences[0]:occurrences[1]]
				fmt.Println("found state", next, "occurrence:", string(togo[occurrences[0]:occurrences[1]]))
				break
			}
		}
		if next == nil {
			possibleStateNames := make([]string, 0)
			for _, possibleState := range possibleStates {
				possibleStateNames = append(possibleStateNames, possibleState.name)
			}
			return nil, JsonPathError.FillError(fmt.Sprintf("Could not find any of the possible states: %v, when at a %s: '%s'", possibleStateNames, s.name, token))
		}
		fmt.Println("next state:", next)

		// Run the validator for the next state
		nextPaths, errs := next.validator(token, togo, jsonMap)
		if errs != nil {
			return nil, JsonPathError.FillFromErrors(errs)
		}
		fmt.Println("nextPaths:", nextPaths)

		// Add the nextPath variable to the end of all absolute paths
		errs = absolutePaths.AddToAll(jsonMap, nextPaths...)
		if errs != nil {
			return nil, JsonPathError.FillFromErrors(errs)
		}
	} else {
		next = &end
	}

	return next, nil
}

// Given a JSON path and a json_map.JsonMapInt will return the list of absolute paths to each value found by that JSON path
//
// If the following path is given: $..property[*].name
// Then following absolute paths could look like this...
// {
//		{0, "property", 0, "name"},
//		{0, "property", 1, "name"},
//		{0, "property", 2, "name"},
// }
func ParseJsonPath(jsonPath string, jsonMap json_map.JsonMapInt) (absolutePaths json_map.AbsolutePaths, err error) {
	// The collection of absolute paths to the values represented by the JSON path
	absolutePaths = make(json_map.AbsolutePaths, 0)
	previousState := &start
	currentState := &root
	jsonPathReader := bufio.NewReader(strings.NewReader(jsonPath))

	for _, state := range []*state{&root, &dot, &index, &filter, &property} {
		state.tokenRegex.Longest()
	}

	for {
		// Break out if at finish state
		if currentState.name == "End" {
			break
		}
		fmt.Println("\ncurrentState", currentState, "previousState", previousState)

		// Peak all the bytes until the end of the buffer
		next, _ := jsonPathReader.Peek(jsonPathReader.Size())
		// Always try to find the longest
		occurrences := currentState.tokenRegex.FindIndex(next)
		// If there are no occurrences found or the occurrence doesn't start at the beginning of the buffer then error out
		if len(occurrences) == 0 || occurrences[0] != 0 {
			err = JsonPathError.FillError(fmt.Sprintf("%s token does not come after %s", currentState.name, previousState.name))
			break
		}

		// Consume len(token) number of bytes
		_, err = jsonPathReader.Discard(occurrences[1] - occurrences[0])
		// If we have reached the end of the JSON path then break and return
		if err == io.EOF {
			break
		}
		next, _ = jsonPathReader.Peek(jsonPathReader.Size())
		// Call the handler function for the current state which will return the next state
		// Also set the current state to be the previous state
		previousState = currentState
		currentState, err = currentState.handler(next, jsonMap, &absolutePaths)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println(absolutePaths)
	return absolutePaths, nil
}
