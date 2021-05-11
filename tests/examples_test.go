package tests

import (
	"encoding/json"
	"fmt"
	"github.com/andygello555/json-dom/code/js"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/utils"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

const (
	exampleLocation     = "../assets/tests/examples/"
	exampleEvalLocation = "../assets/tests/example_out/"
)

// Lookup of all names that require a defer call to catch a panic
var panicExampleNames = map[string]bool{
	"halting": true,
	"unsupported_lang": true,
}

// Contains expected stdout and stderr print headers for all examples that print
var checkStdoutErr = map[string]map[string][]string{
	"depths": {
		"stdout": []string{
			"Print call from: <$>",
			"Print call from: <$.person>",
		},
		"stderr": []string{
			"Error call from: <$>",
		},
	},
	"array_root": {
		"stdout": []string{
			"Print call from: <$.array.[1]>",
		},
		"stderr": []string{
		},
	},
	"json_path": {
		"stdout": []string{
			"Print call from: <$>:1:1",
			"map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:0 typeId:1 typeName:IndexKey]] 1:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:2 typeId:1 typeName:IndexKey]] 2:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:4 typeId:1 typeName:IndexKey]]] getValues:map[] setValues:map[]]",
			"Print call from: <$>:2:1",
			"map[_absolutePaths:map[0:map[0:map[key:JaneDoe typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:map[0:map[key:1 typeId:1 typeName:IndexKey] 1:map[key:3 typeId:1 typeName:IndexKey]] typeId:5 typeName:Slice]]] getValues:map[] setValues:map[]]",
			"Print call from: <$>:13:5",
			"map[0:Ava Forster] 1",
			"map[0:Louis Warren] 1",
			"Print call from: <$>:22:9",
			"Ava Forster &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@ == 'Ava Forster' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Ava Forster]",
			"Louis Warren &map[_absolutePaths:map[0:map[0:map[key:JaneDoe typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@ == 'Louis Warren' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Louis Warren]",
			"Print call from: <$>:28:5",
			"map[0:map[age:42 name:Molly Little] 1:map[age:62 name:Daniel Booth] 2:map[age:84 name:Oscar Hodgson] 3:map[name:Libby Ross] 4:map[age:20 name:Ava Forster]] 5",
			"map[0:map[name:Jayden Welch] 1:map[age:20 name:Louis Warren] 2:map[age:83 name:Libby Willis] 3:map[age:81 name:Mohammad Sutton] 4:map[age:49 name:Katie Cole]] 5",
			"Print call from: <$>:41:9",
			"map[age:42 name:Molly Little] &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@.name == 'Molly Little' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:42 name:Molly Little",
			"map[age:62 name:Daniel Booth] &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@.name == 'Daniel Booth' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:62 name:Daniel Booth]",
			"map[age:84 name:Oscar Hodgson] &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@.name == 'Oscar Hodgson' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:84 name:Oscar Hodgson]",
			"map[age:20 name:Libby Ross] &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@.name == 'Libby Ross' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Libby Ross]",
			"map[age:20 name:Ava Forster] &map[_absolutePaths:map[0:map[0:map[key:JohnSmith typeId:0 typeName:StringKey] 1:map[key:friends typeId:0 typeName:StringKey] 2:map[key:@.name == 'Ava Forster' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Ava Forster]",
		},
		"stderr": []string{
		},
	},
}

type exampleTableEntry struct {
	name string
	in   []byte
	out  interface{}
}

var stdoutBuffer strings.Builder
var stderrBuffer strings.Builder
var buffers = map[string]*strings.Builder{
	"stdout": &stdoutBuffer,
	"stderr": &stderrBuffer,
}
var exampleTable []exampleTableEntry

func init() {
	// Fill out example table by reading from file
	exampleTable = make([]exampleTableEntry, 0)
	if files, err := ioutil.ReadDir(exampleLocation); err != nil {
		panic(err)
	} else {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "example") {
				// Read the input file
				inFileBytes, err := ioutil.ReadFile(filepath.Join(exampleLocation, file.Name()))
				if err != nil {
					panic(err)
				}

				// Read the output file and Unmarshal to interface{} (array at root or object at root)
				outFileBytes, err := ioutil.ReadFile(filepath.Join(exampleEvalLocation, "out_" + strings.Replace(file.Name(), ".hjson", ".json", 1)))
				var outMap interface{}
				if err = json.Unmarshal(outFileBytes, &outMap); err != nil {
					panic(err)
				}

				exampleTable = append(exampleTable, exampleTableEntry{
					name: strings.Split(strings.SplitN(file.Name(), "_", 2)[1], ".")[0],
					in:   inFileBytes,
					out:  outMap,
				})
			}
		}
	}

	// Set the streams for the js module
	js.ExternalConsoleLogStdout = &stdoutBuffer
	js.ExternalConsoleLogStderr = &stderrBuffer

	// Set the halting time delay so that the halting problem examples run a bit quicker
	utils.HaltingDelay = 1
}

func TestExamples(t *testing.T) {
	// Iterate through examples
	for _, example := range exampleTable {
		// Run inside an anonymous function so that defers can be called safely
		func() {
			// Some tests test for appropriate panics so we will need to defer a function call to catch them
			shouldPanic := panicExampleNames[example.name]
			if shouldPanic {
				defer func() {
					if p := recover(); p != nil {
						switch example.name {
						case "halting":
							if !strings.Contains(p.(error).Error(), "(-1) Infinite loop has occurred after") {
								t.Errorf("Halting example panics but it is not a HaltingProblem error: %v", p)
							}
						case "unsupported_lang":
							if !strings.Contains(p.(error).Error(), "(-2) Script has an unsupported script language in the shebang line") {
								t.Errorf("Unsupported lang example panics but it is not a UnsupportedScriptLang error: %v", p)
							}
						}
						return
					}
				}()
			}

			// Create a JsonMap and unmarshal the input file into it
			jsonMap := jom.New()
			if err := jsonMap.Unmarshal(example.in); err != nil {
				t.Errorf("Could not Unmarshal into JsonMap: %v", err)
			}

			// Evaluate the JsonMap
			jsonMap.Run()

			// Check stdout and stderr if needed
			if printHeaders, ok := checkStdoutErr[example.name]; ok {
				// Check if the needed stdout and stderr print headers exist
				for headerTypeKey, headerType := range buffers {
					for _, header := range printHeaders[headerTypeKey] {
						if !strings.Contains(headerType.String(), header) {
							t.Errorf("%s for '%s' does not contain the following print header: %s", headerTypeKey, example.name, header)
						}
					}
					// Reset the buffers
					headerType.Reset()
				}
			}

			// Only check if the given example shouldn't panic
			if !shouldPanic {
				// Handle JsonMap's with arrays at their roots
				var insides interface{}
				if jsonMap.Array {
					insides = (*jsonMap.GetInsides())["array"]
				} else {
					insides = *jsonMap.GetInsides()
				}
				//b, _ := json.MarshalIndent(insides, "", "  ")
				//fmt.Println(string(b))

				// Finally, compare the insides of the JsonMap with the Unmarshalled expected output from the example_out dir
				utils.JsonMapEqualTest(t, insides, example.out, fmt.Sprintf("\"%s\"", example.name))
			}
		}()
	}
}
