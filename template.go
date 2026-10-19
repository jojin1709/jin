package main

import (
	"fmt"
	"strings"
)

// textSegment is one piece of an interpolated string: either literal text or
// an embedded expression.
type textSegment struct {
	Literal string
	Expr    string
}

// splitInterpolation breaks a string literal into literal and expression
// segments.
//
// Rules:
//   - {expr}  embeds an expression
//   - {{ and }} are literal braces
//   - a lone } is literal
//
// Having this in one place means the type checker and the interpreter always
// agree on where an expression starts and ends.
func splitInterpolation(raw string) ([]textSegment, error) {
	segs := []textSegment{}
	var lit strings.Builder
	runes := []rune(raw)

	flush := func() {
		if lit.Len() > 0 {
			segs = append(segs, textSegment{Literal: lit.String()})
			lit.Reset()
		}
	}

	for idx := 0; idx < len(runes); idx++ {
		ch := runes[idx]

		if ch == '{' {
			if idx+1 < len(runes) && runes[idx+1] == '{' {
				lit.WriteRune('{')
				idx++
				continue
			}

			end := -1
			depth := 0
			inStr := false
			isTriple := false

			for j := idx + 1; j < len(runes); j++ {
				r := runes[j]

				if inStr {
					if r == '\\' {
						j++
						continue
					}
					if isTriple {
						if r == '"' && j+2 < len(runes) && runes[j+1] == '"' && runes[j+2] == '"' {
							inStr = false
							isTriple = false
							j += 2
						}
					} else {
						if r == '"' {
							inStr = false
						}
					}
					continue
				}

				if r == '"' {
					inStr = true
					if j+2 < len(runes) && runes[j+1] == '"' && runes[j+2] == '"' {
						isTriple = true
						j += 2
					} else {
						isTriple = false
					}
					continue
				}

				if r == '{' {
					depth++
					continue
				}

				if r == '}' {
					if depth > 0 {
						depth--
						continue
					}
					end = j
					break
				}
			}
			if end == -1 {
				return nil, fmt.Errorf("missing '}' in string")
			}

			flush()
			segs = append(segs, textSegment{Expr: string(runes[idx+1 : end])})
			idx = end
			continue
		}

		if ch == '}' && idx+1 < len(runes) && runes[idx+1] == '}' {
			lit.WriteRune('}')
			idx++
			continue
		}

		lit.WriteRune(ch)
	}

	flush()
	return segs, nil
}
