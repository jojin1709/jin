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
fn greet(name: str) -> str {
    return "Hello, {name}!"
}

let numbers = [1, 2, 3, 4, 5]

for n in numbers {
    if n % 2 == 0 {
        print(greet("friend #{n}"))
    }
}
```

## Build and run

```sh
go build -o jin.exe .
./jin.exe examples/hello.jin   # print the token stream
./jin.exe                      # start the REPL
go test ./...                  # run the test suite
```

## Status

- [x] Stage 1: lexer
- [ ] Stage 2: parser
- [ ] Stage 3: interpreter
- [ ] Stage 4: type checker
- [ ] Stage 5: bytecode compiler + VM
- [ ] Stage 6: standard library
- [ ] Stage 7: package manager + CLI
