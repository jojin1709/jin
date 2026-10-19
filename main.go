package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const version = "0.9.0 (stage 9: error handling)"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		// In a project, `jin` runs it; anywhere else it starts the REPL.
		if dir, ok := FindProject("."); ok {
			runSource(loadProject(dir), nil)
			return
		}
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
	case "new":
		if len(args) < 2 {
			fatal("'new' needs a name, e.g. jin new hello")
		}
		newProject(args[1])
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
		checkSource(loadTarget(firstPath(args)))
		return
	case "build":
		buildSource(loadTarget(firstPath(args)))
		return
	case "compile":
		requireFile(args, "compile")
		compileTarget(args[1])
		return
	case "interpret":
		target, programArgs := splitTarget(args[1:])
		interpretSource(loadTarget(target), programArgs)
		return
	case "run":
		target, programArgs := splitTarget(args[1:])
		runSource(loadTarget(target), programArgs)
		return
	}

	// `jin program.jin ...` is the short form of `jin run`.
	target, programArgs := splitTarget(args)
	runSource(loadTarget(target), programArgs)
}

func usage() {
	fmt.Println(`Jin - a small fast language

Usage:
  jin                       run the project here, or start the REPL
  jin <file.jin> [args]     run a program
  jin run [file|folder] [args]
                            run a program or a project
  jin check [file|folder]   type-check without running
  jin build [folder]        check and compile a whole project
  jin compile <file>        print the bytecode instead of running
  jin interpret <file>      run with the tree-walking interpreter
  jin parse <file>          print the syntax tree instead of running
  jin tokens <file>         print the raw token stream
  jin new <name>            start a project in <name>/
  jin --version             print the version

A folder is a project when it holds a jin.toml. Anything after the file name is
passed to the program, where args() returns it:`)
	fmt.Println(`
  jin greet.jin hello world`)
}

// ---------------------------------------------------------------- commands

// source is what a command works on: one file, or a project's entry file with
// everything that file imports.
type source struct {
	Module   *Module
	Label    string // how the thing is named in messages
	Manifest *Manifest
}

// loadTarget turns a file or folder into something ready to check and run. An
// empty target means the project in the current folder.
func loadTarget(target string) *source {
	if target == "" {
		dir, ok := FindProject(".")
		if !ok {
			fatal("no %s here; name a file, or start one with 'jin new'", manifestName)
		}
		return loadProject(dir)
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fatal("there is no %s", target)
		}
		fatal("cannot read %s: %v", target, err)
	}

	if !info.IsDir() {
		m, err := NewLoader().LoadFile(target)
		if err != nil {
			fatal("%v", err)
		}
		return &source{Module: m, Label: target}
	}

	dir, ok := FindProject(target)
	if !ok {
		fatal("%s is a folder, but it has no %s, so it is not a project", target, manifestName)
	}
	return loadProject(dir)
}

// loadProject loads a project folder: its manifest, and its entry file with
// everything that imports.
func loadProject(dir string) *source {
	m, manifest, err := NewLoader().LoadProject(dir)
	if err != nil {
		fatal("%v", err)
	}
	return &source{Module: m, Label: manifest.Label(), Manifest: manifest}
}

// splitTarget reads the file or folder to run, and the arguments meant for the
// program itself. A leading -- separates the two.
func splitTarget(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	if args[0] == "--" {
		return "", args[1:]
	}
	return args[0], stripSeparator(args[1:])
}

// firstPath is the single optional file or folder a command takes.
func firstPath(args []string) string {
	if len(args) < 2 {
		return ""
	}
	return args[1]
}

// stripSeparator drops the -- that separates a program from its arguments.
func stripSeparator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// runSource checks a file or project, compiles it, and runs it on the bytecode
// VM.
func runSource(src *source, programArgs []string) {
	checkProject(src)

	bc, err := NewCompiler(NewSymbolTable(true)).CompileModules(src.Module)
	if err != nil {
		fatal("%v", err)
	}

	vm := NewVMWithArgs(os.Stdout, programArgs)
	if _, err := vm.Run(bc); err != nil {
		fatal("%v", err)
	}
}

// interpretSource runs the same thing on the tree-walking interpreter, which is
// slower but useful for checking that the two back ends agree.
func interpretSource(src *source, programArgs []string) {
	checkProject(src)

	interp := NewInterpreterWithArgs(os.Stdout, programArgs)
	if _, err := interp.RunModules(src.Module, NewEnvironment()); err != nil {
		fatal("%v", err)
	}
}

// checkProject type-checks everything a file or project reaches, and stops if
// there is anything wrong.
func checkProject(src *source) {
	if reportTypeErrors(CheckProject(src.Module), src.Label) {
		os.Exit(1)
	}
}

// checkSource type-checks without running anything.
func checkSource(src *source) {
	checkProject(src)
	if len(src.Module.Deps) == 0 {
		fmt.Printf("%s: no type errors\n", src.Label)
		return
	}
	fmt.Printf("%s: no type errors in %d file(s)\n", src.Label, 1+len(src.Module.Deps))
}

// buildSource checks and compiles a whole project at once, which is what a
// build is here: Jin runs from source, so there is no separate binary to write.
func buildSource(src *source) {
	checkProject(src)

	bc, err := NewCompiler(NewSymbolTable(true)).CompileModules(src.Module)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("jin build: %s\n", src.Label)
	if src.Manifest != nil {
		fmt.Printf("  entry: %s\n", src.Manifest.Entry)
	}
	fmt.Printf("  files: %s\n", strings.Join(fileNames(src), ", "))
	fmt.Printf("  code:  %d bytes of bytecode, %d constants, %d locals\n",
		len(bc.Instructions), len(bc.Constants), bc.NumLocals)
	fmt.Println("  ok")
}

// fileNames lists every file a program is made of, the entry first.
func fileNames(src *source) []string {
	names := make([]string, 0, len(src.Module.Deps)+1)
	if src.Manifest != nil {
		names = append(names, src.Manifest.Entry)
	} else {
		names = append(names, src.Label)
	}
	for _, dep := range src.Module.Deps {
		names = append(names, dep.Name)
	}
	return names
}

// compileProgram compiles a program with a fresh global table. It is the
// single-file form of a build, and is what the tests use.
func compileProgram(program *Program) (*Bytecode, error) {
	return NewCompiler(NewSymbolTable(true)).Compile(program)
}

// compileTarget prints the bytecode a file and everything it imports compiles
// to.
func compileTarget(path string) {
	src := loadTarget(path)
	checkProject(src)

	bc, err := NewCompiler(NewSymbolTable(true)).CompileModules(src.Module)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Println("== constants ==")
	for i, c := range bc.Constants {
		fmt.Printf("%04d %s\n", i, c.Inspect())
	}
	fmt.Printf("== <script> (locals: %d) ==\n", bc.NumLocals)
	fmt.Print(bc.Instructions)
}

// newProject starts a project in a new folder.
func newProject(name string) {
	if strings.ContainsAny(name, `/\:`) || name == "." || name == ".." {
		fatal("%q is not a folder name for a project", name)
	}
	if err := NewProject(name, name); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("created %s/\n", name)
	fmt.Println("  jin.toml")
	fmt.Println("  main.jin")
	fmt.Println("  greeting.jin")
	fmt.Printf("\nnext: cd %s && jin run\n", name)
}

// ---------------------------------------------------------------- reporting

// fatal reports a problem and stops, the way a command-line tool should.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "jin: %s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}

func requireFile(args []string, cmd string) {
	if len(args) < 2 {
		fatal("'%s' needs a file, e.g. jin %s program.jin", cmd, cmd)
	}
}

func readSource(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("cannot read %s: %v", path, err)
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

// ---------------------------------------------------------------- repl

// repl is an interactive session. State persists between lines, and an
// incomplete line is buffered until it parses, so multi-line functions work.
func repl() {
	fmt.Printf("Jin %s\n", version)
	fmt.Println("Type Jin code and press Enter. 'exit' or Ctrl+D quits.")
	fmt.Println("'clear' discards a half-typed block.")

	vm := NewVM(os.Stdout)
	compiler := NewCompiler(NewSymbolTable(true))
	checker := NewChecker()
	loader := NewLoader()
	checked := map[*Module]map[string]*Type{}
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

		// Whatever the line imports is read from the folder the REPL was
		// started in, so a session can reach the files beside it.
		mod, err := loader.LoadStatements(program, ".")
		if err != nil {
			fmt.Printf("  error: %v\n", err)
			pending = nil
			continue
		}

		// In the REPL, a type error is shown but the prompt continues, so one
		// bad line does not end the session. Types declared by the failing line
		// are rolled back so they cannot affect later lines.
		before := checker.GlobalScope()
		if errs := checker.CheckModules(mod, checked); len(errs) > 0 {
			fmt.Println("  type errors:")
			for _, e := range errs {
				fmt.Printf("    %s\n", e)
			}
			checker.SetGlobalScope(before)
			pending = nil
			continue
		}

		bc, err := compiler.CompileModules(mod)
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
		// A declaration produces a value, but showing a function or a type
		// back at you says nothing useful, so only real results are printed.
		switch result.(type) {
		case nil, *Nil, *Closure, *StructType:
		default:
			fmt.Println(result.Inspect())
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
