package globals

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// CliError type for handling errors which occur in the CLI.
type CliError struct {
	// The return code.
	code     int
	// Whether or not the error is internal or down to user input.
	internal bool
	// The message to print along with the err message if given.
	message  string
}

// CliError(s) (positive codes).
var (
	ParseErr            = CliError{1, true, "Parse error has occurred"}
	FormatErr           = CliError{2, true, "Format error has occurred"}
	ReadFileErr			= CliError{3, true, "A read file error occurred"}
	SubcommandErr       = CliError{5, false, "Subcommand not given/not recognised"}
	RequiredFlagErr     = CliError{6, false, "The following required flag was not given"}
	FileDoesNotExistErr = CliError{7, false, "The following file does not exist and cannot be read"}
	MarshalErr			= CliError{8, false, "The following JsonMap cannot be marshalled for the following reasons"}
	UnmarshalErr		= CliError{9, false, "The following data cannot be unmarshalled for the following reasons"}
	EvaluationErr		= CliError{10, false, "EVAL ERROR"}
	MarkupErr			= CliError{11, false, "MARKUP ERROR"}
)

// Handles print of error details and exit codes of CliError.
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
			for _, flagSet := range flagSetArray {
				fmt.Printf("\n%s: %s\n", flagSet.Name(), CliSubcommandDescriptions()[flagSet.Name()])
				flagSet.PrintDefaults()
			}
		}
	}

	// Finally exit, returning the exit code to the shell
	os.Exit(e.code)
}

// For handling errors that occur at runtime.
type RuntimeError struct {
	// The code of the error. No real use other than identification.
	//
	// Note: All RuntimeError(s) have negative codes.
	code    int
	// The main message which will prefix any handled errors.
	message string
}

// RuntimeError(s) (negative codes).
var (
	HaltingProblem        = RuntimeError{-1, "Infinite loop has occurred after"}
	UnsupportedScriptLang = RuntimeError{-2, "Unsupported script language in shebang"}
	BuiltinGetterError    = RuntimeError{-3, "An error has occurred when getting the value of a builtin variable"}
	OverriddenBuiltin     = RuntimeError{-4, "The following builtin was overridden"}
	ScriptError           = RuntimeError{-5, "The following script has caused an error"}
	JsonPathError		  = RuntimeError{-6, "A JSON path could not be evaluated for the following reason(s)"}
)

// Fill out a RuntimeError error with the given extra info.
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

// Fill out a RuntimeError error with the given list of errors.
//
// For all errors with the same code and message the code and message will be removed from that error before being
// appended to the returned error. This stops situations where you have:
//  (-6) A JSON path could not be evaluated for the following reasons: ..., (-6) A JSON path could not be evaluated for the following reasons: ...
func (e *RuntimeError) FillFromErrors(errs []error) error {
	// Create an array of the error messages so that they can be re-wrapped into another RuntimeError
	errString := make([]string, len(errs))
	for errNo, err := range errs {
		errString[errNo] = strings.Replace(err.Error(), fmt.Sprintf("(%d) %s: ", e.code, e.message), "", -1)
	}
	return e.FillError(errString...)
}
