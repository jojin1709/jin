# Jin: Frequently Asked Questions

## What is Jin?
Jin is a small programming language written from scratch in Go. Programs are type-checked before they run, compiled to bytecode, and executed on Jin's own stack virtual machine. It has closures, structs with methods, try/catch and multi-file projects.

## Why build another programming language?
Mostly to learn how languages work from the inside, and to build something big enough to be a real engineering project: a lexer, parser, type checker, compiler, virtual machine, tests, documentation and a website. Honestly, nobody needs Jin today. Go, Python and other languages already do everything it does, with far larger libraries and years of testing.

## Why not just use Go or Python?
For almost anything serious, you should. Jin is written in Go, so it is not a competitor to Go; they do different jobs. Go is compiled to machine code and wants types everywhere. Jin is a scripting-style language: you run a file, and types are optional but checked when you write them. The idea is Python's feel with fewer silent mistakes: a missing dictionary key, a truthy integer in an if, or a typo that creates a global are all errors, reported before the program runs. Python's libraries are far beyond Jin's, and Jin does not try to compete there.

## Who is Jin for, and what is it useful for?
Today: learning (a small language with readable source, every stage in one repository), small scripts and command-line tools, and anyone curious about how a language is built. One direction being explored is a small embeddable scripting language for Go programs. That does not exist yet: Jin is currently a command-line program, not a Go package you can import.

## Is Jin faster than Python?
Not clearly. On the one benchmark in this repository (naive fib(30)) the Jin VM and CPython take about the same time; the README Performance section has the current numbers and how they were measured. One benchmark is not a verdict, and a faster VM is on the roadmap.

## How does it work?
Source text goes through a lexer (tokens), a parser (syntax tree), the module loader (imports), the type checker, the bytecode compiler, and finally the stack VM. Each stage is its own file. See the Architecture section of the README or the Architecture page on the website.

## Why does Jin have two engines?
Besides the bytecode VM there is a slower, simpler tree-walking interpreter. It acts as a reference: the tests run every program on both engines and require identical output, including error messages. If they ever disagree, one of them has a bug.

## How is it tested?
With a differential test suite: programs, error cases, every file in examples/ and multi-file projects are run on both engines and the results must match exactly. The README Testing section has the current test count.

## How was Jin built?
I designed the language, chose the stages and features, and directed the build one stage at a time. Much of the implementation was written with AI coding agents. The test suite, and the rule that two independent engines must agree, is what keeps it honest. There is no parser generator and there are no third-party libraries.

## What is missing?
- No remote packages. Imports are files beside the one that imports them; there is no registry, no download step, and no lock file, because there is nothing to fetch yet. A project is a folder and nothing else.
- No compiled output. `jin build` checks and compiles a project and reports what came out, but the result is not written to disk, because Jin runs from source and a stale binary is worse than no binary.
- No `finally` blocks, no custom error types, and no `try` as an expression. Catch blocks receive a `str` error message.
- No default field values, no inheritance or interfaces.
- No `switch` or `match`, and no ternary: `if` / `else if` / `else` is the only way to branch, and a branch that yields a value is written as a statement.
- No integer overflow checking. An `int` that runs past its limit wraps round, the way it does in Go.
- No `float` infinity or `NaN`: `1.0 / 0.0` is a division by zero error rather than an infinity.
- No JSON, no regular expressions and no date or time in the standard library.
- No importing from outside the folder a file is in.
- No formatter, no test runner and no language server yet. `gofmt` has no Jin equivalent, and an editor cannot ask Jin what a name means.
- Calls are limited to 1024 frames deep, so a program that recurses further than that is stopped with an error rather than a crash.

The README roadmap shows what is planned.

## Is Jin production-ready?
No. It is a working language with documented limits and a solid test suite, but it has few users, no stability promise yet and a small standard library. Do not run anything important on it.

## How do I install it?
See the Install section of the README (or the Getting started page on the website).

## Can I use Jin from inside a Go program?
Not yet. It is a possible future direction; see the answer above about who Jin is for.

## How can I report a bug or help?
Open an issue on GitHub with the program, the output you got and the output you expected. Pull requests for small fixes are welcome; for bigger changes please open an issue first.

## Why is it called Jin?
Short, easy to type, and not already taken by a language you would confuse it with. It is also two letters away from "Jin" being the only thing the compiler needs to know about you.
