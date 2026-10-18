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
./jin.exe examples/types.jin      # run the type-checked demo
./jin.exe check examples/tour.jin # type-check without running
./jin.exe parse examples/tour.jin # print the syntax tree
./jin.exe tokens examples/tour.jin # print the token stream
./jin.exe                          # REPL
go test ./...
```

Stages 1, 2 and 4 (`tokens`, `parse`, `check`) are available on their own, so
you can watch each layer of the pipeline separately.

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

## Types are optional, but checked

Annotations are never required. When you write them, Jin enforces them before
the program runs, so mistakes are caught up front instead of halfway through a
loop:

```jin
fn area(w: int, h: int) -> int {
    return w * h
}

let width: int = 3
let height = "4"

print(area(width, height))   # error: argument 2: cannot use str where int is expected
print(area(width))           # error: area() expects 2 argument(s), got 1
print(1 + "x")               # error: cannot add int and str
if width { }                 # error: condition must be a bool, got int
```

Run `jin check file.jin` to see the problems without executing anything. Running
a file checks it first, so a program with type errors never starts.

Whatever you leave unannotated is treated as `any` and left alone, so plain
scripts keep working exactly as before. The checker also covers `let` and
assignment, function arguments and returns, loop bodies, indexing, built-in
calls, misplaced `break` / `continue` / `return`, and the expressions inside
`{interpolation}`.

## How it works

A language is built as a pipeline. Jin is through stage 4:

```
source text
   |  lexer.go        -> tokens
   v
tokens
   |  parser.go       -> syntax tree
   v
syntax tree            (ast.go)
   |  typechecker.go  -> type errors, or a clean bill of health
   v
syntax tree
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
| `typechecker.go` | Static types, and every check that runs before the program does |
| `template.go` | Splits `{interpolation}` inside strings, shared by the checker and interpreter |
| `object.go` | Runtime values |
| `environment.go` | Variable scopes |
| `builtins.go` | Built-in functions |
| `interpreter.go` | Evaluates the tree |
| `main.go` | CLI and REPL |

## Status

- [x] Stage 1: lexer
- [x] Stage 2: parser
- [x] Stage 3: interpreter
- [x] Stage 4: type checker
- [ ] Stage 5: bytecode compiler + VM
- [ ] Stage 6: standard library
- [ ] Stage 7: package manager + CLI

## License

MIT
