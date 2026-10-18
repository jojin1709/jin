package main

import (
	"fmt"
	"io"
	"math"
	"strings"
)

// Interpreter walks a syntax tree and produces a result.
type Interpreter struct {
	out      io.Writer
	builtins map[string]*Builtin
}

// NewInterpreter creates an interpreter that writes program output to out.
func NewInterpreter(out io.Writer) *Interpreter {
	i := &Interpreter{out: out}
	i.builtins = newBuiltins(i)
	return i
}

// Run executes every statement in a program. The last statement's value is
// returned so the REPL can show it.
func (i *Interpreter) Run(program *Program, env *Environment) (Object, error) {
	var result Object = &Nil{}

	for _, stmt := range program.Statements {
		var err error
		result, err = i.evalStatement(stmt, env)
		if err != nil {
			return nil, err
		}
		if isControlFlow(result) {
			return nil, controlFlowError(result)
		}
	}

	return result, nil
}

// ---------------------------------------------------------------- statements

func (i *Interpreter) evalStatement(stmt Statement, env *Environment) (Object, error) {
	switch node := stmt.(type) {

	case *ExpressionStatement:
		return i.evalExpression(node.Expression, env)

	case *LetStatement:
		val, err := i.evalExpression(node.Value, env)
		if err != nil {
			return nil, err
		}
		env.Set(node.Name.Value, val)
		return val, nil

	case *ReturnStatement:
		if node.Value == nil {
			return &ReturnValue{Value: &Nil{}}, nil
		}
		val, err := i.evalExpression(node.Value, env)
		if err != nil {
			return nil, err
		}
		return &ReturnValue{Value: val}, nil

	case *BlockStatement:
		return i.evalBlock(node, env)

	case *FunctionStatement:
		fn := &Function{
			Name:       node.Name.Value,
			Parameters: node.Parameters,
			Body:       node.Body,
			Env:        env,
		}
		env.Set(node.Name.Value, fn)
		return fn, nil

	case *IfStatement:
		return i.evalIf(node, env)

	case *ForStatement:
		return i.evalFor(node, env)

	case *WhileStatement:
		return i.evalWhile(node, env)

	case *BreakStatement:
		return &BreakSignal{}, nil

	case *ContinueStatement:
		return &ContinueSignal{}, nil
	}

	return nil, fmt.Errorf("cannot execute %T", stmt)
}

// evalBlock runs statements in order, stopping early on return/break/continue.
func (i *Interpreter) evalBlock(block *BlockStatement, env *Environment) (Object, error) {
	var result Object = &Nil{}

	for _, stmt := range block.Statements {
		var err error
		result, err = i.evalStatement(stmt, env)
		if err != nil {
			return nil, err
		}
		if isControlFlow(result) {
			return result, nil
		}
	}

	return result, nil
}

func (i *Interpreter) evalIf(node *IfStatement, env *Environment) (Object, error) {
	cond, err := i.evalExpression(node.Condition, env)
	if err != nil {
		return nil, err
	}
	truthy, err := asBoolAt(cond, node.Token)
	if err != nil {
		return nil, err
	}

	if truthy {
		return i.evalBlock(node.Consequence, NewEnclosedEnvironment(env))
	}

	if node.Alternative != nil {
		if block, ok := node.Alternative.(*BlockStatement); ok {
			return i.evalBlock(block, NewEnclosedEnvironment(env))
		}
		return i.evalStatement(node.Alternative, env)
	}

	return &Nil{}, nil
}

func (i *Interpreter) evalFor(node *ForStatement, env *Environment) (Object, error) {
	iterable, err := i.evalExpression(node.Iterable, env)
	if err != nil {
		return nil, err
	}

	items, err := i.iterate(iterable, node.Token)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		loopEnv := NewEnclosedEnvironment(env)
		loopEnv.Set(node.VarName.Value, item)

		result, err := i.evalBlock(node.Body, loopEnv)
		if err != nil {
			return nil, err
		}

		switch result.(type) {
		case *BreakSignal:
			return &Nil{}, nil
		case *ContinueSignal:
			continue
		case *ReturnValue:
			return result, nil
		}
	}

	return &Nil{}, nil
}

func (i *Interpreter) evalWhile(node *WhileStatement, env *Environment) (Object, error) {
	for {
		cond, err := i.evalExpression(node.Condition, env)
		if err != nil {
			return nil, err
		}
		truthy, err := asBoolAt(cond, node.Token)
		if err != nil {
			return nil, err
		}
		if !truthy {
			break
		}

		result, err := i.evalBlock(node.Body, NewEnclosedEnvironment(env))
		if err != nil {
			return nil, err
		}

		switch result.(type) {
		case *BreakSignal:
			return &Nil{}, nil
		case *ContinueSignal:
			continue
		case *ReturnValue:
			return result, nil
		}
	}

	return &Nil{}, nil
}

// iterate turns a value into a list of items a for loop can walk.
func (i *Interpreter) iterate(obj Object, tok Token) ([]Object, error) {
	switch v := obj.(type) {
	case *Array:
		return v.Elements, nil
	case *String:
		items := make([]Object, 0, len(v.Value))
		for _, r := range v.Value {
			items = append(items, &String{Value: string(r)})
		}
		return items, nil
	}
	return nil, fmt.Errorf("%d:%d: cannot loop over %s", tok.Line, tok.Column, obj.Type())
}

// ---------------------------------------------------------------- expressions

func (i *Interpreter) evalExpression(expr Expression, env *Environment) (Object, error) {
	switch node := expr.(type) {

	case *IntegerLiteral:
		return &Integer{Value: node.Value}, nil

	case *FloatLiteral:
		return &Float{Value: node.Value}, nil

	case *BooleanLiteral:
		return &Boolean{Value: node.Value}, nil

	case *NilLiteral:
		return &Nil{}, nil

	case *StringLiteral:
		return i.evalString(node, env)

	case *Identifier:
		return i.evalIdentifier(node, env)

	case *PrefixExpression:
		return i.evalPrefix(node, env)

	case *InfixExpression:
		return i.evalInfix(node, env)

	case *AssignExpression:
		return i.evalAssign(node, env)

	case *CallExpression:
		return i.evalCall(node, env)

	case *IndexExpression:
		return i.evalIndex(node, env)

	case *ArrayLiteral:
		elems := make([]Object, 0, len(node.Elements))
		for _, el := range node.Elements {
			v, err := i.evalExpression(el, env)
			if err != nil {
				return nil, err
			}
			elems = append(elems, v)
		}
		return &Array{Elements: elems}, nil

	case *FunctionLiteral:
		return &Function{Parameters: node.Parameters, Body: node.Body, Env: env}, nil

	case *MemberExpression:
		return nil, fmt.Errorf("%d:%d: property access (.%s) is not supported yet",
			node.Token.Line, node.Token.Column, node.Property.Value)
	}

	return nil, fmt.Errorf("cannot evaluate %T", expr)
}

func (i *Interpreter) evalIdentifier(node *Identifier, env *Environment) (Object, error) {
	if val, ok := env.Get(node.Value); ok {
		return val, nil
	}
	if b, ok := i.builtins[node.Value]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("%d:%d: undefined variable %q", node.Token.Line, node.Token.Column, node.Value)
}

func (i *Interpreter) evalPrefix(node *PrefixExpression, env *Environment) (Object, error) {
	right, err := i.evalExpression(node.Right, env)
	if err != nil {
		return nil, err
	}

	switch node.Operator {
	case "!":
		b, ok := right.(*Boolean)
		if !ok {
			return nil, fmt.Errorf("%d:%d: '!' expects a bool, got %s", node.Token.Line, node.Token.Column, right.Type())
		}
		return &Boolean{Value: !b.Value}, nil
	case "-":
		switch v := right.(type) {
		case *Integer:
			return &Integer{Value: -v.Value}, nil
		case *Float:
			return &Float{Value: -v.Value}, nil
		}
		return nil, fmt.Errorf("%d:%d: '-' expects a number, got %s", node.Token.Line, node.Token.Column, right.Type())
	}

	return nil, fmt.Errorf("%d:%d: unknown operator %s%s", node.Token.Line, node.Token.Column, node.Operator, right.Type())
}

func (i *Interpreter) evalInfix(node *InfixExpression, env *Environment) (Object, error) {
	op := node.Operator

	// && and || short-circuit, so the right side is only evaluated when needed.
	if op == "&&" || op == "||" {
		left, err := i.evalExpression(node.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := asBoolAt(left, node.Token)
		if err != nil {
			return nil, err
		}
		if op == "&&" && !lb {
			return &Boolean{Value: false}, nil
		}
		if op == "||" && lb {
			return &Boolean{Value: true}, nil
		}
		right, err := i.evalExpression(node.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := asBoolAt(right, node.Token)
		if err != nil {
			return nil, err
		}
		return &Boolean{Value: rb}, nil
	}

	left, err := i.evalExpression(node.Left, env)
	if err != nil {
		return nil, err
	}
	right, err := i.evalExpression(node.Right, env)
	if err != nil {
		return nil, err
	}

	switch op {
	case "+":
		return addValues(left, right, node.Token)
	case "-", "*", "/", "%":
		return arithValues(op, left, right, node.Token)
	case "<", ">", "<=", ">=":
		return compareValues(op, left, right, node.Token)
	case "==", "!=":
		equal := objectsEqual(left, right)
		if op == "!=" {
			equal = !equal
		}
		return &Boolean{Value: equal}, nil
	}

	return nil, fmt.Errorf("%d:%d: unknown operator %s %s %s",
		node.Token.Line, node.Token.Column, left.Type(), op, right.Type())
}

func addValues(left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return &Integer{Value: l.Value + r.Value}, nil
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return &Float{Value: l.Value + r.Value}, nil
		}
	case *String:
		if r, ok := right.(*String); ok {
			return &String{Value: l.Value + r.Value}, nil
		}
	case *Array:
		if r, ok := right.(*Array); ok {
			elems := make([]Object, 0, len(l.Elements)+len(r.Elements))
			elems = append(elems, l.Elements...)
			elems = append(elems, r.Elements...)
			return &Array{Elements: elems}, nil
		}
	}

	// Mixed numeric types are rejected on purpose: no silent promotion.
	if isNumber(left) && isNumber(right) && left.Type() != right.Type() {
		return nil, fmt.Errorf("%d:%d: cannot add %s and %s; convert one explicitly with int() or float()",
			tok.Line, tok.Column, left.Type(), right.Type())
	}
	return nil, fmt.Errorf("%d:%d: cannot add %s and %s", tok.Line, tok.Column, left.Type(), right.Type())
}

func arithValues(op string, left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			switch op {
			case "-":
				return &Integer{Value: l.Value - r.Value}, nil
			case "*":
				return &Integer{Value: l.Value * r.Value}, nil
			case "/":
				if r.Value == 0 {
					return nil, fmt.Errorf("%d:%d: division by zero", tok.Line, tok.Column)
				}
				return &Integer{Value: l.Value / r.Value}, nil
			case "%":
				if r.Value == 0 {
					return nil, fmt.Errorf("%d:%d: division by zero", tok.Line, tok.Column)
				}
				return &Integer{Value: l.Value % r.Value}, nil
			}
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			switch op {
			case "-":
				return &Float{Value: l.Value - r.Value}, nil
			case "*":
				return &Float{Value: l.Value * r.Value}, nil
			case "/":
				if r.Value == 0 {
					return nil, fmt.Errorf("%d:%d: division by zero", tok.Line, tok.Column)
				}
				return &Float{Value: l.Value / r.Value}, nil
			case "%":
				if r.Value == 0 {
					return nil, fmt.Errorf("%d:%d: division by zero", tok.Line, tok.Column)
				}
				return &Float{Value: math.Mod(l.Value, r.Value)}, nil
			}
		}
	}

	if isNumber(left) && isNumber(right) && left.Type() != right.Type() {
		return nil, fmt.Errorf("%d:%d: cannot use %s on %s and %s; convert one explicitly with int() or float()",
			tok.Line, tok.Column, op, left.Type(), right.Type())
	}
	return nil, fmt.Errorf("%d:%d: cannot use %s on %s and %s", tok.Line, tok.Column, op, left.Type(), right.Type())
}

func compareValues(op string, left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return &Boolean{Value: compareOrder(op, float64(l.Value), float64(r.Value))}, nil
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return &Boolean{Value: compareOrder(op, l.Value, r.Value)}, nil
		}
	case *String:
		if r, ok := right.(*String); ok {
			return &Boolean{Value: compareStrings(op, l.Value, r.Value)}, nil
		}
	}
	return nil, fmt.Errorf("%d:%d: cannot compare %s and %s", tok.Line, tok.Column, left.Type(), right.Type())
}

// compareOrder applies an ordering operator to two numbers.
func compareOrder(op string, a, b float64) bool {
	switch op {
	case "<":
		return a < b
	case ">":
		return a > b
	case "<=":
		return a <= b
	case ">=":
		return a >= b
	}
	return false
}

func compareStrings(op string, a, b string) bool {
	c := strings.Compare(a, b)
	switch op {
	case "<":
		return c < 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	case ">=":
		return c >= 0
	}
	return false
}

func (i *Interpreter) evalAssign(node *AssignExpression, env *Environment) (Object, error) {
	value, err := i.evalExpression(node.Value, env)
	if err != nil {
		return nil, err
	}

	switch target := node.Target.(type) {
	case *Identifier:
		if !env.Assign(target.Value, value) {
			return nil, fmt.Errorf("%d:%d: undefined variable %q; use 'let' to declare it first",
				node.Token.Line, node.Token.Column, target.Value)
		}
		return value, nil

	case *IndexExpression:
		container, err := i.evalExpression(target.Left, env)
		if err != nil {
			return nil, err
		}
		arr, ok := container.(*Array)
		if !ok {
			return nil, fmt.Errorf("%d:%d: cannot assign into %s", node.Token.Line, node.Token.Column, container.Type())
		}
		idx, err := i.indexValue(target.Index, env)
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx >= int64(len(arr.Elements)) {
			return nil, fmt.Errorf("%d:%d: index %d out of range (length %d)",
				node.Token.Line, node.Token.Column, idx, len(arr.Elements))
		}
		arr.Elements[idx] = value
		return value, nil
	}

	return nil, fmt.Errorf("%d:%d: cannot assign to this expression", node.Token.Line, node.Token.Column)
}

func (i *Interpreter) evalIndex(node *IndexExpression, env *Environment) (Object, error) {
	container, err := i.evalExpression(node.Left, env)
	if err != nil {
		return nil, err
	}
	idx, err := i.indexValue(node.Index, env)
	if err != nil {
		return nil, err
	}

	switch c := container.(type) {
	case *Array:
		if idx < 0 || idx >= int64(len(c.Elements)) {
			return nil, fmt.Errorf("%d:%d: index %d out of range (length %d)",
				node.Token.Line, node.Token.Column, idx, len(c.Elements))
		}
		return c.Elements[idx], nil
	case *String:
		runes := []rune(c.Value)
		if idx < 0 || idx >= int64(len(runes)) {
			return nil, fmt.Errorf("%d:%d: index %d out of range (length %d)",
				node.Token.Line, node.Token.Column, idx, len(runes))
		}
		return &String{Value: string(runes[idx])}, nil
	}

	return nil, fmt.Errorf("%d:%d: cannot index %s", node.Token.Line, node.Token.Column, container.Type())
}

// indexValue evaluates an index expression and requires it to be an int.
func (i *Interpreter) indexValue(expr Expression, env *Environment) (int64, error) {
	obj, err := i.evalExpression(expr, env)
	if err != nil {
		return 0, err
	}
	n, ok := obj.(*Integer)
	if !ok {
		return 0, fmt.Errorf("index must be an int, got %s", obj.Type())
	}
	return n.Value, nil
}

func (i *Interpreter) evalCall(node *CallExpression, env *Environment) (Object, error) {
	callee, err := i.evalExpression(node.Function, env)
	if err != nil {
		return nil, err
	}

	args := make([]Object, 0, len(node.Arguments))
	for _, a := range node.Arguments {
		v, err := i.evalExpression(a, env)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}

	switch fn := callee.(type) {
	case *Function:
		return i.applyFunction(fn, args, node.Token)
	case *Builtin:
		return fn.Fn(args)
	}

	return nil, fmt.Errorf("%d:%d: %s is not callable", node.Token.Line, node.Token.Column, callee.Type())
}

func (i *Interpreter) applyFunction(fn *Function, args []Object, tok Token) (Object, error) {
	if len(args) != len(fn.Parameters) {
		name := fn.Name
		if name == "" {
			name = "fn"
		}
		return nil, fmt.Errorf("%d:%d: %s() expects %d argument(s), got %d",
			tok.Line, tok.Column, name, len(fn.Parameters), len(args))
	}

	callEnv := NewEnclosedEnvironment(fn.Env)
	for idx, p := range fn.Parameters {
		callEnv.Set(p.Name.Value, args[idx])
	}

	result, err := i.evalBlock(fn.Body, callEnv)
	if err != nil {
		return nil, err
	}
	if rv, ok := result.(*ReturnValue); ok {
		return rv.Value, nil
	}
	if isControlFlow(result) {
		return nil, controlFlowError(result)
	}
	return result, nil
}

// evalString produces a string value, resolving {expr} interpolation.
func (i *Interpreter) evalString(node *StringLiteral, env *Environment) (Object, error) {
	raw := node.Value
	if !strings.Contains(raw, "{") {
		return &String{Value: raw}, nil
	}
	out, err := i.interpolate(raw, env, node.Token)
	if err != nil {
		return nil, err
	}
	return &String{Value: out}, nil
}

// interpolate replaces every {expression} with its value. Use {{ and }} for
// literal braces. A '#' before a brace is just a normal character, so
// "friend #{n}" reads naturally.
func (i *Interpreter) interpolate(raw string, env *Environment, tok Token) (string, error) {
	var sb strings.Builder
	runes := []rune(raw)

	for idx := 0; idx < len(runes); idx++ {
		ch := runes[idx]

		if ch == '{' {
			if idx+1 < len(runes) && runes[idx+1] == '{' {
				sb.WriteRune('{')
				idx++
				continue
			}

			end := -1
			for j := idx + 1; j < len(runes); j++ {
				if runes[j] == '}' {
					end = j
					break
				}
			}
			if end == -1 {
				return "", fmt.Errorf("%d:%d: missing '}' in string", tok.Line, tok.Column)
			}

			src := string(runes[idx+1 : end])
			value, err := i.evalInline(src, env, tok)
			if err != nil {
				return "", err
			}
			sb.WriteString(value.Inspect())
			idx = end
			continue
		}

		if ch == '}' && idx+1 < len(runes) && runes[idx+1] == '}' {
			sb.WriteRune('}')
			idx++
			continue
		}

		sb.WriteRune(ch)
	}

	return sb.String(), nil
}

// evalInline parses and evaluates a single expression from source text. This
// is what makes {n + 1} inside a string work.
func (i *Interpreter) evalInline(src string, env *Environment, tok Token) (Object, error) {
	p := NewParser(NewLexer(src))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("%d:%d: bad interpolation {%s}: %s",
			tok.Line, tok.Column, src, strings.Join(p.Errors(), "; "))
	}
	if len(program.Statements) != 1 {
		return nil, fmt.Errorf("%d:%d: interpolation {%s} must be one expression", tok.Line, tok.Column, src)
	}
	stmt, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		return nil, fmt.Errorf("%d:%d: interpolation {%s} must be an expression", tok.Line, tok.Column, src)
	}
	return i.evalExpression(stmt.Expression, env)
}

// ---------------------------------------------------------------- helpers

func isNumber(o Object) bool {
	switch o.(type) {
	case *Integer, *Float:
		return true
	}
	return false
}

func asBoolAt(o Object, tok Token) (bool, error) {
	b, ok := o.(*Boolean)
	if !ok {
		return false, fmt.Errorf("%d:%d: expected a bool, got %s", tok.Line, tok.Column, o.Type())
	}
	return b.Value, nil
}

func objectsEqual(a, b Object) bool {
	switch av := a.(type) {
	case *Integer:
		bv, ok := b.(*Integer)
		return ok && av.Value == bv.Value
	case *Float:
		bv, ok := b.(*Float)
		return ok && av.Value == bv.Value
	case *String:
		bv, ok := b.(*String)
		return ok && av.Value == bv.Value
	case *Boolean:
		bv, ok := b.(*Boolean)
		return ok && av.Value == bv.Value
	case *Nil:
		_, ok := b.(*Nil)
		return ok
	case *Array:
		bv, ok := b.(*Array)
		if !ok || len(av.Elements) != len(bv.Elements) {
			return false
		}
		for idx := range av.Elements {
			if !objectsEqual(av.Elements[idx], bv.Elements[idx]) {
				return false
			}
		}
		return true
	}
	return false
}
