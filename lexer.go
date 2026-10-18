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
	position     int  // current position (byte offset of ch)
	readPosition int  // next position to read
	ch           rune // current rune under examination
	line         int  // 1-based line of the current rune
	lineStart    int  // byte offset where the current line begins
}

// NewLexer primes a lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, line: 1, lineStart: 0}
	l.readChar()
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
		lit, ok := l.readString()
		if !ok {
			return Token{Type: ILLEGAL, Literal: lit, Line: line, Column: col}
		}
		l.readChar() // consume the closing quote
		return Token{Type: STRING, Literal: lit, Line: line, Column: col}
	case eof:
		return Token{Type: EOF, Literal: "", Line: line, Column: col}
	}

	if isLetter(l.ch) {
		lit := l.readIdentifier()
		return Token{Type: LookupIdent(lit), Literal: lit, Line: line, Column: col}
	}

	if isDigit(l.ch) {
		lit, tt := l.readNumber()
		return Token{Type: tt, Literal: lit, Line: line, Column: col}
	}

	// Anything else is an illegal character.
	ch := l.ch
	l.readChar()
	return Token{Type: ILLEGAL, Literal: string(ch), Line: line, Column: col}
}

// simple builds a one-rune token and advances past it.
func (l *Lexer) simple(tt TokenType, line, col int) Token {
	lit := string(l.ch)
	l.readChar()
	return Token{Type: tt, Literal: lit, Line: line, Column: col}
}

// twoChar builds a two-rune token (==, !=, <=, >=, &&, ||, ->) and advances
// past both runes. The lexer is positioned on the first rune when called.
func (l *Lexer) twoChar(tt TokenType, lit string, line, col int) Token {
	l.readChar() // move onto the second rune
	l.readChar() // move past the token
	return Token{Type: tt, Literal: lit, Line: line, Column: col}
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
