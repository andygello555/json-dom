package utils

import (
	"bufio"
	"fmt"
	"github.com/andygello555/json-dom/jom/json_map"
	"io"
	"regexp"
	"strings"
)

// Represents a state in the finite state machine used to tokenize JSON paths
type state struct {
	// name used in error messages
	name       string
	// The regular expression that will match the state
	tokenRegex *regexp.Regexp
	// The function that should be called which returns the next state
	handler    func(token []byte, togo []byte, s *state, jsonMap json_map.JsonMapInt, absolutePaths *[][]interface{}) (next *state, err error)
	// Given the JsonMap, the current absolute paths, the remaining path expression and the token, will validate whether
	// the new path exists in the JsonMap.
	// If error is nil then the path is valid, otherwise the path is not valid
	validator  func(token []byte, togo []byte, jsonMap json_map.JsonMapInt, absolutePaths *[][]interface{}) (errs []error)
}

var (
	property = state{
		name:       "Property (property)",
		tokenRegex: regexp.MustCompile("([a-zA-Z_]+([a-zA-Z0-9_]*)|\\*)"),
		handler:    nil,
	}
	filter = state{
		name:       "Filter Expression ([?(...)])",
		// We allow anything to be written as a filter expression as it will be passed to otto which will parse the
		// expression and throw up an error if it's incorrect
		tokenRegex: regexp.MustCompile("\\[\\?\\(.*\\)]"),
		handler:    nil,
	}
	index = state{
		name:       "Array Index ([n])",
		tokenRegex: regexp.MustCompile("\\[(-?\\d+:?|:?-?\\d+|\\d+:\\d+|\\*|\\d+(, \\d+)*)]"),
		handler:    nil,
	}
	dot = state{
		name:       "Dot (.)",
		tokenRegex: regexp.MustCompile("\\."),
		handler: func(token []byte, togo []byte, s *state, jsonMap json_map.JsonMapInt, absolutePaths *[][]interface{}) (next *state, err error) {
			// Root can go to itself (dot), index, filter or a property
			next = nil
			possibleStates := []*state{s, &index, &filter, &property}

			for _, possibleState := range possibleStates {
				// If possible state's regex matches the start of the characters to go then that is the next state
				if occurrences := possibleState.tokenRegex.FindIndex(togo); len(occurrences) != 0 && occurrences[0] == 0 {
					next = possibleState
				}
			}
			if next == nil {
				return nil, JsonPathError.FillError(fmt.Sprintf("Could not find any of the possible states: %v, when at a %s token", possibleStates, s.name))
			}
			// We don't add anything to the absolute paths when handling the root node
			return next, nil
		},
		// The validator for a dot needs to check if it is a recursive descent (the next token is a dot) if it is then
		// append zero to all absolute paths and test whether the
		validator: func(token []byte, togo []byte, jsonMap json_map.JsonMapInt, absolutePaths *[][]interface{}) (errs []error) {
			// Check if recursive descent, aka. the next token is a dot
			errs = nil
			if togo[1] == '.' {
				// Add 0 to all the paths
				for i, absolutePath := range *absolutePaths {
					(*absolutePaths)[i] = append(absolutePath, 0)
				}
				// Check if there is a way to all of those paths
				_, errs = jsonMap.GetAbsolutePaths(absolutePaths)
				if errs != nil {
					return errs
				}
			}
			return nil
		},
	}
	root = state{
		name:       "Root node ($)",
		tokenRegex: regexp.MustCompile("\\$"),
		handler: 	func(token []byte, togo []byte, s *state, jsonMap json_map.JsonMapInt, absolutePaths *[][]interface{}) (next *state, err error) {
			// Root can go to a dot, index or a filter
			next = nil
			possibleStates := []*state{&dot, &index, &filter}

			for _, possibleState := range possibleStates {
				// If possible state's regex matches the start of the characters to go then that is the next state
				if occurrences := possibleState.tokenRegex.FindIndex(togo); len(occurrences) != 0 && occurrences[0] == 0 {
					next = possibleState
				}
			}
			if next == nil {
				return nil, JsonPathError.FillError(fmt.Sprintf("Could not find any of the possible states: %v, when at a %s token", possibleStates, s.name))
			}
			// We don't add anything to the absolute paths when handling the root node
			return next, nil
		},
		// The root does not need a validator as it is the first state
		validator: nil,
	}
	// A dummy state used for error messages
	start = state{
		name:       "Start",
		tokenRegex: nil,
		handler:    nil,
	}
)

// Given a JSON path and a json_map.JsonMapInt will return the list of absolute paths to each value found by that JSON path
//
// If the following path is given: $..property[*].name
// Then following absolute paths could look like this...
// {
//		{0, "property", 0, "name"},
//		{0, "property", 1, "name"},
//		{0, "property", 2, "name"},
// }
func ParseJsonPath(jsonPath string, jsonMap json_map.JsonMapInt) (absolutePaths [][]interface{}, err error) {
	// The collection of absolute paths to the values represented by the JSON path
	absolutePaths = make([][]interface{}, 0)
	previousState := &start
	currentState := &root
	jsonPathReader := bufio.NewReader(strings.NewReader(jsonPath))

	for {
		// Peak all the bytes until the end of the buffer
		next, _ := jsonPathReader.Peek(jsonPathReader.Size())
		occurrences := currentState.tokenRegex.FindIndex(next)
		// If there are no occurrences found or the occurrence doesn't start at the beginning of the buffer then error out
		if len(occurrences) == 0 || occurrences[0] != 0 {
			err = JsonPathError.FillError(fmt.Sprintf("%s token does not come after %s", currentState.name, previousState.name))
			break
		}

		// Consume len(token) number of bytes
		token := next[occurrences[0]:occurrences[1]]
		_, err = jsonPathReader.Discard(len(token))
		// If we have reached the end of the JSON path then break and return
		if err == io.EOF {
			break
		}
		// Call the handler function for the current state which will return the next state
		// Also set the current state to be the previous state
		previousState = currentState
		currentState, err = currentState.handler(token, next, currentState, jsonMap, &absolutePaths)
		if err != nil {
			return nil, err
		}
	}
	return absolutePaths, nil
}
