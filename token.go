package main

import (
	"fmt"
	"strings"
)

// TokenType identifies the kind of a token.
type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Identifiers and literals
	IDENT  TokenType = "IDENT"
	INT    TokenType = "INT"
	FLOAT  TokenType = "FLOAT"
	STRING TokenType = "STRING"

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"
	PERCENT  TokenType = "%"
	BANG     TokenType = "!"

	LT     TokenType = "<"
	GT     TokenType = ">"
	LE     TokenType = "<="
	GE     TokenType = ">="
	EQ     TokenType = "=="
	NOT_EQ TokenType = "!="
	AND    TokenType = "&&"
	OR     TokenType = "||"
	AMP    TokenType = "&"
	PIPE   TokenType = "|"

	// Delimiters
	COMMA     TokenType = ","
	SEMICOLON TokenType = ";"
	COLON     TokenType = ":"
	DOT       TokenType = "."
	ARROW     TokenType = "->"

	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"

	// Keywords
	IMPORT   TokenType = "import"
	AS       TokenType = "as"
	STRUCT   TokenType = "struct"
	FUNCTION TokenType = "fn"
	LET      TokenType = "let"
	IF       TokenType = "if"
	ELSE     TokenType = "else"
	FOR      TokenType = "for"
	IN       TokenType = "in"
	WHILE    TokenType = "while"
	RETURN   TokenType = "return"
	BREAK    TokenType = "break"
	CONTINUE TokenType = "continue"
	TRUE     TokenType = "true"
	FALSE    TokenType = "false"
	NIL      TokenType = "nil"
	TRY      TokenType = "try"
	CATCH    TokenType = "catch"
)

// Token is a single lexical unit with its source position. File is empty for
// the file that was run and holds the import path for everything else, so an
// error only has to name a file when it did not come from the one you ran.
type Token struct {
	Type    TokenType
	Literal string
	File    string
	Line    int
	Column  int
}

// Position is a token's place in the source, in the short form errors use:
// "line:col", or "file:line:col" for an imported file.
func (t Token) Position() string {
	if t.File != "" {
		return fmt.Sprintf("%s:%d:%d", t.File, t.Line, t.Column)
	}
	return fmt.Sprintf("%d:%d", t.Line, t.Column)
}

// hasPositionPrefix reports whether msg starts with a "line:col: " or
// "file:line:col: " location prefix.
func hasPositionPrefix(msg string) bool {
	colonSpace := strings.Index(msg, ": ")
	if colonSpace == -1 {
		return false
	}
	prefix := msg[:colonSpace]
	lastColon := strings.LastIndex(prefix, ":")
	if lastColon == -1 {
		return false
	}
	colStr := prefix[lastColon+1:]
	if !isAllDigits(colStr) {
		return false
	}
	rem := prefix[:lastColon]
	secondLastColon := strings.LastIndex(rem, ":")
	lineStr := rem
	if secondLastColon != -1 {
		lineStr = rem[secondLastColon+1:]
	}
	return isAllDigits(lineStr)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

var keywords = map[string]TokenType{
	"import":   IMPORT,
	"as":       AS,
	"struct":   STRUCT,
	"fn":       FUNCTION,
	"let":      LET,
	"if":       IF,
	"else":     ELSE,
	"for":      FOR,
	"in":       IN,
	"while":    WHILE,
	"return":   RETURN,
	"break":    BREAK,
	"continue": CONTINUE,
	"true":     TRUE,
	"false":    FALSE,
	"nil":      NIL,
	"try":      TRY,
	"catch":    CATCH,
}

// LookupIdent turns an identifier into a keyword token when it matches one.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
