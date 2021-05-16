package _go

import (
	"fmt"
	"github.com/andygello555/json-dom/code"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"time"
)

func init() {
	code.RegisterLang("go", RunCallback)
}

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
			if caught == utils.HaltingProblem {
				err = utils.HaltingProblem.FillError(
					duration.String(),
					fmt.Sprintf(utils.ScriptErrorFormatString, jsonMap.GetCurrentScopePath(), fmt.Sprintf("%v", code.Script)),
				)
				return
			}
			// Another error that we should panic for
			panic(caught)
		}
	}()

	// Start the timer which will also write to the interrupt channel to indicate that we are finished
	go func() {
		time.Sleep(time.Duration(utils.HaltingDelay) * utils.HaltingDelayUnits)
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
