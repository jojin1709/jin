package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const version = "0.6.1 (stage 6: shared captures)"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		repl()
		return
	}

	switch args[0] {
	case "-v", "--version", "version":
		fmt.Printf("Jin %s\n", version)
		return
	case "-h", "--help", "help":
		usage()
		return
	case "tokens":
		requireFile(args, "tokens")
		printTokens(args[1])
		return
	case "parse":
		requireFile(args, "parse")
		printAST(args[1])
		return
	case "check":
		requireFile(args, "check")
		checkFile(args[1])
		return
	case "compile":
		requireFile(args, "compile")
		compileFile(args[1])
		return
	case "interpret":
		requireFile(args, "interpret")
		interpretFile(args[1])
		return
	case "run":
		requireFile(args, "run")
		runFile(args[1])
		return
	}

	runFile(args[0])
}

func usage() {
	fmt.Println(`Jin - a small fast language

Usage:
  jin <file.jin>       run a program
  jin check <file>     type-check a program without running it
  jin compile <file>   print the bytecode instead of running
  jin interpret <file> run with the tree-walking interpreter
  jin parse <file>     print the syntax tree instead of running
  jin tokens <file>    print the raw token stream
  jin                  start the REPL
  jin --version        print the version`)
}

func requireFile(args []string, cmd string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "jin: '%s' needs a file, e.g. jin %s program.jin\n", cmd, cmd)
		os.Exit(1)
	}
}

func readSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jin: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}
	return string(data)
}

// parseProgram reads and parses a file, exiting with a report on syntax errors.
func parseProgram(path string) *Program {
	p := NewParser(NewLexer(readSource(path)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		fmt.Fprintf(os.Stderr, "jin: %s has syntax errors:\n", path)
		for _, e := range p.Errors() {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		os.Exit(1)
	}
	return program
}

// reportTypeErrors prints type errors, if any, and reports whether there were
// any. Errors are printed all at once rather than one per run.
func reportTypeErrors(errs []string, label string) bool {
	if len(errs) == 0 {
		return false
	}
	fmt.Fprintf(os.Stderr, "jin: %s has type errors:\n", label)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  %s\n", e)
	}
	return true
}

// checkFile type-checks a program and reports the result without running it.
func checkFile(path string) {
	program := parseProgram(path)
	if reportTypeErrors(Check(program), path) {
		os.Exit(1)
	}
	fmt.Printf("%s: no type errors\n", path)
}

// runFile executes a program on the bytecode VM.
func runFile(path string) {
	program := parseProgram(path)

	if reportTypeErrors(Check(program), path) {
		os.Exit(1)
	}

	vm := NewVM(os.Stdout)
	bc, err := compileProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jin: %v\n", err)
		os.Exit(1)
	}

	if _, err := vm.Run(bc); err != nil {
		fmt.Fprintf(os.Stderr, "jin: %v\n", err)
		os.Exit(1)
	}
}

// interpretFile executes a program with the tree-walking interpreter, which is
// slower but useful for checking the two back ends agree.
func interpretFile(path string) {
	program := parseProgram(path)

	if reportTypeErrors(Check(program), path) {
		os.Exit(1)
	}

	interp := NewInterpreter(os.Stdout)
	if _, err := interp.Run(program, NewEnvironment()); err != nil {
		fmt.Fprintf(os.Stderr, "jin: %v\n", err)
		os.Exit(1)
	}
}

// compileProgram compiles a program with a fresh global table.
func compileProgram(program *Program) (*Bytecode, error) {
	return NewCompiler(NewSymbolTable(true)).Compile(program)
}

// compileFile prints the bytecode a program compiles to.
func compileFile(path string) {
	program := parseProgram(path)

	if reportTypeErrors(Check(program), path) {
		os.Exit(1)
	}

	bc, err := compileProgram(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jin: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("== constants ==")
	for i, c := range bc.Constants {
		fmt.Printf("%04d %s\n", i, c.Inspect())
	}
	fmt.Printf("== <script> (locals: %d) ==\n", bc.NumLocals)
	fmt.Print(bc.Instructions)
}

// printAST shows the syntax tree without running anything.
func printAST(path string) {
	fmt.Print(DumpProgram(parseProgram(path)))
}

// printTokens shows the raw token stream.
func printTokens(path string) {
	lex := NewLexer(readSource(path))
	for {
		tok := lex.NextToken()
		fmt.Printf("%d:%d\t%-8s %s\n", tok.Line, tok.Column, tok.Type, describe(tok))
		if tok.Type == EOF {
			return
		}
	}
}

// repl is an interactive session. State persists between lines, and an
// incomplete line is buffered until it parses, so multi-line functions work.
func repl() {
	fmt.Printf("Jin %s\n", version)
	fmt.Println("Type Jin code and press Enter. 'exit' or Ctrl+D quits.")
	fmt.Println("'clear' discards a half-typed block.")

	vm := NewVM(os.Stdout)
	compiler := NewCompiler(NewSymbolTable(true))
	checker := NewChecker()
	sc := bufio.NewScanner(os.Stdin)
	var pending []string

	for {
		if len(pending) == 0 {
			fmt.Print("jin> ")
		} else {
			fmt.Print("  .. ")
		}

		if !sc.Scan() {
			break
		}
		line := sc.Text()
		trimmed := strings.TrimSpace(line)

		if len(pending) == 0 {
			if trimmed == "exit" {
				break
			}
			if trimmed == "" {
				continue
			}
			if trimmed == "clear" {
				fmt.Println("(nothing pending)")
				continue
			}
		}

		source := strings.Join(append(append([]string{}, pending...), line), "\n")
		p := NewParser(NewLexer(source))
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			// Hold the text back: it may just be an unfinished block.
			pending = append(pending, line)
			continue
		}

		// In the REPL, a type error is shown but the prompt continues, so one
		// bad line does not end the session. Types declared by the failing line
		// are rolled back so they cannot affect later lines.
		before := checker.GlobalScope()
		if errs := checker.CheckProgram(program); len(errs) > 0 {
			fmt.Println("  type errors:")
			for _, e := range errs {
				fmt.Printf("    %s\n", e)
			}
			checker.SetGlobalScope(before)
			pending = nil
			continue
		}

		bc, err := compiler.Compile(program)
		if err != nil {
			fmt.Printf("  error: %v\n", err)
			pending = nil
			continue
		}

		result, err := vm.Run(bc)
		pending = nil

		if err != nil {
			fmt.Printf("  error: %v\n", err)
			continue
		}
		if _, isNil := result.(*Nil); !isNil {
			if _, isFn := result.(*Closure); !isFn {
				fmt.Println(result.Inspect())
			}
		}
	}

	fmt.Println()
}

// describe renders a token's literal for display.
func describe(t Token) string {
	switch t.Type {
	case EOF:
		return ""
	case STRING:
		return fmt.Sprintf("%q", t.Literal)
	default:
		return t.Literal
	}
}
