package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// eof marks the end of input as a rune that can never appear in source.
const eof = rune(0)

// Lexer turns Jin source text into a stream of tokens.
type Lexer struct {
	input        string
	file         string // the import path this text came from, empty for the file that was run
	position     int    // current position (byte offset of ch)
	readPosition int    // next position to read
	ch           rune   // current rune under examination
	line         int    // 1-based line of the current rune
	lineStart    int    // byte offset where the current line begins
}

// NewLexer primes a lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, line: 1, lineStart: 0}
	l.readChar()
	return l
}

// NewLexerFile primes a lexer for text that came from an imported file. Its
// tokens carry the file's name so errors can point at the right file.
func NewLexerFile(input, file string) *Lexer {
	l := NewLexer(input)
	l.file = file
	return l
}

// NewLexerAt starts lexing as if the input began at the given 1-based line.
// It is used for expressions pulled out of a string, so that errors inside
// {interpolation} point at the real line in the file rather than line 1.
func NewLexerAt(input string, line int, file string) *Lexer {
	l := NewLexerFile(input, file)
	if line > 0 {
		l.line = line
	}
	return l
}

// readChar advances to the next rune, keeping line/lineStart in sync.
func (l *Lexer) readChar() {
	// If we are leaving a newline, the next rune starts a new line.
	if l.ch == '\n' {
		l.line++
		l.lineStart = l.position + 1
	}

	if l.readPosition >= len(l.input) {
		l.ch = eof
		l.position = l.readPosition
		return
	}

	r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
	l.ch = r
	l.position = l.readPosition
	l.readPosition += size
}

// peekChar returns the next rune without consuming it.
func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return eof
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

// column returns the 1-based column of the current rune.
func (l *Lexer) column() int {
	return l.position - l.lineStart + 1
}

// skipWhitespaceAndComments eats spaces, tabs, newlines and # comments.
func (l *Lexer) skipWhitespaceAndComments() {
	for {
		switch {
		case l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n':
			l.readChar()
		case l.ch == '#':
			for l.ch != '\n' && l.ch != eof {
				l.readChar()
			}
		default:
			return
		}
	}
}

// NextToken returns the next token in the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespaceAndComments()

	line := l.line
	col := l.column()

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			return l.twoChar(EQ, "==", line, col)
		}
		return l.simple(ASSIGN, line, col)
	case '!':
		if l.peekChar() == '=' {
			return l.twoChar(NOT_EQ, "!=", line, col)
		}
		return l.simple(BANG, line, col)
	case '<':
		if l.peekChar() == '=' {
			return l.twoChar(LE, "<=", line, col)
		}
		return l.simple(LT, line, col)
	case '>':
		if l.peekChar() == '=' {
			return l.twoChar(GE, ">=", line, col)
		}
		return l.simple(GT, line, col)
	case '&':
		if l.peekChar() == '&' {
			return l.twoChar(AND, "&&", line, col)
		}
		return l.simple(AMP, line, col)
	case '|':
		if l.peekChar() == '|' {
			return l.twoChar(OR, "||", line, col)
		}
		return l.simple(PIPE, line, col)
	case '-':
		if l.peekChar() == '>' {
			return l.twoChar(ARROW, "->", line, col)
		}
		return l.simple(MINUS, line, col)
	case '+':
		return l.simple(PLUS, line, col)
	case '*':
		return l.simple(ASTERISK, line, col)
	case '/':
		return l.simple(SLASH, line, col)
	case '%':
		return l.simple(PERCENT, line, col)
	case ',':
		return l.simple(COMMA, line, col)
	case ';':
		return l.simple(SEMICOLON, line, col)
	case ':':
		return l.simple(COLON, line, col)
	case '.':
		return l.simple(DOT, line, col)
	case '(':
		return l.simple(LPAREN, line, col)
	case ')':
		return l.simple(RPAREN, line, col)
	case '{':
		return l.simple(LBRACE, line, col)
	case '}':
		return l.simple(RBRACE, line, col)
	case '[':
		return l.simple(LBRACKET, line, col)
	case ']':
		return l.simple(RBRACKET, line, col)
	case '"':
		if strings.HasPrefix(l.input[l.position:], `"""`) {
			lit, ok := l.readMultiLineString()
			if !ok {
				return l.tok(ILLEGAL, lit, line, col)
			}
			return l.tok(STRING, lit, line, col)
		}
		lit, ok := l.readString()
		if !ok {
			return l.tok(ILLEGAL, lit, line, col)
		}
		l.readChar() // consume the closing quote
		return l.tok(STRING, lit, line, col)
	case eof:
		return l.tok(EOF, "", line, col)
	}

	if isLetter(l.ch) {
		lit := l.readIdentifier()
		return l.tok(LookupIdent(lit), lit, line, col)
	}

	if isDigit(l.ch) {
		lit, tt := l.readNumber()
		return l.tok(tt, lit, line, col)
	}

	// Anything else is an illegal character.
	ch := l.ch
	l.readChar()
	return l.tok(ILLEGAL, string(ch), line, col)
}

// simple builds a one-rune token and advances past it.
func (l *Lexer) simple(tt TokenType, line, col int) Token {
	lit := string(l.ch)
	l.readChar()
	return l.tok(tt, lit, line, col)
}

// tok builds a token, stamping it with the file it came from.
func (l *Lexer) tok(tt TokenType, lit string, line, col int) Token {
	return Token{Type: tt, Literal: lit, File: l.file, Line: line, Column: col}
}

// twoChar builds a two-rune token (==, !=, <=, >=, &&, ||, ->) and advances
// past both runes. The lexer is positioned on the first rune when called.
func (l *Lexer) twoChar(tt TokenType, lit string, line, col int) Token {
	l.readChar() // move onto the second rune
	l.readChar() // move past the token
	return l.tok(tt, lit, line, col)
}

// readIdentifier reads an identifier or keyword.
func (l *Lexer) readIdentifier() string {
	start := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[start:l.position]
}

// readNumber reads an integer or float literal. Underscores are allowed
// as visual separators, e.g. 1_000_000.
func (l *Lexer) readNumber() (string, TokenType) {
	start := l.position
	tt := INT

	for isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	if l.ch == '.' && isDigit(l.peekChar()) {
		tt = FLOAT
		l.readChar()
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	lit := strings.ReplaceAll(l.input[start:l.position], "_", "")
	return lit, tt
}

// readMultiLineString reads a triple-quoted string """ ... """.
func (l *Lexer) readMultiLineString() (string, bool) {
	// Consume the 3 opening quotes
	l.readChar() // 2nd quote
	l.readChar() // 3rd quote
	l.readChar() // move to first char of content

	var sb strings.Builder
	for {
		if l.ch == eof {
			return sb.String(), false
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '0':
				sb.WriteRune(0)
			case '\\':
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case eof:
				return sb.String(), false
			default:
				sb.WriteRune('\\')
				sb.WriteRune(l.ch)
			}
			l.readChar()
			continue
		}
		if l.ch == '"' && strings.HasPrefix(l.input[l.position:], `"""`) {
			// Found closing """
			l.readChar() // 2nd quote
			l.readChar() // 3rd quote
			l.readChar() // move past 3rd quote
			return sb.String(), true
		}
		sb.WriteRune(l.ch)
		l.readChar()
	}
}

// readString reads a double-quoted string and resolves escape sequences.
// It returns ok=false for an unterminated string.
func (l *Lexer) readString() (string, bool) {
	var sb strings.Builder

	for {
		l.readChar()

		if l.ch == eof || l.ch == '\n' {
			return sb.String(), false
		}

		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '0':
				sb.WriteRune(0)
			case '\\':
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case eof:
				return sb.String(), false
			default:
				sb.WriteRune('\\')
				sb.WriteRune(l.ch)
			}
			continue
		}

		if l.ch == '"' {
			return sb.String(), true
		}

		sb.WriteRune(l.ch)
	}
}

func isLetter(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}
