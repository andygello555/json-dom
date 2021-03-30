# json-dom
Embedded JSON manipulation using [Hjson](https://hjson.github.io/) and Go

## What?

```javascript
{
    name: "John Smith",
    script:
        '''#//!js
        var first_last = json.trail.name.split(' ');
        json.trail['first_name'] = first_last[0];
        json.trail['last_name'] = first_last[1];
        delete json.trail.name;
        '''
}
```

Embed Javascript into your hjson and start tweaking that JSON. This **allows you to do anything that you would** from 
within your frontend, backend, *anywhere* from directly within your JSON. The above example evaluates to the following 
JSON:

```json
{
    "first_name": "John",
    "last_name": "Smith"
}
```

## How?

### CLI

*TODO*

### Go Package

To use the API run:
1. `go get -u github.com/andygello555/json-dom`: Download the `json-dom` src
2. `import github.com/andygello555/json-dom/jom`: Import `jom` package

### Example usage

```go
package main

import (
  "github.com/andygello555/json-dom/jom"
  "fmt"
)

func main() {

    // The hjson to evaluate
    sampleText := []byte(`
    {
        name: John Smith,
        script:
            '''#//!js
            var first_last = json.trail.name.split(' ');
            json.trail['first_name'] = first_last[0];
            json.trail['last_name'] = first_last[1];
            delete json.trail.name;
            '''
    }`)

    // Create map to keep decoded data
    jsonMap := jom.New()

    // Unmarshal into the JsonMap
    err := jsonMap.Unmarshal(sampleText)
    if err != nil {
        panic(err)
    }

    // To evaluate the JsonMap call the .Run() method
    jsonMap.Run()

    // Then we can marshal the JsonMap back into a JSON byte string
    if out, err := jsonMap.Marshal(); err != nil {
    	panic(err)
    } else {
        fmt.Println(string(out))
    }
}
```

### Scope

Similar to DOM manipulation a builtin variable is parsed to all your scripts with an object representing the current 
*scope*

```javascript
{
    people: [
        {
            name: "Jessica Day",
            attrs: [
                "Bug eyes",
                "Black hair"
            ],
            script:
                '''#//!js
                // This JOM is scoped to just this object
                json.trail.attrs.push('Married to Nick Miller (spoilers)')
                '''
        },
        {
            name: "Nick Miller",
            attrs: [
                "Heavy drinker",
                "Writer"
            ],
            script:
                '''#//!js
                // ... same with this one
                json.trail.attrs.push('Married to Jessica Day (spoilers)')
                '''
        }
    ]
    // Script keys don't need to be named 'script'
    scrippidy_script:
        '''#//!js
        // This JOM is scoped to the entire JSON
        json.trail.people.push({
            name: "Winston Bishop",
            attrs: [
                "Ferguson",
                "Married to Ally (spoilers)"
            ]
        })
        '''
}
```

Evaluates to...

```json
{
    "people": [
        {
            "name": "Jessica Day",
            "attrs": [
                "Anime eyes",
                "Black hair",
                "Married to Nick Miller (spoilers)"
            ]
        },
        {
            "name": "Nick Miller",
            "attrs": [
                "Heavy drinker",
                "Writer",
                "Married to Jessica Day (spoilers)"
            ]
        },
        {
            "name": "Winston Bishop",
            "attrs": [
              "Ferguson",
              "Married to Ally (spoilers)"
            ]
        }
    ]
}
```

### Order execution

**What about _multiple_ script tags that share the same scope?**

```javascript
{
    // Demonstrate how multiple scripts on the same level are executed lexicographically.
    // Counter should be evaluated as 9.
    counter: 0,
    nested_boi: {
        script:
            '''#//!js
            json.trail['Hello'] = "World";
            '''
    },
    d:
        '''#//!js
        json.trail['counter'] *= 3;
        '''
    a:
        '''#//!js
        json.trail['counter'] += 6;
        '''
    c:
        '''#//!js
        json.trail['counter'] /= 2;
        '''
    b:
        '''#//!js
        json.trail['counter'] -= 4;
        '''
    e:
        '''#//!js
        json.trail['counter'] *= 3;
        '''
}
```

Will be evaluated as...

```json
{
    "counter": 9,
    "nested_boi": {
        "Hello": "World"
    }
}
```

The only two rules:
- Scripts are run **level-by-level**.
- If there are multiple scripts on the same level of the scope then scripts will be run in **lexicographical script-key
order**.

## Available languages

At the moment the only available language to write scripts in is Javascript via [otto](https://pkg.go.dev/github.com/robertkrimen/otto)

### Javascript

Scripts are run using the [otto](https://pkg.go.dev/github.com/robertkrimen/otto) interpreter available for Go. Therefore, all the caveats that exist within otto also exist when using Javascript in json-dom. Among other [things](https://pkg.go.dev/github.com/robertkrimen/otto#hdr-Caveat_Emptor) the following caveats exist:
- "use strict" will parse but does nothing.
- The regular expression engine (re2/regexp) is not fully compatible with the ECMA5 specification.
  - Lookaheads
  - Lookbehinds
  - Back-references
- Otto targets ES5. ES6 features are not supported.
  - Typed arrays
  - `let` and `const` variable definitions
- Although not really a caveat: scripts that run for over `utils.HaltingDelay` seconds will terminate to avoid the **halting problem**
- `setInterval` and `setTimeout` are not supported and will probably never be supported
  - **json-dom was designed to be non-blocking**

#### Builtin functions

| Name              | Params      | Returns | Description                     |
|:------------------|:------------|:--------|:--------------------------------|
| `printlnExternal` | `...Object` | Nothing | Legacy version of `console.log` |
| `console.log`     | `...Object` | Nothing | Will print to stdout            |
| `console.error`   | `...Object` | Nothing | Will print to stderr            |

#### Builtin symbols

| Name              | Type   | Description                                                                                                                                                                                                                    |
|:------------------|:-------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `json`            | Object | The JOM. Aka. the JSON Document. Contains helpers and properties to aid with JSON manipulation.                                                                                                                                |
| `json.trail`      | Object | Contains a replica of the key-value pairs of the JSON at the script's scope level. Any changes to it will be reflected in the final output JSON.                                                                               |
| `json.scriptPath` | String | The JSON path to the current scope. Mostly just used by the `console` object to print out where a print came from.                                                                                                             |
| `console`         | Object | The standard `console` object you know and love. Currently, the only supported methods are `log` and `error`. Both print the JSON path location to the call. The former will print to stdout. The latter will print to stderr. |
