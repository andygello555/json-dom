package tests

import (
	"encoding/json"
	"fmt"
	_ "github.com/andygello555/json-dom/code/go"
	"github.com/andygello555/json-dom/code/js"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/jom/json_map"
	"github.com/andygello555/json-dom/utils"
	"io/ioutil"
	"path/filepath"
	"strconv"
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
			"Print call from: <$>:9:1",
			":Ava Forster",
			":Louis Warren]",
			"Print call from: <$>:19:5",
			"Ava Forster &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@ == 'Ava Forster' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Ava Forster]",
			"Louis Warren &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@ == 'Louis Warren' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Louis Warren]",
			"Print call from: <$>:25:1",
			"Print call from: <$>:39:5",
			"map[age:20 name:Jayden Welch] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Jayden Welch' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Jayden Welch]",
			"map[age:20 name:Louis Warren] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Louis Warren' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Louis Warren]",
			"map[age:83 name:Libby Willis] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Libby Willis' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:83 name:Libby Willis]",
			"map[age:81 name:Mohammad Sutton] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Mohammad Sutton' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:81 name:Mohammad Sutton]",
			"map[age:49 name:Katie Cole] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Katie Cole' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:49 name:Katie Cole]",
			"map[age:42 name:Molly Little] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Molly Little' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:42 name:Molly Little]",
			"map[age:62 name:Daniel Booth] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Daniel Booth' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:62 name:Daniel Booth]",
			"map[age:84 name:Oscar Hodgson] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Oscar Hodgson' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:84 name:Oscar Hodgson]",
			"map[age:20 name:Libby Ross] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Libby Ross' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Libby Ross]",
			"map[age:20 name:Ava Forster] &map[_absolutePaths:map[0:map[0:map[key:friends typeId:7 typeName:RecursiveLookup] 1:map[key:@.name == 'Ava Forster' typeId:3 typeName:Filter]]] getValues:map[] setValues:map[]] &map[age:20 name:Ava Forster]",
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

// Each supported language should appear in this array.
// Each example will be run in each supported language. If the supported language in the array below does not have the
// strip property set then the marking up of the example will be skipped. If it does then the example will be stripped
// and marked up with the available markups.
var differentLanguageMarkups = []struct{
	name        string
	// An array of maps which correspond to the markups to be applied to each example.
	markups     []map[string]interface{}
	// Whether or not to markup each example with their corresponding markups in the markups array
	strip       bool
	// Whether or not the stderr and stdout should be checked
	checkOutErr bool
}{
	// Our examples are written in Javascript so we don't have to mark them up
	{
		name:        "JS",
		markups:     nil,
		strip: 	     false,
		checkOutErr: true,
	},
	{
		name:        "GO",
		strip:       true,
		checkOutErr: false,
		markups:     []map[string]interface{} {
			{
				"$.script": func(json json_map.JsonMapInt) {
					name := json.MustGet("$.name")[0].(string)
					firstLast := strings.Split(name, " ")
					json.MustSet("$.first_name", firstLast[0])
					json.MustSet("$.last_name", firstLast[1])
					json.MustDelete("$.name")
				},
			},
			{
				"$.seren-scrippidy": func(json json_map.JsonMapInt) {
					name := json.MustGet("$.person.name")[0].(string)
					firstLast := strings.Split(name, " ")
					json.MustSet("$.person.first_name", firstLast[0])
					json.MustSet("$.person.last_name", firstLast[1])
					json.MustDelete("$.person.name")
				},
				"$.person.script1": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.age", float64(18))
				},
				"$.person.pets[0].attrs.script2": func(json json_map.JsonMapInt) {
					for i := 0; i < 10; i++ {
						_ = json.JsonPathSetter("$.Woof"+strconv.Itoa(i), "Bark")
					}
				},
				"$.person.pets[1].script3": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.name", "Nyan Cat")
				},
			},
			{
				"$.seren-scrippidy": func(json json_map.JsonMapInt) {
					name := json.MustGet("$.person.name")[0].(string)
					firstLast := strings.Split(name, " ")
					json.MustSet("$.person.first_name", firstLast[0])
					json.MustSet("$.person.last_name", firstLast[1])
					json.MustDelete("$.person.name")
				},
				"$.person.script1": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.age", float64(18))
				},
				"$.person.pets[2].attrs.script2": func(json json_map.JsonMapInt) {
					for i := 0; i < 10; i++ {
						_ = json.JsonPathSetter("$.Woof"+strconv.Itoa(i), "Bark")
					}
				},
			},
			{
				"$.delete_attrs": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.attrs", nil)
				},
				"$.attrs.clown_shoe": func(json json_map.JsonMapInt) {
					clownShoe, _ := json.JsonPathSelector("$.shoe_size")
					_ = json.JsonPathSetter("$.clown_shoe_size", clownShoe[0].Value.(float64)+3)
				},
				"$.person.script": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.age", float64(18))
				},
				"$.person.pets[2].attrs.script": func(json json_map.JsonMapInt) {
					for i := 0; i < 10; i++ {
						_ = json.JsonPathSetter("$.Woof"+strconv.Itoa(i), "Bark")
					}
				},
			},
			{
				"$.nested_boi.script": func(json json_map.JsonMapInt) {
					_ = json.JsonPathSetter("$.Hello", "World")
				},
				"$.d": func(json json_map.JsonMapInt) {
					counter, _ := json.JsonPathSelector("$.counter")
					_ = json.JsonPathSetter("$.counter", counter[0].Value.(float64)*3)
				},
				"$.a": func(json json_map.JsonMapInt) {
					counter, _ := json.JsonPathSelector("$.counter")
					_ = json.JsonPathSetter("$.counter", counter[0].Value.(float64)+6)
				},
				"$.c": func(json json_map.JsonMapInt) {
					counter, _ := json.JsonPathSelector("$.counter")
					_ = json.JsonPathSetter("$.counter", counter[0].Value.(float64)/2)
				},
				"$.b": func(json json_map.JsonMapInt) {
					counter, _ := json.JsonPathSelector("$.counter")
					_ = json.JsonPathSetter("$.counter", counter[0].Value.(float64)-4)
				},
				"$.e": func(json json_map.JsonMapInt) {
					counter, _ := json.JsonPathSelector("$.counter")
					_ = json.JsonPathSetter("$.counter", counter[0].Value.(float64)*3)
				},
			},
			{
				"$.script": func(json json_map.JsonMapInt) {
					i := 0
					for {
						_ = json.JsonPathSetter("$."+strconv.Itoa(i), float64(i))
					}
				},
			},
			{
				"$.people[0].script": func(json json_map.JsonMapInt) {
					json.MustPush("$.attrs", "Married to Nick Miller (spoilers)")
				},
				"$.people[1].script": func(json json_map.JsonMapInt) {
					json.MustPush("$.attrs", "Married to Jessica Day (spoilers)")
				},
				"$.scrippidy_script": func(json json_map.JsonMapInt) {
					json.MustPush("$.people", map[string]interface{}{
						"name": "Winston Bishop",
						"attrs": []interface{}{
							"Ferguson",
							"Married to Ally (spoilers)",
						},
					})
				},
			},
			{},
			{
				"$.array[0].script": func(json json_map.JsonMapInt) {
					name := json.MustGet("$.name")[0].(string)
					firstLast := strings.Split(name, " ")
					json.MustSet("$.first_name", firstLast[0])
					json.MustSet("$.last_name", firstLast[1])
					json.MustDelete("$.name")
				},
				"$.array[1].script": func(json json_map.JsonMapInt) {
					name := json.MustGet("$.name")[0].(string)
					firstLast := strings.Split(name, " ")
					json.MustSet("$.first_name", firstLast[0])
					json.MustSet("$.last_name", firstLast[1])
					json.MustDelete("$.name")
				},
			},
			{
				"$.script": func(json json_map.JsonMapInt) {
					basePath := "$..friends"
					stringSet, _ := json.JsonPathSelector(basePath + "[?(typeof @ == 'string')]")
					defaultAgeNode, _ := json.JsonPathSelector("$.default_age")
					defaultAge := defaultAgeNode[0].Value.(float64)

					for _, node := range stringSet {
						normalised := make(map[string]interface{})
						normalised["name"] = node.Value
						normalised["age"] = defaultAge
						_ = json.JsonPathSetter(basePath+"[?(@ == '"+node.Value.(string)+"')]", normalised)
					}

					objectSet, _ := json.JsonPathSelector(basePath + "[?(typeof @ == 'object')]")
					for _, node := range objectSet {
						normalised := node.Value.(map[string]interface{})
						if _, ok := normalised["name"]; !ok {
							normalised["name"] = "Bob bob"
						}

						if _, ok := normalised["age"]; !ok {
							normalised["age"] = defaultAge
						}
						_ = json.JsonPathSetter(basePath+"[?(@.name == '"+normalised["name"].(string)+"')]", normalised)
					}

					_ = json.JsonPathSetter("$..friends[0]", nil)
					_ = json.JsonPathSetter("$.default_age", nil)
				},
			},
		},
	},
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
	// For each supported language we will run all the examples which is run in a subtest
	for _, supportedLang := range differentLanguageMarkups {
		t.Run(supportedLang.name, func(tt *testing.T) {
			markupsAvailable := supportedLang.strip
			markups := supportedLang.markups
			// Iterate through examples
			for exampleNo, example := range exampleTable {
				// Run as a subtest only if...
				// 1. There are no markups available for any example for the language (like the Javascript examples)
				// 2. There are markups available for all examples for the language and the current example has a markup
				if !markupsAvailable || exampleNo < len(markups) && exampleNo >= 0 && len(markups[exampleNo]) != 0 {
					tt.Run(example.name, func(ttt *testing.T) {
						// Some tests test for appropriate panics so we will need to defer a function call to catch them
						shouldPanic := panicExampleNames[example.name]
						if shouldPanic && !markupsAvailable || exampleNo < len(markups) && exampleNo >= 0 && len(markups[exampleNo]) != 0 {
							defer func() {
								if p := recover(); p != nil {
									switch example.name {
									case "halting":
										if !strings.Contains(p.(error).Error(), "(-1) Infinite loop has occurred after") {
											ttt.Errorf("Halting example panics but it is not a HaltingProblem error: %v", p)
										}
									case "unsupported_lang":
										if !strings.Contains(p.(error).Error(), "(-2) Unsupported script language in shebang") {
											ttt.Errorf("Unsupported lang example panics but it is not a UnsupportedScriptLang error: %v", p)
										}
									}
									return
								}
							}()
						}

						// Create a JsonMap and unmarshal the input file into it
						jsonMap := jom.New()
						if err := jsonMap.Unmarshal(example.in); err != nil {
							ttt.Errorf("Could not Unmarshal into JsonMap: %v", err)
						}

						// If we have available markups we should strip the jsonMap and mark it up
						if markupsAvailable {
							// Strip out all the script key-value pairs
							jsonMap.Strip()

							// For each markup call JsonPathSetter to mark the JSON map up with it
							for jsonPath, callback := range markups[exampleNo] {
								if err := jsonMap.JsonPathSetter(jsonPath, callback); err != nil {
									t.Errorf("Could not set path \"%s\" to callback in example %s, err: %v", jsonPath, example.name, err)
								}
							}
						}

						// Evaluate the JsonMap
						jsonMap.Run()

						if supportedLang.checkOutErr {
							// Check stdout and stderr if needed
							if printHeaders, ok := checkStdoutErr[example.name]; ok {
								// Check if the needed stdout and stderr print headers exist
								for headerTypeKey, headerType := range buffers {
									for _, header := range printHeaders[headerTypeKey] {
										if !strings.Contains(headerType.String(), header) {
											fmt.Println(headerType.String())
											ttt.Errorf("%s for '%s' does not contain the following print header: \"%s\"", headerTypeKey, example.name, header)
										}
									}
									// Reset the buffers
									headerType.Reset()
								}
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
							utils.JsonMapEqualTest(ttt, insides, example.out, fmt.Sprintf("\"%s\"", example.name))
						}
					})
				}
			}
		})
	}
}
