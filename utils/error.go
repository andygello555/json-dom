package utils

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// CliError type for handling errors which occur in the CLI
type CliError struct {
	code     int	 // The return code
	internal bool    // Whether or not the error is internal or down to user input
	message  string  // The message to print along with the err message if given
}

var (
	ParseErr            = CliError{1, true, "Parse error has occurred"}
	FormatErr           = CliError{2, true, "Format error has occurred"}
	ReadFileErr			= CliError{3, true, "A read file error occurred"}
	KeyValueErr         = CliError{4, false, "Error while handling Key Value flag"}
	SubcommandErr       = CliError{5, false, "Subcommand not given/not recognised"}
	RequiredFlagErr     = CliError{6, false, "The following required flag was not given"}
	FileDoesNotExistErr = CliError{7, false, "The following file does not exist and cannot be read"}
	EvaluationErr		= CliError{8, false, "EVAL ERROR"}
)

func (e *CliError) Handle(err error, flagSetArray ...*flag.FlagSet) {
	if err != nil {
		fmt.Println(e.message + ":", err)
	} else {
		fmt.Println(e.message)
	}

	// PrintDefaults if not an internal error
	if !e.internal {
		if len(flagSetArray) == 0 {
			flag.PrintDefaults()
		} else {
			flagSetArray[0].PrintDefaults()
		}
	}

	// Finally exit, returning the exit code to the shell
	os.Exit(e.code)
}

type RuntimeError struct {
	code    int
	message string
}

// Runtime errors (negative codes)
var (
	HaltingProblem = RuntimeError{-1,"Infinite loop has occurred after"}
)

// Fill out a RuntimeError error with the given extra info
func (e *RuntimeError) FillError(extraInfo ...string) error {
	var b strings.Builder
	for i, s := range extraInfo {
		_, _ = fmt.Fprint(&b, s)
		if i < len(extraInfo) - 1 {
			_, _ = fmt.Fprint(&b, ", ")
		}
	}

	// Fill out the message
	var message string
	if b.String() != "" {
		// If there is extra info then add it after a colon
		message = fmt.Sprintf("(%d) %v: %v", e.code, e.message, b.String())
	} else {
		message = fmt.Sprintf("(%d) %v", e.code, e.message)
	}
	return errors.New(message)
}
