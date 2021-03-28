# json-dom
Embedded JSON manipulation using [Hjson](https://hjson.github.io/) and Go

## What?

```javascript
{
    name: "John Smith",
    script:
        '''#//!js
        var first_last = json.name.split(' ');
        json['first_name'] = first_last[0];
        json['last_name'] = first_last[1];
        delete json.name;
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
                json.attrs.push('Married to Nick Miller (spoilers)')
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
                json.attrs.push('Married to Jessica Day (spoilers)')
                '''
        }
    ]
    // Script keys don't need to be named 'script'
    scrippidy_script:
        '''#//!js
        // This JOM is scoped to the entire JSON
        json.people.push({
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
            json['Hello'] = "World";
            '''
    },
    d:
        '''#//!js
        json['counter'] *= 3;
        '''
    a:
        '''#//!js
        json['counter'] += 6;
        '''
    c:
        '''#//!js
        json['counter'] /= 2;
        '''
    b:
        '''#//!js
        json['counter'] -= 4;
        '''
    e:
        '''#//!js
        json['counter'] *= 3;
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

If there are multiple scripts on the same level of the scope then scripts will be run in **lexicographical script-key 
order**.
