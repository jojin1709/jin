package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const version = "0.0.1 (stage 1: lexer)"

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "-v", "--version", "version":
			fmt.Printf("Jin %s\n", version)
			return
		}
		runFile(args[0])
		return
	}

	repl()
}

// runFile tokenizes a .jin file and prints the token stream.
func runFile(path string) {
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

// repl reads Jin source line by line and prints the tokens it finds.
func repl() {
	fmt.Printf("Jin %s\n", version)
	fmt.Println("Type Jin code, then press Enter. 'exit' or Ctrl+D quits.")
	fmt.Println("(Stage 1 prints the token stream instead of running code.)")

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("jin> ")

	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "exit" {
			break
		}

		lex := NewLexer(line)
		for {
			tok := lex.NextToken()
			fmt.Printf("  %-8s %s\n", tok.Type, describe(tok))
			if tok.Type == EOF {
				break
			}
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
