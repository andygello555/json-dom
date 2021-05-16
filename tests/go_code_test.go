package tests

import (
	"encoding/json"
	"fmt"
	_ "github.com/andygello555/json-dom/code/go"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var goCodeExamples []exampleTableEntryWithMarkup

type exampleTableEntryWithMarkup struct {
	name   string
	markup map[string]func(json json_map.JsonMapInt)
	in     []byte
	out    interface{}
}

// An array of maps which correspond to the markups to be applied to each example.
// The key of the map is the JSON path and the value is the script that the JsonMap should be marked up with at that path
var goCodeExampleMarkups = []map[string]func(json json_map.JsonMapInt) {
	{
		"$.script": func(json json_map.JsonMapInt) {
			name, _ := json.JsonPathSelector("$.name")
			firstLast := strings.Split(name[0].Value.(string), " ")
			_ = json.JsonPathSetter("$.first_name", firstLast[0])
			_ = json.JsonPathSetter("$.last_name", firstLast[1])
			_ = json.JsonPathSetter("$.name", nil)
		},
	},
	{
		"$.seren-scrippidy": func(json json_map.JsonMapInt) {
			name, _ := json.JsonPathSelector("$.person.name")
			firstLast := strings.Split(name[0].Value.(string), " ")
			_ = json.JsonPathSetter("$.person.first_name", firstLast[0])
			_ = json.JsonPathSetter("$.person.last_name", firstLast[1])
			_ = json.JsonPathSetter("$.person.name", nil)
		},
		"$.person.script1": func(json json_map.JsonMapInt) {
			_ = json.JsonPathSetter("$.age", float64(18))
		},
		"$.person.pets[0].attrs.script2": func(json json_map.JsonMapInt) {
			for i := 0; i<10; i++ {
				_ = json.JsonPathSetter("$.Woof" + strconv.Itoa(i), "Bark")
			}
		},
		"$.person.pets[1].script3": func(json json_map.JsonMapInt) {
			_ = json.JsonPathSetter("$.name", "Nyan Cat")
		},
	},
}

func init() {
	// Fill out example table by reading from file
	goCodeExamples = make([]exampleTableEntryWithMarkup, 0)
	if files, err := ioutil.ReadDir(exampleLocation); err != nil {
		panic(err)
	} else {
		for i, file := range files {
			if strings.HasPrefix(file.Name(), "example") && i < len(goCodeExampleMarkups) {
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

				goCodeExamples = append(goCodeExamples, exampleTableEntryWithMarkup{
					name:   strings.Split(strings.SplitN(file.Name(), "_", 2)[1], ".")[0],
					markup: goCodeExampleMarkups[i],
					in:     inFileBytes,
					out:    outMap,
				})
			}
		}
	}

	// Set the halting time delay so that the halting problem examples run a bit quicker
	utils.HaltingDelay = 1
}

// Similar to TestExamples but with the script key-value pairs being replaced by native go functions instead. This is
// to test the Markup, Code type and go code execution.
func TestGoCodeExamples(t *testing.T) {
	// Iterate through examples
	for _, example := range goCodeExamples {
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
							if !strings.Contains(p.(error).Error(), "(-2) Unsupported script language in shebang") {
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

			// Then we strip out all the script key-value pairs
			jsonMap.Strip()

			// For each markup call Markup to mark the JSON map up with it
			for jsonPath, callback := range example.markup {
				if err := jsonMap.JsonPathSetter(jsonPath, callback); err != nil {
					t.Errorf("Could not set path \"%s\" to callback in example %s", jsonPath, example.name)
				}
			}

			// Evaluate the JsonMap
			jsonMap.Run()

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
