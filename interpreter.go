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

	// The top-level scope of each imported file, and which files have already
	// run. A REPL keeps one interpreter, so a file imported on one line stays
	// usable on the next without running it again.
	moduleEnvs map[*Module]*Environment
	ran        map[*Module]bool
}

// NewInterpreter creates an interpreter that writes program output to out.
func NewInterpreter(out io.Writer) *Interpreter {
	return NewInterpreterWithArgs(out, nil)
}

// NewInterpreterWithArgs creates an interpreter that also knows the arguments
// the program was run with, which is what args() reports.
func NewInterpreterWithArgs(out io.Writer, programArgs []string) *Interpreter {
	i := &Interpreter{
		out:        out,
		moduleEnvs: map[*Module]*Environment{},
		ran:        map[*Module]bool{},
	}
	i.builtins = newBuiltins(out, programArgs)
	return i
}

// Run executes every statement in a program. The last statement's value is
// returned so the REPL can show it.
func (i *Interpreter) Run(program *Program, env *Environment) (Object, error) {
	return i.RunModules(&Module{Program: program}, env)
}

// RunModules runs a file and everything it imports. Every imported file runs
// once, before the file that imported it, matching the bytecode back end.
func (i *Interpreter) RunModules(entry *Module, env *Environment) (Object, error) {
	for _, dep := range entry.Deps {
		if err := i.runModule(dep); err != nil {
			return nil, err
		}
	}

	for alias, mod := range entry.Aliases() {
		env.modules[alias] = i.moduleEnvs[mod]
	}

	return i.runProgram(entry.Program, env)
}

// runModule evaluates one imported file's top-level statements in a scope of
// its own, once, and keeps that scope so other files can reach its names.
func (i *Interpreter) runModule(m *Module) error {
	if i.ran[m] {
		return nil
	}
	i.ran[m] = true

	modEnv := NewEnvironment()
	for alias, dep := range m.Aliases() {
		modEnv.modules[alias] = i.moduleEnvs[dep]
	}
	i.moduleEnvs[m] = modEnv

	_, err := i.runProgram(m.Program, modEnv)
	return err
}

// runProgram executes every statement of one file, stopping if control flow
// escapes to the top level.
func (i *Interpreter) runProgram(program *Program, env *Environment) (Object, error) {
	// Predeclare struct types so functions and methods can reference them
	// regardless of statement order.
	for _, stmt := range program.Statements {
		if node, ok := stmt.(*StructStatement); ok {
			t := &StructType{
				Name:    node.Name.Value,
				Fields:  make([]string, 0, len(node.Fields)),
				Methods: make(map[string]Object),
			}
			for _, f := range node.Fields {
				t.Fields = append(t.Fields, f.Name.Value)
			}
			env.Set(node.Name.Value, t)
		}
	}

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

	case *ImportStatement:
		// The file it names has already run; the alias was installed before
		// this file's statements started.
		return &Nil{}, nil

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
		if node.Receiver != nil {
			typeVal, ok := env.Get(node.Receiver.Type.Name)
			if !ok {
				return nil, at(node.Receiver.Token, "unknown type %q for method %s", node.Receiver.Type.Name, node.Name.Value)
			}
			st, ok := typeVal.(*StructType)
			if !ok {
				return nil, at(node.Receiver.Token, "%s is not a record type", node.Receiver.Type.Name)
			}
			methodParams := append([]*Parameter{node.Receiver}, node.Parameters...)
			fn := &Function{
				Name:       node.Receiver.Type.Name + "." + node.Name.Value,
				Parameters: methodParams,
				Body:       node.Body,
				Env:        env,
			}
			if st.Methods == nil {
				st.Methods = make(map[string]Object)
			}
			st.Methods[node.Name.Value] = fn
			return fn, nil
		}
		fn := &Function{
			Name:       node.Name.Value,
			Parameters: node.Parameters,
			Body:       node.Body,
			Env:        env,
		}
		env.Set(node.Name.Value, fn)
		return fn, nil

	case *StructStatement:
		// A record type becomes a value the name holds, so the name can be
		// called to make an instance, and can be reached through an import.
		var t *StructType
		if existing, ok := env.Get(node.Name.Value); ok {
			if st, ok := existing.(*StructType); ok {
				t = st
			}
		}
		if t == nil {
			t = &StructType{
				Name:    node.Name.Value,
				Fields:  make([]string, 0, len(node.Fields)),
				Methods: make(map[string]Object),
			}
			for _, f := range node.Fields {
				t.Fields = append(t.Fields, f.Name.Value)
			}
			env.Set(node.Name.Value, t)
		}
		return t, nil

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
	seq, err := iterateValues(obj)
	if err != nil {
		return nil, at(tok, "%s", err)
	}
	// iterateValues hands back an array for every collection, and for an array
	// it hands back the array itself, so a loop sees the live elements.
	return seq.(*Array).Elements, nil
}

// ---------------------------------------------------------------- expressions

func (i *Interpreter) evalExpression(expr Expression, env *Environment) (Object, error) {
	switch node := expr.(type) {

	case *IntegerLiteral:
		return newInteger(node.Value), nil

	case *FloatLiteral:
		return newFloat(node.Value), nil

	case *BooleanLiteral:
		return newBoolean(node.Value), nil

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

	case *MapLiteral:
		m := &Map{}
		for _, pair := range node.Pairs {
			key, err := i.evalExpression(pair.Key, env)
			if err != nil {
				return nil, err
			}
			value, err := i.evalExpression(pair.Value, env)
			if err != nil {
				return nil, err
			}
			if err := m.Set(key, value); err != nil {
				return nil, at(node.Token, "%s", err)
			}
		}
		return m, nil

	case *FunctionLiteral:
		return &Function{Parameters: node.Parameters, Body: node.Body, Env: env}, nil

	case *MemberExpression:
		return i.evalMember(node, env)
	}

	return nil, fmt.Errorf("cannot evaluate %T", expr)
}

// evalMember reads a name from an imported file (u.name), or a field of a
// record (p.x). Only a file's top-level names are reachable, and only through
// the alias the import gave it.
func (i *Interpreter) evalMember(node *MemberExpression, env *Environment) (Object, error) {
	if id, ok := node.Object.(*Identifier); ok {
		if _, shadowed := env.Get(id.Value); !shadowed {
			if modEnv, isModule := env.modules[id.Value]; isModule {
				val, ok := modEnv.Get(node.Property.Value)
				if !ok {
					return nil, at(node.Token, "%s has no name %q", id.Value, node.Property.Value)
				}
				return val, nil
			}
		}
	}

	obj, err := i.evalExpression(node.Object, env)
	if err != nil {
		return nil, err
	}
	value, err := getField(obj, node.Property.Value)
	if err != nil {
		return nil, at(node.Token, "%s", err)
	}
	return value, nil
}

func (i *Interpreter) evalIdentifier(node *Identifier, env *Environment) (Object, error) {
	if val, ok := env.Get(node.Value); ok {
		return val, nil
	}
	if b, ok := i.builtins[node.Value]; ok {
		return b, nil
	}
	if _, isModule := env.modules[node.Value]; isModule {
		return nil, at(node.Token, "%q is a module; write %s.<name> to use a name from it",
			node.Value, node.Value)
	}
	return nil, at(node.Token, "undefined variable %q", node.Value)
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
			return nil, at(node.Token, "'!' expects a bool, got %s", right.Type())
		}
		return newBoolean(!b.Value), nil
	case "-":
		switch v := right.(type) {
		case *Integer:
			return newInteger(-v.Value), nil
		case *Float:
			return newFloat(-v.Value), nil
		}
		return nil, at(node.Token, "'-' expects a number, got %s", right.Type())
	}

	return nil, at(node.Token, "unknown operator %s%s", node.Operator, right.Type())
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
			return newBoolean(false), nil
		}
		if op == "||" && lb {
			return newBoolean(true), nil
		}
		right, err := i.evalExpression(node.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := asBoolAt(right, node.Token)
		if err != nil {
			return nil, err
		}
		return newBoolean(rb), nil
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
		return newBoolean(equal), nil
	}

	return nil, at(node.Token, "unknown operator %s %s %s", left.Type(), op, right.Type())
}

func addValues(left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return newInteger(l.Value + r.Value), nil
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return newFloat(l.Value + r.Value), nil
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
		return nil, at(tok, "cannot add %s and %s; convert one explicitly with int() or float()", left.Type(), right.Type())
	}
	return nil, at(tok, "cannot add %s and %s", left.Type(), right.Type())
}

func arithValues(op string, left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			switch op {
			case "-":
				return newInteger(l.Value - r.Value), nil
			case "*":
				return newInteger(l.Value * r.Value), nil
			case "/":
				if r.Value == 0 {
					return nil, at(tok, "division by zero")
				}
				return newInteger(l.Value / r.Value), nil
			case "%":
				if r.Value == 0 {
					return nil, at(tok, "division by zero")
				}
				return newInteger(l.Value % r.Value), nil
			}
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			switch op {
			case "-":
				return newFloat(l.Value - r.Value), nil
			case "*":
				return newFloat(l.Value * r.Value), nil
			case "/":
				if r.Value == 0 {
					return nil, at(tok, "division by zero")
				}
				return newFloat(l.Value / r.Value), nil
			case "%":
				if r.Value == 0 {
					return nil, at(tok, "division by zero")
				}
				return newFloat(math.Mod(l.Value, r.Value)), nil
			}
		}
	}

	if isNumber(left) && isNumber(right) && left.Type() != right.Type() {
		return nil, at(tok, "cannot use %s on %s and %s; convert one explicitly with int() or float()", op, left.Type(), right.Type())
	}
	return nil, at(tok, "cannot use %s on %s and %s", op, left.Type(), right.Type())
}

func compareValues(op string, left, right Object, tok Token) (Object, error) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return newBoolean(compareOrder(op, float64(l.Value), float64(r.Value))), nil
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return newBoolean(compareOrder(op, l.Value, r.Value)), nil
		}
	case *String:
		if r, ok := right.(*String); ok {
			return newBoolean(compareStrings(op, l.Value, r.Value)), nil
		}
	}
	return nil, at(tok, "cannot compare %s and %s", left.Type(), right.Type())
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
			return nil, at(node.Token, "undefined variable %q; use 'let' to declare it first", target.Value)
		}
		return value, nil

	case *IndexExpression:
		container, err := i.evalExpression(target.Left, env)
		if err != nil {
			return nil, err
		}
		index, err := i.evalExpression(target.Index, env)
		if err != nil {
			return nil, err
		}
		if err := setInto(container, index, value); err != nil {
			return nil, at(target.Token, "%s", err)
		}
		return value, nil

	case *MemberExpression:
		if id, ok := target.Object.(*Identifier); ok {
			if _, shadowed := env.Get(id.Value); !shadowed {
				if _, isModule := env.modules[id.Value]; isModule {
					return nil, at(target.Token, "cannot assign to %s.%s; another file's names are read-only",
						id.Value, target.Property.Value)
				}
			}
		}
		obj, err := i.evalExpression(target.Object, env)
		if err != nil {
			return nil, err
		}
		if err := setField(obj, target.Property.Value, value); err != nil {
			return nil, at(target.Token, "%s", err)
		}
		return value, nil
	}

	return nil, at(node.Token, "cannot assign to this expression")
}

func (i *Interpreter) evalIndex(node *IndexExpression, env *Environment) (Object, error) {
	container, err := i.evalExpression(node.Left, env)
	if err != nil {
		return nil, err
	}
	index, err := i.evalExpression(node.Index, env)
	if err != nil {
		return nil, err
	}

	value, err := indexInto(container, index)
	if err != nil {
		return nil, at(node.Token, "%s", err)
	}
	return value, nil
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
		result, err := fn.Fn(args)
		if err != nil {
			return nil, at(node.Token, "%s", err)
		}
		return result, nil
	case *StructType:
		// A record type is called to make an instance, the way a class is.
		value, err := newStruct(fn, args)
		if err != nil {
			return nil, at(node.Token, "%s", err)
		}
		return value, nil
	case *BoundMethod:
		callArgs := append([]Object{fn.Receiver}, args...)
		switch m := fn.Method.(type) {
		case *Function:
			return i.applyFunction(m, callArgs, node.Token)
		case *Builtin:
			res, err := m.Fn(callArgs)
			if err != nil {
				return nil, at(node.Token, "%s", err)
			}
			return res, nil
		}
		return nil, at(node.Token, "%s is not callable", fn.Method.Type())
	}

	return nil, at(node.Token, "%s is not callable", callee.Type())
}

func (i *Interpreter) applyFunction(fn *Function, args []Object, tok Token) (Object, error) {
	if len(args) != len(fn.Parameters) {
		name := fn.Name
		if name == "" {
			name = "fn"
		}
		return nil, at(tok, "%s() expects %d argument(s), got %d", name, len(fn.Parameters), len(args))
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
//
// Splitting the string is shared with the type checker (see splitInterpolation)
// so both always find the same embedded expressions.
func (i *Interpreter) interpolate(raw string, env *Environment, tok Token) (string, error) {
	segs, err := splitInterpolation(raw)
	if err != nil {
		return "", at(tok, "%s", err)
	}

	var sb strings.Builder
	for _, seg := range segs {
		if seg.Expr == "" {
			sb.WriteString(seg.Literal)
			continue
		}
		value, err := i.evalInline(seg.Expr, env, tok)
		if err != nil {
			return "", err
		}
		sb.WriteString(value.Inspect())
	}

	return sb.String(), nil
}

// evalInline parses and evaluates a single expression from source text. This
// is what makes {n + 1} inside a string work.
func (i *Interpreter) evalInline(src string, env *Environment, tok Token) (Object, error) {
	p := NewParser(NewLexerAt(src, tok.Line, tok.File))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, at(tok, "bad interpolation {%s}: %s", src, strings.Join(p.Errors(), "; "))
	}
	if len(program.Statements) != 1 {
		return nil, at(tok, "interpolation {%s} must be one expression", src)
	}
	stmt, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		return nil, at(tok, "interpolation {%s} must be an expression", src)
	}
	return i.evalExpression(stmt.Expression, env)
}

// ---------------------------------------------------------------- helpers

// at builds a runtime error that points at a piece of source. An imported
// file's name is included; the file that was run is left out, because there is
// only one of it and the message stays shorter without it.
func at(tok Token, format string, args ...any) error {
	return fmt.Errorf("%s: %s", tok.Position(), fmt.Sprintf(format, args...))
}

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
		return false, at(tok, "expected a bool, got %s", o.Type())
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
	case *Struct:
		// Two records are equal when they are the same kind of record and
		// every field holds an equal value, the same way two arrays are.
		bv, ok := b.(*Struct)
		if !ok || av.Def.Name != bv.Def.Name || len(av.Fields) != len(bv.Fields) {
			return false
		}
		for idx := range av.Fields {
			if !objectsEqual(av.Fields[idx], bv.Fields[idx]) {
				return false
			}
		}
		return true
	case *BoundMethod:
		bv, ok := b.(*BoundMethod)
		if !ok {
			return false
		}
		return objectsEqual(av.Receiver, bv.Receiver) && av.Method == bv.Method
	}
	return false
}
