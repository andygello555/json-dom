package main

import (
	"errors"
	"flag"
	"fmt"
	_ "github.com/andygello555/json-dom/code/js"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/utils"
	"io/ioutil"
	"os"
	"strings"
	"unicode/utf8"
)

type Subcommands map[string]map[string]interface{}

// Flag Type for a list of Files
type Files []string

// Returns the String representation of the Files flag
func (s *Files) String() string {
	return fmt.Sprintf("%v", *s)
}

// Sets the value of the Files flag
func (s *Files) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

// Flag Type for a JsonPathScriptPair flag
type JsonPathScriptPair map[string]string

// Returns the String representation of the JsonPathScriptPair flag
func (jpscp *JsonPathScriptPair) String() string {
	// a=b, c=d, ...
	var b strings.Builder
	for key, element := range *jpscp {
		_, err := fmt.Fprintf(&b, "%v%c%v,", key, utils.KeyValuePairDelim, element)
		if err != nil {
			utils.FormatErr.Handle(err)
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
func (jpscp *JsonPathScriptPair) Set(value string) error {
	jsonPathScripts := strings.Split(value, ",")
	*jpscp = make(JsonPathScriptPair)
	for _, jsonPathScript := range jsonPathScripts {
		jsonPathScriptArr := strings.Split(jsonPathScript, string(utils.KeyValuePairDelim))
		// Throw an error if len is 0, aka: string is "="
		if len(jsonPathScriptArr) == 0 {
			return errors.New(fmt.Sprintf("%v is not a pair (syntax is \"<JSON path>%c<script>\")", jsonPathScript, utils.KeyValuePairDelim))
		}
		// Throw error if there is more than one delim in the string
		if len(jsonPathScriptArr) > 2 {
			return errors.New(fmt.Sprintf("%v is not a pair (must only be a single \"%c\" char)", jsonPathScript, utils.KeyValuePairDelim))
		}

		// Check if the json path given is valid
		if _, err := utils.ParseJsonPath(jsonPathScriptArr[0]); err != nil {
			return err
		}

		if len(jsonPathScriptArr) == 1 {
			// Use the default value of "0" if no value is specified
			(*jpscp)[jsonPathScriptArr[0]] = "// No script specified"
		} else {
			(*jpscp)[jsonPathScriptArr[0]] = jsonPathScriptArr[1]
		}
	}
	return nil
}

// usage: json-dom { eval | markup [-language <language>] [-eval] <key>:<value>,... } { -input <input> | -files <file>... } [-verbose]

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
		subcommandMap[key]["input"] = flagSet.String("input", "", "The json-dom object to read in (required if <file> is not given)")
		subcommandMap[key]["verbose"] = flagSet.Bool("verbose", false, "Verbose output")

		// Add the extra JsonPathScriptPair flag, language flag and eval flag to the markup subcommand
		if key == "markup" {
			keyValPair := new(JsonPathScriptPair)
			subcommandMap[key]["path-scripts"] = keyValPair
			flagSet.Var(keyValPair, "path-scripts", fmt.Sprintf("The JSONPath-script pairs that should be added to the input json-dom. Format: \"<JSON path>%cscript\" (at least 1 required)", utils.KeyValuePairDelim))
			subcommandMap[key]["language"] = flagSet.String("language", "js", "The language which the markups are in")
			subcommandMap[key]["eval"] = flagSet.Bool("eval", false, "Evaluate the JSON map after markup")
		}
		flagSet.Var(fileList, "files", "Files to evaluate as json-dom (required if --input not given)")
	}

	// Verify a subcommand has been given
	if len(os.Args) < 2 {
		utils.SubcommandErr.Handle(nil, subcommandMap["eval"]["flagSet"].(*flag.FlagSet),
			subcommandMap["markup"]["flagSet"].(*flag.FlagSet))
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
		utils.SubcommandErr.Handle(nil, subcommandMap["eval"]["flagSet"].(*flag.FlagSet),
			subcommandMap["markup"]["flagSet"].(*flag.FlagSet))
	}

	// Handle any parse errors
	if parseErr != nil {
		utils.ParseErr.Handle(parseErr)
	}

	// Check which subcommand was parsed and handle it
	for subcommand, element := range subcommandMap {
		flagSet := element["flagSet"].(*flag.FlagSet)
		verbose := *element["verbose"].(*bool)

		// Print flags if verbose
		if verbose {
			fmt.Println("flags:")
			const formatString = "\t%v = %v\n"
			for flagKey, flagElement := range element {
				if flagKey != "flagSet" {
					switch flagKey {
					case "path-scripts":
						fallthrough
					case "files":
						fmt.Printf(formatString, flagKey, flagElement)
					case "eval":
						fallthrough
					case "verbose":
						fmt.Printf(formatString, flagKey, *flagElement.(*bool))
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

			dataSet := make(map[string][]byte, 0)
			if len(*filesPtr) != 0 || *inputPtr != "" {
				// If both a file and a stdin input is given then evaluate the files first
				var data []byte
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
							utils.FileDoesNotExistErr.Handle(errors.New(file))
						}

						// Read the file
						var err error
						data, err = ioutil.ReadFile(file)
						if err != nil {
							utils.ReadFileErr.Handle(err)
						}
						dataSet[file] = data
					}
				} else {
					// Convert the input to a byte buffer
					data = []byte(*inputPtr)
					dataSet["stdin"] = data
				}
			} else {
				// Files and input not given so throw RequiredFlagErr
				utils.RequiredFlagErr.Handle(errors.New("files or input (one of these must be given)"),
					element["flagSet"].(*flag.FlagSet))
			}

			for dataName, data := range dataSet {
				if verbose {
					fmt.Printf("\n%s:\n", dataName)
				}
				switch subcommand {
				case "eval":
					// Evaluate the json-dom object
					eval, err := jom.Eval(data, verbose)
					if err != nil {
						utils.EvaluationErr.Handle(err)
					}

					// TODO: This is where saving to a destination file would come in
					fmt.Println(string(eval))
				case "markup":
					pathScripts := element["path-scripts"].(*JsonPathScriptPair)
					language := element["language"].(*string)
					eval := element["eval"].(*bool)

					// Check if any JSONPath-script pairs are present
					if len(*pathScripts) == 0 {
						// No path-scripts given so throw RequiredFlagErr
						utils.RequiredFlagErr.Handle(errors.New(
							fmt.Sprintf("a single path-script must be supplied as a \"<JSON path>%c<script>\" pair",
								utils.KeyValuePairDelim)),
							element["flagSet"].(*flag.FlagSet),
						)
					}

					// Unmarshal the data to a JsonMap
					jsonMap := jom.New()
					err := jsonMap.Unmarshal(data)
					if err != nil {
						utils.UnmarshalErr.Handle(errors.New(fmt.Sprintf("data: %s, err: %v", string(data), err)))
					}

					// Run the Markup for all paths
					for path, script := range *pathScripts {
						err = jsonMap.Markup(path, *language, script)
						if err != nil {
							utils.MarkupErr.Handle(err)
						}
					}

					if *eval {
						if data, err = jsonMap.Marshal(); err != nil {
							utils.MarshalErr.Handle(errors.New(fmt.Sprintf("JsonMap: %s, err: %v", jsonMap, err)))
						}

						var evalOut []byte
						evalOut, err = jom.Eval(data, verbose)
						if err != nil {
							utils.EvaluationErr.Handle(err)
						}
						fmt.Println(string(evalOut))
					} else {
						fmt.Println(jsonMap)
					}
				}
			}

			os.Exit(0)
		}
	}
	utils.ParseErr.Handle(nil)
}
