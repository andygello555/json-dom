package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/andygello555/json-dom/utils"
	"io/ioutil"
	"os"
	"strings"
	"unicode/utf8"
)

const KeyValuePairDelim = ':'

// CliError type for handling errors
type CliError struct {
	code int	    // The return code
	internal bool   // Whether or not the error is internal or down to user input
	message string  // The message to print along with the err message if given
}

var (
	ParseErr            = CliError{1, true, "Parse error has occurred"}
	FormatErr           = CliError{2, true, "Format error has occurred"}
	ReadFileErr			= CliError{3, true, "A read file error occurred"}
	KeyValueErr         = CliError{4, false, "Error while handling Key Value flag"}
	SubcommandErr       = CliError{5, false, "Subcommand not given/not recognised"}
	RequiredFlagErr     = CliError{6, false, "The following required flag was not given"}
	FileDoesNotExistErr = CliError{7, false, "The following file does not exist and cannot be read"}
	EvaluationErr		= CliError{8, false, "The following error occurred while evaluating a json-dom object"}
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

type Subcommands map[string]map[string]interface{}

// Flag Type for a list of Files
type Files []string

// Returns the String representation of the Files flag
func (s *Files) String() string {
	return fmt.Sprintf("%v", *s)
}

// Sets the value of the Files flag
func (s *Files) Set(value string) error {
	fmt.Println(value)
	*s = strings.Split(value, ",")
	return nil
}

// Flag Type for a KeyValuePair flag
type KeyValuePair map[string]string

// Returns the String representation of the Files flag
func (kvp *KeyValuePair) String() string {
	// a=b, c=d, ...
	var b strings.Builder
	for key, element := range *kvp {
		_, err := fmt.Fprintf(&b, "%v%v%v,", key, KeyValuePairDelim, element)
		if err != nil {
			FormatErr.Handle(err)
		}
	}

	// Remove last character
	bString := b.String()
	r, size := utf8.DecodeLastRuneInString(bString)
	if r == utf8.RuneError && (size == 0 || size == 1) {
		size = 0
	}
	return bString[:len(bString) - size]
}

// Sets the value of the Files flag
func (kvp *KeyValuePair) Set(value string) error {
	keyVals := strings.Split(value, ",")
	*kvp = make(KeyValuePair)
	for _, keyVal := range keyVals {
		keyValArray := strings.Split(keyVal, string(KeyValuePairDelim))
		// Throw an error if len is 0, aka: string is "="
		if len(keyValArray) == 0 {
			return errors.New(fmt.Sprintf("%v is not a pair (syntax is 'key%vvalue')", keyVal, KeyValuePairDelim))
		}
		// Throw error if there is more than one delim in the string
		if len(keyValArray) > 2 {
			return errors.New(fmt.Sprintf("%v is not a pair (must only be a single %v char)", keyVal, KeyValuePairDelim))
		}

		if len(keyValArray) == 1 {
			// Use the default value of "0" if no value is specified
			(*kvp)[keyValArray[0]] = "0"
		} else {
			(*kvp)[keyValArray[0]] = keyValArray[1]
		}
	}
	return nil
}

// usage: json-dom {eval|markup <key>:<value>,...} {-i <input> | <file>...} [-d <file>] [-v]

func main() {
	// Subcommands
	subcommandMap := Subcommands{
		"eval": map[string]interface{}{
			"flagSet": flag.NewFlagSet("eval", flag.ExitOnError),  // Evaluates a json-dom file/input
		},
		"markup": map[string]interface{}{
			"flagSet": flag.NewFlagSet("markup", flag.ExitOnError),  // Markup a json-dom file/input with a file/input,
		},
	}

	for key, element := range subcommandMap {
		// Flag pointers
		fileList := new(Files)
		flagSet := element["flagSet"].(*flag.FlagSet)
		subcommandMap[key]["files"] = fileList
		subcommandMap[key]["input"] = flagSet.String("input", "", "The json-dom object to read in (Required if <file> is not given)")
		subcommandMap[key]["verbose"] = flagSet.Bool("verbose", false, "Verbose output")

		// Add the extra KeyValuePair flag to the markup subcommand
		if key == "markup" {
			keyValPair := new(KeyValuePair)
			subcommandMap[key]["key-vals"] = keyValPair
			flagSet.Var(keyValPair, "key-vals", "The key value pairs that should be added to the input json-dom (Required)")
		}
		flagSet.Var(fileList, "files", "Files to evaluate as json-dom (Required if --input not given)")
	}

	// Verify a subcommand has been given
	if len(os.Args) < 2 {
		SubcommandErr.Handle(nil)
	}

	var parseErr error
	flags := os.Args[2:]
	switch os.Args[1] {
	case "eval":
		fallthrough
	case "markup":
		flagSet := subcommandMap[os.Args[1]]["flagSet"].(*flag.FlagSet)
		parseErr = flagSet.Parse(flags)
	default:
		SubcommandErr.Handle(nil)
	}

	// Handle any parse errors
	if parseErr != nil {
		ParseErr.Handle(parseErr)
	}

	// Check which subcommand was parsed and handle it
	for _, element := range subcommandMap {
		flagSet := element["flagSet"].(*flag.FlagSet)
		verbose := *element["verbose"].(*bool)

		// Print flags if verbose
		if verbose {
			fmt.Println("flags:")
			const formatString = "\t%v = %v\n"
			for flagKey, flagElement := range element {
				if flagKey != "flagSet" {
					switch flagKey {
					case "key-vals":
						fallthrough
					case "files":
						fmt.Printf(formatString, flagKey, flagElement)
					case "verbose":
						fmt.Printf(formatString, flagKey, verbose)
					default:
						// Default just casts the pointer to a string pointer and takes the value at the location
						fmt.Printf(formatString, flagKey, *flagElement.(*string))
					}
				}
			}
		}

		if flagSet.Parsed() {
			// Recast the pointers
			filesPtr := element["files"].(*Files)
			inputPtr := element["input"].(*string)

			if len(*filesPtr) != 0 && *inputPtr != "" {
				var data []byte

				// If both a file and a stdin input is given then evaluate the files first
				if len(*filesPtr) != 0 {
					if verbose {
						fmt.Println("\nfiles:")
					}
					for _, file := range *filesPtr {
						exists := utils.CheckFileExists(file)
						if verbose {
							formatString := "\t%v"
							if exists {
								formatString += " (exists)"
							} else {
								formatString += " (does not exist)"
							}
							formatString += "\n"
							fmt.Printf(formatString, file)
						}

						// Throw an error if a file doesn't exist
						if !exists {
							FileDoesNotExistErr.Handle(errors.New(file))
						}

						// Read the file
						var err error
						data, err = ioutil.ReadFile(file)
						if err != nil {
							ReadFileErr.Handle(err)
						}
					}
				} else {
					// Convert the input to a byte buffer
					data = []byte(*inputPtr)
				}

				// Evaluate the json-dom object
				eval, err := Eval(data)
				if err != nil {
					EvaluationErr.Handle(err)
				}

				// TODO: This is where saving to a destination file would come in
				fmt.Println()
				fmt.Println("CLI main:", eval)
			} else {
				// Files and input not given so throw RequiredFlagErr
				RequiredFlagErr.Handle(errors.New("files or input (one of these must be given)"),
					element["flagSet"].(*flag.FlagSet))
			}
			os.Exit(0)
		}
	}
	ParseErr.Handle(nil)
}
