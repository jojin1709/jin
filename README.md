# Jin

Jin is a small, fast programming language designed to be as easy to write as
Python but with fewer footguns and much better performance.

## Design rules

1. **Braces, not invisible whitespace.** No broken programs because of a stray tab.
2. **One obvious way to do things.** No decorators, no metaclasses, no dunder magic.
3. **No silent type mixing.** `1 + 1.5` is an error, not a surprise. Convert with `int()` or `float()`.
4. **Conditions must be `bool`.** `if 1 { }` is rejected instead of quietly being truthy.
5. **No accidental globals.** Assigning to a name that was never declared is an error.
6. **Fast by default.** Source compiles to a tree-walked form today and a bytecode VM later.

## Example

```jin
fn fib(n) {
    if n < 2 { return n }
    return fib(n - 1) + fib(n - 2)
}

let fibs = []
for i in range(10) {
    fibs = push(fibs, fib(i))
}
print("fib: {join(fibs, \", \")}")
```

Output:

```
fib: 0, 1, 1, 2, 3, 5, 8, 13, 21, 34
```

## Usage

```sh
go build -o jin.exe .

./jin.exe examples/tour.jin       # run a program
./jin.exe examples/control.jin    # run the control-flow demo
./jin.exe parse examples/tour.jin # print the syntax tree
./jin.exe tokens examples/tour.jin # print the token stream
./jin.exe                          # REPL
go test ./...
```

Stage 1 and 2 behaviour (`tokens`, `parse`) is still available, so you can watch
each layer of the pipeline separately.

## Language tour

```jin
# comments start with a hash

let x = 5                  # let declares
let name = "Jin"
x = x + 1                  # assign to an existing name

fn add(a, b) { return a + b }        # implicit return also works: fn sq(x) { x * x }

if x > 5 { print("big") } else if x == 5 { print("five") } else { print("small") }

for n in [1, 2, 3] { print(n) }      # also: for ch in "abc", for i in range(10)
while x > 0 { x = x - 1; if x == 2 { break } }

let f = fn(v) { v * 2 }              # anonymous functions
print("{name} has {len(name)} letters")   # string interpolation
print("use {{braces}} for literal braces")
```

Built-ins: `print`, `len`, `push`, `str`, `int`, `float`, `type`, `range`,
`upper`, `lower`, `trim`, `split`, `join`, `contains`, `abs`, `min`, `max`.

## How it works

A language is built as a pipeline. Jin is through stage 3:

```
source text
   |  lexer.go        -> tokens
   v
tokens
   |  parser.go       -> syntax tree
   v
syntax tree            (ast.go)
   |  interpreter.go  -> result
   v
values                 (object.go, environment.go, builtins.go)
```

| File | Role |
|---|---|
| `token.go` | Token types and keywords |
| `lexer.go` | Source text to tokens, with line/column tracking |
| `ast.go` | Syntax tree node definitions |
| `parser.go` | Tokens to syntax tree (Pratt parser) |
| `dump.go` | Renders a syntax tree as indented text |
| `object.go` | Runtime values |
| `environment.go` | Variable scopes |
| `builtins.go` | Built-in functions |
| `interpreter.go` | Evaluates the tree |
| `main.go` | CLI and REPL |

## Status

- [x] Stage 1: lexer
- [x] Stage 2: parser
- [x] Stage 3: interpreter
- [ ] Stage 4: type checker
- [ ] Stage 5: bytecode compiler + VM
- [ ] Stage 6: standard library
- [ ] Stage 7: package manager + CLI

## License

MIT
