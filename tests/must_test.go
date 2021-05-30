package tests

import (
	"encoding/json"
	"fmt"
	"github.com/andygello555/gotils/maps"
	"github.com/andygello555/gotils/slices"
	"github.com/andygello555/json-dom/jom"
	"github.com/andygello555/json-dom/jom/json_map"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	exampleMustOutLocation = "../assets/tests/must_out/"
	exampleMustOutPrefix   = "must_out_"
	emptyString            = ""
)

var exampleJsonMap json_map.JsonMapInt
var exampleMustOut []interface{}

func setupTest(t *testing.T, setter bool) {
	// Get the directory name from the test name (split at '/' then trim "Test" from the left)
	directoryName := strings.ToLower(strings.TrimLeft(strings.Split(t.Name(), "/")[0], "Test"))
	// Fill example JsonMap
	exampleJsonMap = jom.New()
	err := exampleJsonMap.Unmarshal([]byte(`
	{
		hello: world
		people: [
			{
				name: Jeff
				age: 20
			}
			{
				name: Bob
				age: 24
			}
			{
				name: Tim
				age: 38
			}
		]
	}
	`))
	if err != nil {
		t.Error("Cannot Unmarshal test data into JsonMap:", err)
	}

	// Fill out expected outputs array only if we are testing setters
	if setter {
		exampleMustOut = make([]interface{}, 0)
		if files, err := ioutil.ReadDir(filepath.Join(exampleMustOutLocation, directoryName)); err != nil {
			panic(err)
		} else {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), exampleMustOutPrefix) {
					// Read the output file and Unmarshal to interface{} (array at root or object at root)
					outFileBytes, err := ioutil.ReadFile(filepath.Join(exampleMustOutLocation, directoryName, file.Name()))
					var outMap interface{}
					if err = json.Unmarshal(outFileBytes, &outMap); err != nil {
						panic(err)
					}

					exampleMustOut = append(exampleMustOut, outMap)
				}
			}
		}
	}
}

func testNoFromTestName(t *testing.T) int {
	testNo, _ := strconv.Atoi(strings.Split(t.Name(), "/")[1])
	return testNo
}

func testEqualityWithOut(t *testing.T, failString string, panics bool) {
	if !panics {
		// Handle JsonMap's with arrays at their roots
		var insides interface{}
		if exampleJsonMap.IsArray() {
			insides = (*exampleJsonMap.GetInsides())["array"]
		} else {
			insides = *exampleJsonMap.GetInsides()
		}
		//b, _ := json.MarshalIndent(insides, "", "  ")
		//fmt.Println(string(b))

		// Check for equality between jsonMap and expected output
		maps.JsonMapEqualTest(t, insides, exampleMustOut[testNoFromTestName(t) - 1], failString)
	}
}

func testPanic(t *testing.T, errMsg string, panics bool) {
	if panics {
		testNo := testNoFromTestName(t)
		if p := recover(); p != nil {
			if !strings.Contains(p.(error).Error(), errMsg) {
				t.Errorf("%d panics but it does not contain \"%s\", instead it contains: \"%v\"", testNo, errMsg, p)
			}
		} else {
			t.Errorf("%d does not panic", testNo)
		}
		return
	}
}

func TestMustSet(t *testing.T) {
	for testNo, test := range []struct{
		jsonPath string
		value    interface{}
		panics   bool
		errMsg   string
	}{
		{
			"$.hello",
			"me",
			false,
			emptyString,
		},
		{
			"$.people",
			"no people here",
			false,
			emptyString,
		},
		{
			"$.people[4]",
			"oh no",
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Index (4) is out of bounds for array of length 3",
		},
		{
			"$.hello",
			nil,
			false,
			emptyString,
		},
	} {
		t.Run(strconv.Itoa(testNo + 1), func(tt *testing.T) {
			// Test for panics if the test is supposed to
			defer testPanic(tt, test.errMsg, test.panics)

			// Fill the example JsonMap and then run the MustSet function
			setupTest(tt, true)
			exampleJsonMap.MustSet(test.jsonPath, test.value)

			// Check for equality between jsonMap and expected output
			testEqualityWithOut(tt, fmt.Sprintf("MustSet(%s, %v)", test.jsonPath, test.value), test.panics)
		})
	}
}

func TestMustGet(t *testing.T) {
	for testNo, test := range []struct{
		jsonPath    string
		expectedOut []interface{}
		panics      bool
		errMsg      string
	}{
		{
			"$.hello",
			[]interface{}{"world"},
			false,
			emptyString,
		},
		{
			"$.people[0, 2]",
			[]interface{}{
				map[string]interface{} {
					"name": "Jeff",
					"age": float64(20),
				},
				map[string]interface{} {
					"name": "Tim",
					"age": float64(38),
				},
			},
			false,
			emptyString,
		},
		{
			"$.does_not_exist",
			[]interface{}{},
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Key 'does_not_exist' does not exist in map",
		},
	} {
		t.Run(strconv.Itoa(testNo + 1), func(tt *testing.T) {
			// Test for panics if the test is supposed to
			defer testPanic(tt, test.errMsg, test.panics)

			// Fill the example JsonMap and then run the MustSet function
			setupTest(tt, false)
			nodes := exampleJsonMap.MustGet(test.jsonPath)

			// Check for equality between the retrieved nodes and the expectedOut
			if !slices.SameElements(nodes, test.expectedOut) {
				tt.Errorf("MustGet(%s) gets nodes \"%v\" not \"%v\"", test.jsonPath, nodes, test.expectedOut)
			}
		})
	}
}

func TestMustDelete(t *testing.T) {
	for testNo, test := range []struct{
		jsonPath string
		panics   bool
		errMsg   string
	}{
		{
			"$.hello",
			false,
			emptyString,
		},
		{
			"$.people",
			false,
			emptyString,
		},
		{
			"$.people[4]",
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Index (4) is out of bounds for array of length 3",
		},
		{
			"$.people[0, 1]",
			false,
			emptyString,
		},
	} {
		t.Run(strconv.Itoa(testNo + 1), func(tt *testing.T) {
			// Test for panics if the test is supposed to
			defer testPanic(tt, test.errMsg, test.panics)

			// Fill the example JsonMap and then run the MustDelete function
			setupTest(tt, true)
			exampleJsonMap.MustDelete(test.jsonPath)

			// Check for equality between jsonMap and expected output
			testEqualityWithOut(tt, fmt.Sprintf("MustDelete(%s)", test.jsonPath), test.panics)
		})
	}
}

func TestMustPush(t *testing.T) {
	for testNo, test := range []struct{
		jsonPath string
		value    interface{}
		indices  []int
		panics   bool
		errMsg   string
	}{
		{
			"$.people",
			map[string]interface{}{
				"name": "Sarah",
				"age": float64(41),
			},
			[]int{},
			false,
			emptyString,
		},
		{
			"$.people",
			map[string]interface{}{
				"name": "Sarah",
				"age": float64(41),
			},
			// Should give a null value at index 5
			[]int{0, 1, 1, 0, 6},
			false,
			emptyString,
		},
		// Demonstrates what happens when the path you specify is not pointing to a []interface{} value
		{
			"$.*",
			map[string]interface{}{
				"name": "Sarah",
				"age": float64(41),
			},
			[]int{},
			false,
			emptyString,
		},
		{
			"$.does_not_exist",
			map[string]interface{}{
				"name": "Sarah",
				"age": float64(41),
			},
			[]int{},
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Key 'does_not_exist' does not exist in map",
		},
		{
			"$.does_not_exist.defo_not_exist",
			map[string]interface{}{
				"name": "Sarah",
				"age": float64(41),
			},
			[]int{},
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Cannot access key |StringKey: defo_not_exist| of type \"<nil>\"",
		},
	} {
		t.Run(strconv.Itoa(testNo + 1), func(tt *testing.T) {
			// Test for panics if the test is supposed to
			defer testPanic(tt, test.errMsg, test.panics)

			// Fill the example JsonMap and then run the MustPush function
			setupTest(tt, true)
			exampleJsonMap.MustPush(test.jsonPath, test.value, test.indices...)

			// Check for equality between jsonMap and expected output
			testEqualityWithOut(tt, fmt.Sprintf("MustPush(%s, %v, %v)", test.jsonPath, test.value, test.indices), test.panics)
		})
	}
}

func TestMustPop(t *testing.T) {
	for testNo, test := range []struct{
		jsonPath       string
		indices        []int
		expectedOutput []interface{}
		panics         bool
		errMsg         string
	}{
		{
			"$.people",
			[]int{},
			[]interface{}{
				map[string]interface{}{
					"name": "Jeff",
					"age": float64(20),
				},
			},
			false,
			emptyString,
		},
		{
			// Shows what will happen when popping from an element that is not a []interface{} value.
			//
			// Will set the value of the pointed to key-value pair to be an empty []interface{} slice.
			"$.hello",
			[]int{},
			[]interface{}{
				"world",
			},
			false,
			emptyString,
		},
		{
			"$.*",
			[]int{},
			[]interface{}{
				"world",
			},
			false,
			emptyString,
		},
		{
			"$.does_not_exist",
			[]int{},
			[]interface{}{},
			true,
			"(-6) A JSON path could not be evaluated for the following reason(s): Key 'does_not_exist' does not exist in map",
		},
		{
			"$.people",
			[]int{2, 0, 2},
			[]interface{}{
				map[string]interface{}{
					"name": "Jeff",
					"age": float64(20),
				},
				map[string]interface{}{
					"name": "Tim",
					"age": float64(38),
				},
			},
			false,
			emptyString,
		},
	} {
		t.Run(strconv.Itoa(testNo + 1), func(tt *testing.T) {
			// Test for panics if the test is supposed to
			defer testPanic(tt, test.errMsg, test.panics)

			// Fill the example JsonMap and then run the MustPop function
			setupTest(tt, true)
			nodes := exampleJsonMap.MustPop(test.jsonPath, test.indices...)

			// Check for equality between jsonMap and expected JSON output
			testEqualityWithOut(tt, fmt.Sprintf("MustPop(%s, %v)", test.jsonPath, test.indices), test.panics)

			// Then check if the popped nodes match the expected popped nodes
			if !test.panics && !slices.SameElements(nodes, test.expectedOutput) {
				tt.Errorf("MustPop(%s, %v) pops nodes \"%v\" not \"%v\"", test.jsonPath, test.indices, nodes, test.expectedOutput)
			}
		})
	}
}
