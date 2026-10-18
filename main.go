package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const version = "0.2.0 (stage 2: parser)"

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
	case "tokens":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "jin: 'tokens' needs a file")
			os.Exit(1)
		}
		printTokens(args[1])
		return
	case "-h", "--help", "help":
		usage()
		return
	}

	printAST(args[0])
}

func usage() {
	fmt.Println(`Jin - a small fast language

Usage:
  jin <file.jin>       parse a file and print its syntax tree
  jin tokens <file>    print the raw token stream (stage 1 view)
  jin                  start the REPL
  jin --version        print the version`)
}

// parseFile reads and parses a file, reporting errors to stderr.
func parseFile(path string) (*Program, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jin: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}
	p := NewParser(NewLexer(string(data)))
	program := p.ParseProgram()
	return program, p.Errors()
}

func printAST(path string) {
	program, errs := parseFile(path)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "parse errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Print(DumpProgram(program))
}

func printTokens(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jin: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}

	lex := NewLexer(string(data))
	for {
		tok := lex.NextToken()
		fmt.Printf("%d:%d\t%-8s %s\n", tok.Line, tok.Column, tok.Type, describe(tok))
		if tok.Type == EOF {
			return
		}
	}
}

func repl() {
	fmt.Printf("Jin %s\n", version)
	fmt.Println("Type Jin code, then press Enter. 'exit' or Ctrl+D quits.")
	fmt.Println("(Stages 1 and 2 only: this parses code and prints the tree.)")

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("jin> ")

	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "exit" {
			break
		}
		if strings.TrimSpace(line) == "" {
			fmt.Print("jin> ")
			continue
		}

		p := NewParser(NewLexer(line))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				fmt.Printf("  error: %s\n", e)
			}
		} else {
			fmt.Print(DumpProgram(program))
		}

		fmt.Print("jin> ")
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
