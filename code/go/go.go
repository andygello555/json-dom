// Contains runner for native Go callbacks within JOM.
//
// This is a fairly stripped down version of a script package due to there not being any VM to get and set values from/to.
package _go

import (
	"fmt"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/globals"
	"time"
)

// Registers "go" as a supported language.
func init() {
	code.RegisterLang("go", RunCallback)
}

// Runs a Go callback.
//
// Callback must have the signature:
//  func(json json_map.JsonMapInt)
// Otherwise RunCallback will panic.
//
// Halting Problem
//
// The given callback within code will be wrapped in a goroutine which will push an interrupt once the callback has finished.
// If the callback doesn't finish within globals.HaltingDelay seconds a separate goroutine will push the interrupt which
// will cause RunCallback to return early.
//
// Note: If a halting problem issue occurs then there will be a goroutine running the callback until it has finished, which may be never.
// Keep this in mind if you have a long running program which utilises native Go callback execution.
func RunCallback(code code.Code, jsonMap json_map.JsonMapInt) (data json_map.JsonMapInt, err error) {
	// Get the callback from the code object
	callback := code.Script.(func(json json_map.JsonMapInt))
	interrupt := make(chan bool)

	// Construct a wrapper around the callback which will write to the interrupt channel once finished
	callbackSafe := func() {
		defer func() {
			// If we have finished then we will signal to everyone that we have
			interrupt <- true
		}()
		callback(jsonMap)
	}

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
					fmt.Sprintf(globals.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), fmt.Sprintf("%v", code.Script)),
				)
				return
			}
			// Another error that we should panic for
			panic(caught)
		}
	}()

	// Start the timer which will also write to the interrupt channel to indicate that we are finished
	go func() {
		time.Sleep(time.Duration(globals.HaltingDelay) * globals.HaltingDelayUnits)
		interrupt <- true
	}()

	go callbackSafe()
	// We keep looping until we have read from the interrupt which signals us that the callback is done
	for {
		_, ok := <- interrupt
		if ok {
			break
		}
	}
	return jsonMap, err
}
