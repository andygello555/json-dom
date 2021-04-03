package tests

import (
	"fmt"
	"github.com/andygello555/json-dom/jom"
	"testing"
)

var exampleBytes = []byte(`
{
	person: {
		friends: [
			{
				name: Jane Doe
				age: 24
			},
			{
				name: Bob Smith
				age: 55
			},
			{
				name: Dwayne Johnson
				age: 36
			},
			{
				name: Gary Twain
				age: 40
			},
			{
				name: Elizabeth Swindon
				age: 21
			},
			{
				name: Frank Bob
			},
		],
		name: John Smith
		age: 18
	},
	forty: 40
}`)

// Stores the unmarshalled exampleBytes
var example *jom.JsonMap
var exampleMap map[string]interface{}

// Some example absolute paths to input into GetAbsolutePaths
var exampleAbsolutePathInput = [][][]interface{} {
	{
		{"forty"},
		{"person", "friends", 0},
		{"person", "friends", 1},
	},
	{
		{"person", "name"},
		{"person", "age"},
		{"person", "friends", 3},
		{"person", "friends", 4},
	},
	{
		{"person", "friends", -1},
		{"person", "children"},
		{1.2},
		{"person", 1},
	},
}

// The output of GetAbsolutePaths after inputting exampleAbsolutePathInput absolute paths
var exampleAbsolutePathOutput = [][]interface{} {
	{
		40,
		map[string]interface{} {
			"name": "Jane Doe",
			"age": 24,
		},
		map[string]interface{} {
			"name": "Bob Smith",
			"age": 55,
		},
	},
	{
		"John Smith",
		18,
		map[string]interface{}{
			"name": "Gary Twain",
			"age":  40,
		},
		map[string]interface{} {
			"name": "Elizabeth Swindon",
			"age": 21,
		},
	},
	{},
}

// Json paths evaluate using the above example
var exampleJsonPathInput = []string{
	// Property selection
	"$.person.name",
	"$.person.age",
	// Array selection
	"$.person.friends[0]",
	"$.person.friends[0, 2, 4]",
	// Recursive descent
	"$..friends[1].name",
	// Wildcard
	"$.person.friends[*]",
	"$.person.friends[0].*",
	// List slicing
	"$..friends[1:5]",
	"$..friends[1:]",
	"$..friends[:2]",
	"$..friends[-2:]",
	// Filter expressions
	//"$..friends[?(@.age==21)]",
	//"$..friends[?(@.name!='Gary Twain')]",
	//"$..friends[?(@.age>39)]",
	//"$..friends[?(@.age>=40)]",
	//"$..friends[?(@.age<30)]",
	//"$..friends[?(@.age<=24)]",
	//"$..friends[?(!@.age)]",
	//"$..friends[?(@.name=='Bob Smith' && @.age > $.forty)]",
	//"$..friends[?(@.age < 30 || @.age > $.forty)]",
}
var exampleJsonPathOutput [][]interface{}

func sameInterfaceSlice(x, y []interface{}) bool {
	if len(x) != len(y) {
		return false
	}
	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the value
		diff[fmt.Sprint(_x)]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[fmt.Sprint(_y)]; !ok {
			return false
		}
		diff[fmt.Sprint(_y)] -= 1
		if diff[fmt.Sprint(_y)] == 0 {
			delete(diff, fmt.Sprint(_y))
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}

func init() {
	// Unmarshal the exampleBytes into a JsonMap and set exampleMap as its insides
	example = jom.New()
	if err := example.Unmarshal(exampleBytes); err != nil {
		panic(err)
	}
	exampleMap = *example.GetInsides()

	// The output of evaluating the above JsonPaths on the example json map
	exampleJsonPathOutput = [][]interface{}{
		// $.person.name
		{exampleMap["person"].(map[string]interface{})["name"]},
		// $.person.age
		{exampleMap["person"].(map[string]interface{})["age"]},
		// $.person.friends[0]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[0]},
		// $.person.friends[0, 2, 4]
		{
			exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[0],
			exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[2],
			exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[4],
		},
		// $..friends[1].name
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[1].(map[string]interface{})["name"]},
		// $.person.friends[*]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})},
		// $.person.friends[0].*
		{
			exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[0].(map[string]interface{})["name"],
			exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[0].(map[string]interface{})["age"],
		},
		// $..friends[1:5]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[1:5]},
		// $..friends[1:]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[1:]},
		// $..friends[:2]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[:2]},
		// $..friends[-2:]
		{exampleMap["person"].(map[string]interface{})["friends"].([]interface{})[4:]},
	}
}

func TestAbsolutePath(t *testing.T) {
	// Iterate over all example absolute paths to input into GetAbsolutePaths
	for i, absolutePath := range exampleAbsolutePathInput {
		values, err := example.GetAbsolutePaths(&absolutePath)
		if err != nil {
			// Check if an error was expected
			if len(exampleAbsolutePathOutput[i]) != 0 {
				t.Errorf("The following error happened whilst evaluating the absolute paths %v: %v", absolutePath, err)
				continue
			}
		}

		// Check if the values returned by GetAbsolutePaths is equal to the expected values
		if !sameInterfaceSlice(values, exampleAbsolutePathOutput[i]) {
			t.Errorf("%v and %v are not equal", values, exampleAbsolutePathOutput[i])
		}
	}
}

func TestJsonPath(t *testing.T) {
	// Iterate over all example JSON path expressions and see if it matches it's expected output
	for i, jsonPath := range exampleJsonPathInput {
		nodes, err := example.JsonPathSelector(jsonPath)
		if err != nil {
			t.Errorf("The following error happened whilst evaluating the JSON path %s: %v", jsonPath, err)
			continue
		}

		// Create a new array which contains just the value of each returned JsonPathNode
		nodeVals := make([]interface{}, 0)
		for _, node := range nodes {
			nodeVals = append(nodeVals, node.Value)
		}
		// Use reflect.DeepEqual to check equality between expected and array of nodeVals
		if !sameInterfaceSlice(nodeVals, exampleJsonPathOutput[i]) {
			t.Errorf("%v and %v are not equal", nodeVals, exampleJsonPathOutput[i])
		}
	}
}
