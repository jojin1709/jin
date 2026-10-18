# Jin

Jin is a small, fast programming language designed to be as easy to write as
Python but with fewer footguns and much better performance.

## Design rules

1. **Braces, not invisible whitespace.** No broken programs because of a stray tab.
2. **One obvious way to do things.** No decorators, no metaclasses, no dunder magic.
3. **Types are optional but checked.** Annotate when it helps; the compiler catches
   mistakes before your program runs.
4. **No null surprises.** `nil` has to be handled explicitly and the compiler warns
   when it is not.
5. **Fast by default.** Source compiles to bytecode and runs on a VM written in Go.
6. **No packaging hell.** One binary and one package manager. No virtualenvs.

## Example

```jin
fn classify(n: int) -> str {
    if n % 2 == 0 {
        return "even"
    } else if n == 3 {
        return "three"
    } else {
        return "odd"
    }
}

fn main() {
    let numbers = [1, 2, 3, 4, 5]

    for n in numbers {
        print("{n} is {classify(n)}")
    }
}
```

## Usage

```sh
go build -o jin.exe .

./jin.exe examples/control.jin   # parse a file, print its syntax tree
./jin.exe tokens examples/control.jin   # print the raw token stream
./jin.exe                        # REPL
./jin.exe --version
```

## How it works

A language is built in pipeline stages. Jin is currently through stage 2:

```
source text
   |  lexer.go      = tokens
   v
tokens
   |  parser.go     = syntax tree
   v
syntax tree          (ast.go, printed by dump.go)
   |  <next: interpreter>
   v
result
```

| File | Role |
|---|---|
| `token.go` | Token types and keywords |
| `lexer.go` | Source text to tokens, with line/column tracking |
| `ast.go` | Syntax tree node definitions |
| `parser.go` | Tokens to syntax tree (Pratt parser) |
| `dump.go` | Renders a syntax tree as indented text |
| `main.go` | CLI and REPL |

## Status

- [x] Stage 1: lexer
- [x] Stage 2: parser
- [ ] Stage 3: interpreter
- [ ] Stage 4: type checker
- [ ] Stage 5: bytecode compiler + VM
- [ ] Stage 6: standard library
- [ ] Stage 7: package manager + CLI
