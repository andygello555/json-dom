package jom

import (
	"fmt"
	_ "github.com/andygello555/json-dom/code/go"
	_ "github.com/andygello555/json-dom/code/js"
	"github.com/andygello555/json-dom/jom/json_map"
)

// How to create a new JSON map which contains a script and how to mark it up with other script types.
func Example()  {
	jsonMap := New()
	err := jsonMap.Unmarshal([]byte(`
	{
		hello: world
		script: 
			'''#//!js
			json.trail['hello'] += '/js';
			'''
	}
	`))
	if err != nil {
		panic(err)
	}

	// We evaluate the JOM to run the JS script in the "script" key.
	jsonMap.Run()

	// We can use the MustSet/JsonPathSetter function to set a callback within the JOM that will run natively in Go.
	// Note: Go callbacks and scripts written as strings directly within the JOM cannot be executed at the same time
	//       due to the way the JOM is serialised and passed to the JS VM.
	jsonMap.MustSet("$.script", func(json json_map.JsonMapInt) {
		insides := json.GetInsides()
		(*insides)["hello"] = (*insides)["hello"].(string) + "/go"
	})

	// We evaluate the JOM to run the Go callback in the "script" key.
	jsonMap.Run()

	// The String implementation of JsonMap will Marshall the JOM to hjson, then convert to a string.
	fmt.Println(jsonMap)
	// Output:
	// {
	//   hello: world/js/go
	// }
}

// Eval takes a JOM in the form of hjson byte array, runs all scripts within it and returns the evaluated byte array as
// JSON (not hjson).
//
// Ideal for evaluating a JOM read from a file and saving to another file.
func ExampleEval() {
	out, _ := Eval([]byte(`
	{
		eval: ""
		script:
			'''#//!js
			json.trail["eval"] = "does this!";
			'''
	}
	`), false)
	fmt.Println(string(out))
	// Output:
	// {"eval":"does this!"}
}

// Deleting a key from a JSON map using a JSON path.
func ExampleJsonMap_MustDelete() {
	jsonMap := New()
	_ = jsonMap.Unmarshal([]byte(`
	{
		hello: world
		friends: [
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

	// Deleting the "hello" key-value pair pointed to by "$.hello"
	jsonMap.MustDelete("$.hello")

	fmt.Println(jsonMap)
	// Output:
	// {
	//   friends:
	//   [
	//     {
	//       age: 20
	//       name: Jeff
	//     }
	//     {
	//       age: 24
	//       name: Bob
	//     }
	//     {
	//       age: 38
	//       name: Tim
	//     }
	//   ]
	// }
}

// Getting a key from a JSON map using a JSON path.
func ExampleJsonMap_MustGet() {
	jsonMap := New()
	_ = jsonMap.Unmarshal([]byte(`
	{
		hello: world
		friends: [
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

	// Getting the "hello" key-value pair pointed to by "$.hello"
	nodes := jsonMap.MustGet("$.hello")

	fmt.Println(nodes[0].(string))
	// Output:
	// world
}

// Setting a key from a JSON map using a JSON path.
func ExampleJsonMap_MustSet() {
	jsonMap := New()
	_ = jsonMap.Unmarshal([]byte(`
	{
		hello: world
		friends: [
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

	// Setting the "hello" key-value pair pointed to by "$.hello" to "me"
	jsonMap.MustSet("$.hello", "me")

	fmt.Println(jsonMap)
	// Output:
	// {
	//   friends:
	//   [
	//     {
	//       age: 20
	//       name: Jeff
	//     }
	//     {
	//       age: 24
	//       name: Bob
	//     }
	//     {
	//       age: 38
	//       name: Tim
	//     }
	//   ]
	//   hello: me
	// }
}

// Pushing a new friend to "friends" array.
func ExampleJsonMap_MustPush() {
	jsonMap := New()
	_ = jsonMap.Unmarshal([]byte(`
	{
		hello: world
		friends: [
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

	// Pushing a new map[string]interface{} to "$.friends" array
	jsonMap.MustPush("$.friends", map[string]interface{} {
		"name": "David",
		"age": 32,
	})

	fmt.Println(jsonMap)
	// Output:
	// {
	//   friends:
	//   [
	//     {
	//       age: 20
	//       name: Jeff
	//     }
	//     {
	//       age: 24
	//       name: Bob
	//     }
	//     {
	//       age: 38
	//       name: Tim
	//     }
	//     {
	//       age: 32
	//       name: David
	//     }
	//   ]
	//   hello: world
	// }
}

// Popping a friend from the "friends" array.
func ExampleJsonMap_MustPop() {
	jsonMap := New()
	_ = jsonMap.Unmarshal([]byte(`
	{
		hello: world
		friends: [
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

	// Popping the head of the "$.friends" array.
	popped := jsonMap.MustPop("$.friends")

	fmt.Println(popped[0].(map[string]interface{}))
	fmt.Println(jsonMap)
	// Output:
	// map[age:20 name:Jeff]
	// {
	//   friends:
	//   [
	//     {
	//       age: 24
	//       name: Bob
	//     }
	//     {
	//       age: 38
	//       name: Tim
	//     }
	//   ]
	//   hello: world
	// }
}
