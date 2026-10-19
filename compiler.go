package main

import (
	"fmt"
	"strings"
)

// Position is where a piece of code came from, so run-time errors can point at
// the right line and column even though the tree is gone. File is empty for the
// file that was run — there is only one of those, and the message is shorter
// without it — and holds the file's name for anything imported.
type Position struct {
	Line int
	Col  int
	File string
}

// Bytecode is a compiled program ready for the VM.
type Bytecode struct {
	Instructions Instructions
	Positions    []Position
	Constants    []Object
	NumLocals    int
}

// constantPool is shared by every compiler in one program, so nested functions
// put their constants in the same table as the code that creates them.
type constantPool struct {
	items []Object
}

func (p *constantPool) add(obj Object) int {
	p.items = append(p.items, obj)
	return len(p.items) - 1
}

// loopContext is what break and continue need to know about the loop they sit
// in. Jumps are patched once the loop's layout is known.
type loopContext struct {
	continueJumps []int
	breaks        []int
}

// Compiler turns a syntax tree into bytecode. One compiler handles one
// function; nested functions get their own compiler that shares the constant
// pool and can reach out through enclosing for captured variables.
type Compiler struct {
	pool      *constantPool
	symbols   *SymbolTable
	root      *SymbolTable // the one table every file's globals live in
	enclosing *Compiler

	instructions Instructions
	positions    []Position
	position     Position // where the instruction being emitted came from

	functionName string
	loops        []*loopContext

	// The file being compiled, and the names it can reach other files by. A
	// function inherits both from the file it was written in.
	modulePrefix string
	aliases      map[string]*Module

	// Files already put into this chunk, and the namespaces handed out so far.
	// A REPL reuses one compiler, so an import typed on one line stays usable
	// on the next.
	compiled    map[*Module]bool
	structTypes map[string]*StructType
}

// NewCompiler prepares a compiler that shares a global symbol table. Pass nil
// for globals to start a fresh one.
func NewCompiler(globals *SymbolTable) *Compiler {
	if globals == nil {
		globals = NewSymbolTable(true)
	}
	return &Compiler{
		pool:        &constantPool{},
		symbols:     globals,
		root:        globals,
		aliases:     map[string]*Module{},
		compiled:    map[*Module]bool{},
		structTypes: map[string]*StructType{},
	}
}

// Compile turns a program into bytecode.
func (c *Compiler) Compile(program *Program) (*Bytecode, error) {
	return c.CompileModules(&Module{Program: program})
}

// CompileModules turns a file and everything it imports into one chunk. The
// imports are compiled first, in an order where a file always follows the files
// it imports, so by the time a file's own code runs every name it imported is
// already there.
func (c *Compiler) CompileModules(entry *Module) (*Bytecode, error) {
	// A REPL reuses one compiler, so each chunk starts from a clean slate.
	c.instructions = nil
	c.positions = nil
	c.loops = nil

	// Local slots do not survive between REPL lines, but globals do.
	c.symbols.ResetLocals()

	for _, dep := range entry.Deps {
		if err := c.compileModule(dep); err != nil {
			return nil, err
		}
	}

	c.modulePrefix = entry.Prefix
	for alias, mod := range entry.Aliases() {
		c.aliases[alias] = mod
	}
	c.predefineTopLevel(entry)

	for _, stmt := range entry.Program.Statements {
		if _, ok := stmt.(*ImportStatement); ok {
			continue // an import is a name to reach a file by, not work to do
		}
		if err := c.compileStatement(stmt); err != nil {
			return nil, err
		}
		// The script remembers the last statement's value so the REPL can
		// show it.
		c.emit(OpPopStatement)
	}

	c.emit(OpNil)
	c.emit(OpReturnValue)

	return &Bytecode{
		Instructions: c.instructions,
		Positions:    c.positions,
		Constants:    c.pool.items,
		NumLocals:    c.symbols.NumLocals(),
	}, nil
}

// compileModule puts one imported file's top-level statements into the chunk,
// once, with its own names and its own view of the files it imports.
func (c *Compiler) compileModule(m *Module) error {
	if c.compiled[m] {
		return nil
	}
	c.compiled[m] = true

	savedPrefix, savedAliases := c.modulePrefix, c.aliases
	c.modulePrefix = m.Prefix
	c.aliases = m.Aliases()
	defer func() {
		c.modulePrefix, c.aliases = savedPrefix, savedAliases
	}()

	c.predefineTopLevel(m)

	for _, stmt := range m.Program.Statements {
		if _, ok := stmt.(*ImportStatement); ok {
			continue
		}
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
		c.emit(OpPop)
	}
	return nil
}

// predefineTopLevel declares a file's top-level names before anything in it is
// compiled, so a function can use a global that is defined further down.
func (c *Compiler) predefineTopLevel(m *Module) {
	for _, stmt := range m.Program.Statements {
		switch node := stmt.(type) {
		case *LetStatement:
			c.symbols.Define(c.globalName(node.Name.Value))
		case *FunctionStatement:
			if node.Receiver == nil {
				c.symbols.Define(c.globalName(node.Name.Value))
			}
		case *StructStatement:
			c.symbols.Define(c.globalName(node.Name.Value))
			t := &StructType{
				Name:    node.Name.Value,
				Fields:  make([]string, 0, len(node.Fields)),
				Methods: make(map[string]Object),
			}
			for _, f := range node.Fields {
				t.Fields = append(t.Fields, f.Name.Value)
			}
			c.structTypes[c.globalName(node.Name.Value)] = t
		}
	}
}

// globalName prefixes a top-level name from an imported file, because every
// file's globals live in the one table the VM keeps. The file that was run
// keeps the plain names, so a single-file program is compiled exactly as it
// always was, and names inside a function are never touched.
func (c *Compiler) globalName(name string) string {
	if c.modulePrefix == "" || !c.symbols.isRoot || c.symbols.depth() != 0 {
		return name
	}
	return c.modulePrefix + name
}

// fail reports a compile error, naming the file when the code came from an
// imported one.
func (c *Compiler) fail(tok Token, format string, args ...any) error {
	return fmt.Errorf("%s: %s", tok.Position(), fmt.Sprintf(format, args...))
}

// at says where the instructions about to be emitted came from.
func (c *Compiler) at(tok Token) {
	c.position = Position{Line: tok.Line, Col: tok.Column, File: tok.File}
}

// ---------------------------------------------------------------- emitting

func (c *Compiler) addInstruction(ins []byte) int {
	pos := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	for range ins {
		c.positions = append(c.positions, c.position)
	}
	return pos
}

func (c *Compiler) emit(op Opcode, operands ...int) int {
	return c.addInstruction(Make(op, operands...))
}

// emitTwice adds two raw bytes, used by the closure instruction to list where
// each captured variable comes from.
func (c *Compiler) emitPair(a, b byte) {
	c.addInstruction([]byte{a, b})
}

func (c *Compiler) addConstant(obj Object) int {
	return c.pool.add(obj)
}

func (c *Compiler) changeOperand(pos int, operand int) {
	op := Opcode(c.instructions[pos])
	c.replaceInstruction(pos, Make(op, operand))
}

func (c *Compiler) replaceInstruction(pos int, ins []byte) {
	copy(c.instructions[pos:], ins)
}

// patchJump points a jump at the current end of the code.
func (c *Compiler) patchJump(pos int) {
	c.changeOperand(pos, len(c.instructions))
}

// ---------------------------------------------------------------- lookups

func (c *Compiler) emitGet(sym *Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpGetGlobal, sym.Index)
	case LocalScope:
		c.emit(OpGetLocal, sym.Index)
	case FreeScope:
		c.emit(OpGetFree, sym.Index)
	case BuiltinScope:
		c.emit(OpGetBuiltin, sym.Index)
	}
}

func (c *Compiler) emitSet(sym *Symbol) {
	switch sym.Scope {
	case GlobalScope:
		c.emit(OpSetGlobal, sym.Index)
	case LocalScope:
		c.emit(OpSetLocal, sym.Index)
	case FreeScope:
		c.emit(OpSetFree, sym.Index)
	}
}

// resolve finds a name, creating a captured reference when the variable lives
// in an enclosing function.
func (c *Compiler) resolve(name string) (*Symbol, bool) {
	if sym, ok := c.lookupHere(name); ok {
		return sym, true
	}

	if c.enclosing != nil {
		outer, ok := c.enclosing.resolve(name)
		if !ok {
			return nil, false
		}
		switch outer.Scope {
		case GlobalScope, BuiltinScope:
			return outer, true
		}
		index := c.symbols.DefineFreeNamed(name, freeRef{
			fromLocal: outer.Scope == LocalScope,
			index:     outer.Index,
		})
		return &Symbol{Name: name, Scope: FreeScope, Index: index}, true
	}

	if index, ok := builtinIndex(name); ok {
		return &Symbol{Name: name, Scope: BuiltinScope, Index: index}, true
	}
	return nil, false
}

// lookupHere looks a name up in this function's own table. At the top level of
// an imported file the name carries the file's prefix, so one file never sees
// another file's globals by accident.
func (c *Compiler) lookupHere(name string) (*Symbol, bool) {
	if c.modulePrefix != "" && c.symbols.isRoot && c.symbols.depth() == 0 {
		return c.symbols.Lookup(c.modulePrefix + name)
	}
	return c.symbols.Lookup(name)
}

// namesAVariable reports whether a name is already a parameter, local or
// global here. It is used to decide whether a module alias is really an alias,
// and it walks outwards without capturing anything the way resolve does.
func (c *Compiler) namesAVariable(name string) bool {
	for cur := c; cur != nil; cur = cur.enclosing {
		if sym, ok := cur.lookupHere(name); ok && sym.Scope != BuiltinScope {
			return true
		}
	}
	return false
}

// moduleName finds a name at the top level of another file. Only top-level
// names are reachable, which is what makes them a file's exports.
func (c *Compiler) moduleName(mod *Module, name string) (*Symbol, bool) {
	sym, ok := c.root.Lookup(mod.Prefix + name)
	if !ok || sym.Scope != GlobalScope {
		return nil, false
	}
	return sym, true
}

// ---------------------------------------------------------------- statements

// statementToken finds the token a statement starts at, so every instruction
// it emits can point back to real source.
func statementToken(stmt Statement) Token {
	switch node := stmt.(type) {
	case *LetStatement:
		return node.Token
	case *FunctionStatement:
		return node.Token
	case *StructStatement:
		return node.Token
	case *ReturnStatement:
		return node.Token
	case *BlockStatement:
		return node.Token
	case *ExpressionStatement:
		return node.Token
	case *IfStatement:
		return node.Token
	case *ForStatement:
		return node.Token
	case *WhileStatement:
		return node.Token
	case *BreakStatement:
		return node.Token
	case *ContinueStatement:
		return node.Token
	}
	return Token{}
}

func (c *Compiler) compileStatement(stmt Statement) error {
	if tok := statementToken(stmt); tok.Line > 0 {
		c.at(tok)
	}

	switch node := stmt.(type) {

	case *ImportStatement:
		// The file it names was compiled before this one, and the alias lives
		// in the compiler's alias table, so there is nothing to emit.
		return nil

	case *ExpressionStatement:
		return c.compileExpression(node.Expression)

	case *LetStatement:
		// A function written straight into a let takes the variable's name and
		// is declared first, so it can call itself.
		if lit, ok := node.Value.(*FunctionLiteral); ok {
			sym := c.symbols.Define(c.globalName(node.Name.Value))
			if err := c.compileFunction(lit.Parameters, lit.Body, node.Name.Value); err != nil {
				return err
			}
			c.emit(OpDup)
			c.emitSet(sym)
			return nil
		}

		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		sym := c.symbols.Define(c.globalName(node.Name.Value))
		c.emit(OpDup)
		c.emitSet(sym)
		return nil

	case *FunctionStatement:
		if node.Receiver != nil {
			t := c.structTypes[c.globalName(node.Receiver.Type.Name)]
			if t == nil {
				return c.fail(node.Receiver.Token, "unknown type %q for method %s", node.Receiver.Type.Name, node.Name.Value)
			}
			methodParams := append([]*Parameter{node.Receiver}, node.Parameters...)
			methodFullName := node.Receiver.Type.Name + "." + node.Name.Value
			if err := c.compileFunction(methodParams, node.Body, methodFullName); err != nil {
				return err
			}
			typeConst := c.addConstant(t)
			nameConst := c.addConstant(&String{Value: node.Name.Value})
			c.emit(OpDefineMethod, typeConst, nameConst)
			return nil
		}
		sym := c.symbols.Define(c.globalName(node.Name.Value))
		if err := c.compileFunction(node.Parameters, node.Body, node.Name.Value); err != nil {
			return err
		}
		c.emit(OpDup)
		c.emitSet(sym)
		return nil

	case *StructStatement:
		// A struct is a run-time value: its name holds the type, so the name
		// can be called like anything else to make an instance.
		t := c.structTypes[c.globalName(node.Name.Value)]
		if t == nil {
			t = &StructType{
				Name:    node.Name.Value,
				Fields:  make([]string, 0, len(node.Fields)),
				Methods: make(map[string]Object),
			}
			for _, f := range node.Fields {
				t.Fields = append(t.Fields, f.Name.Value)
			}
			c.structTypes[c.globalName(node.Name.Value)] = t
		}
		c.emit(OpConstant, c.addConstant(t))
		sym := c.symbols.Define(c.globalName(node.Name.Value))
		c.emit(OpDup)
		c.emitSet(sym)
		return nil
	case *ReturnStatement:
		if node.Value == nil {
			c.emit(OpNil)
		} else if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		c.emit(OpReturnValue)
		return nil

	case *BlockStatement:
		c.symbols.EnterBlock()
		err := c.compileBlock(node, true)
		c.symbols.LeaveBlock()
		return err

	case *IfStatement:
		return c.compileIf(node)

	case *ForStatement:
		return c.compileFor(node)

	case *WhileStatement:
		return c.compileWhile(node)

	case *BreakStatement:
		if len(c.loops) == 0 {
			return c.fail(node.Token, "'break' is only allowed inside a loop")
		}
		loop := c.loops[len(c.loops)-1]
		loop.breaks = append(loop.breaks, c.emit(OpJump, 0))
		return nil

	case *ContinueStatement:
		if len(c.loops) == 0 {
			return c.fail(node.Token, "'continue' is only allowed inside a loop")
		}
		loop := c.loops[len(c.loops)-1]
		loop.continueJumps = append(loop.continueJumps, c.emit(OpJump, 0))
		return nil
	}

	return fmt.Errorf("cannot compile %T", stmt)
}

// predefineFunctions declares the function statements in a block before any of
// its statements are compiled. Two functions in the same block can then call
// each other in either order, because each name is already in scope by the time
// the other body is compiled. Only functions get this treatment: a local
// declared with 'let' still has to appear above the line that uses it, which is
// what the type checker enforces too.
func (c *Compiler) predefineFunctions(block *BlockStatement) {
	for _, stmt := range block.Statements {
		if fn, ok := stmt.(*FunctionStatement); ok {
			c.symbols.Define(fn.Name.Value)
		}
	}
}

// compileBlock compiles statements in order. Every statement leaves one value;
// all but the last are discarded, and the last is kept when keepLast is set.
func (c *Compiler) compileBlock(block *BlockStatement, keepLast bool) error {
	if len(block.Statements) == 0 {
		c.emit(OpNil)
		return nil
	}

	c.predefineFunctions(block)

	for i, stmt := range block.Statements {
		if err := c.compileStatement(stmt); err != nil {
			return err
		}
		if i < len(block.Statements)-1 || !keepLast {
			c.emit(OpPop)
		}
	}
	return nil
}

func (c *Compiler) compileIf(node *IfStatement) error {
	c.at(node.Token)
	if err := c.compileExpression(node.Condition); err != nil {
		return err
	}

	jumpNotTruthy := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	c.symbols.EnterBlock()
	if err := c.compileBlock(node.Consequence, true); err != nil {
		return err
	}
	c.symbols.LeaveBlock()

	jumpToEnd := c.emit(OpJump, 0)

	c.patchJump(jumpNotTruthy)
	c.emit(OpPop)

	if node.Alternative != nil {
		if block, ok := node.Alternative.(*BlockStatement); ok {
			c.symbols.EnterBlock()
			err := c.compileBlock(block, true)
			c.symbols.LeaveBlock()
			if err != nil {
				return err
			}
		} else if err := c.compileStatement(node.Alternative); err != nil {
			return err
		}
	} else {
		c.emit(OpNil)
	}

	c.patchJump(jumpToEnd)
	return nil
}

func (c *Compiler) compileWhile(node *WhileStatement) error {
	loop := &loopContext{}
	c.loops = append(c.loops, loop)

	c.at(node.Token)
	// Each pass closes the cells the pass before it left open, so a closure
	// made in one pass keeps that pass's values instead of following the slot
	// into the next one. Slots below the body belong to code outside the loop
	// and stay open.
	bodyStart := c.symbols.numDefs
	start := len(c.instructions)
	c.emit(OpCloseUpvalues, bodyStart)

	if err := c.compileExpression(node.Condition); err != nil {
		return err
	}
	exitJump := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	c.symbols.EnterBlock()
	err := c.compileBlock(node.Body, false)
	c.symbols.LeaveBlock()
	if err != nil {
		return err
	}

	// continue jumps back to the condition.
	for _, pos := range loop.continueJumps {
		c.changeOperand(pos, start)
	}
	c.emit(OpJump, start)

	c.patchJump(exitJump)
	c.emit(OpPop)

	// break lands here, where the loop's value is waiting to be produced.
	breakTarget := len(c.instructions)
	for _, pos := range loop.breaks {
		c.changeOperand(pos, breakTarget)
	}
	c.emit(OpNil)

	c.loops = c.loops[:len(c.loops)-1]
	return nil
}

func (c *Compiler) compileFor(node *ForStatement) error {
	c.at(node.Token)

	if err := c.compileExpression(node.Iterable); err != nil {
		return err
	}
	// Turn whatever was given into something that can be walked by position,
	// so an array, a string and a map all use the same loop.
	c.emit(OpIterate)

	// The sequence and the position are kept in hidden slots.
	iterSym := c.symbols.DefineTemp()
	indexSym := c.symbols.DefineTemp()
	c.emitSet(iterSym)
	c.emit(OpConstant, c.addConstant(&Integer{Value: 0}))
	c.emitSet(indexSym)

	loop := &loopContext{}
	c.loops = append(c.loops, loop)

	// Everything the body binds lives above every slot already in use, so
	// closing from there at the top of each pass hands the body's closures
	// this pass's values and leaves the loop's own slots alone.
	bodyStart := c.symbols.numDefs
	start := len(c.instructions)
	c.emit(OpCloseUpvalues, bodyStart)

	c.emitGet(indexSym)
	c.emitGet(iterSym)
	c.emit(OpLen)
	c.emit(OpLess)
	exitJump := c.emit(OpJumpNotTruthy, 0)
	c.emit(OpPop)

	// Bind the loop variable for this pass. It is a fresh slot in a block
	// scope, so a closure made in the body captures this pass's value.
	c.symbols.EnterBlock()
	loopVar := c.symbols.Define(node.VarName.Value)
	c.emitGet(iterSym)
	c.emitGet(indexSym)
	c.emit(OpIndex)
	c.emitSet(loopVar)

	if err := c.compileBlock(node.Body, false); err != nil {
		return err
	}

	// continue jumps to the step below.
	stepTarget := len(c.instructions)
	for _, pos := range loop.continueJumps {
		c.changeOperand(pos, stepTarget)
	}

	c.emitGet(indexSym)
	c.emit(OpConstant, c.addConstant(&Integer{Value: 1}))
	c.emit(OpAdd)
	c.emitSet(indexSym)
	c.emit(OpJump, start)

	c.patchJump(exitJump)
	c.emit(OpPop)

	breakTarget := len(c.instructions)
	for _, pos := range loop.breaks {
		c.changeOperand(pos, breakTarget)
	}
	c.emit(OpNil)

	c.symbols.LeaveBlock()
	c.loops = c.loops[:len(c.loops)-1]
	return nil
}

// ---------------------------------------------------------------- expressions

func (c *Compiler) compileExpression(expr Expression) error {
	switch node := expr.(type) {

	case *IntegerLiteral:
		c.emit(OpConstant, c.addConstant(&Integer{Value: node.Value}))
		return nil

	case *FloatLiteral:
		c.emit(OpConstant, c.addConstant(&Float{Value: node.Value}))
		return nil

	case *BooleanLiteral:
		if node.Value {
			c.emit(OpTrue)
		} else {
			c.emit(OpFalse)
		}
		return nil

	case *NilLiteral:
		c.emit(OpNil)
		return nil

	case *StringLiteral:
		return c.compileString(node)

	case *Identifier:
		c.at(node.Token)

		// A function referring to itself uses the closure that is running,
		// which is the only way recursion works for a local function.
		if c.functionName != "" && node.Value == c.functionName {
			if sym, ok := c.resolve(node.Value); ok && sym.Scope != GlobalScope && sym.Scope != BuiltinScope {
				c.emit(OpCurrentClosure)
				return nil
			}
		}

		sym, ok := c.resolve(node.Value)
		if !ok {
			if _, isModule := c.aliases[node.Value]; isModule {
				return c.fail(node.Token, "%q is a module; write %s.<name> to use a name from it",
					node.Value, node.Value)
			}
			return c.fail(node.Token, "undefined variable %q", node.Value)
		}
		c.emitGet(sym)
		return nil

	case *PrefixExpression:
		c.at(node.Token)
		if err := c.compileExpression(node.Right); err != nil {
			return err
		}
		c.at(node.Token)
		switch node.Operator {
		case "!":
			c.emit(OpBang)
		case "-":
			c.emit(OpMinus)
		default:
			return c.fail(node.Token, "unknown operator %s", node.Operator)
		}
		return nil

	case *InfixExpression:
		c.at(node.Token)
		return c.compileInfix(node)

	case *AssignExpression:
		c.at(node.Token)
		return c.compileAssign(node)

	case *CallExpression:
		c.at(node.Token)
		if err := c.compileExpression(node.Function); err != nil {
			return err
		}
		for _, arg := range node.Arguments {
			if err := c.compileExpression(arg); err != nil {
				return err
			}
		}
		c.at(node.Token)
		c.emit(OpCall, len(node.Arguments))
		return nil

	case *IndexExpression:
		c.at(node.Token)
		if err := c.compileExpression(node.Left); err != nil {
			return err
		}
		if err := c.compileExpression(node.Index); err != nil {
			return err
		}
		c.at(node.Token)
		c.emit(OpIndex)
		return nil

	case *ArrayLiteral:
		for _, el := range node.Elements {
			if err := c.compileExpression(el); err != nil {
				return err
			}
		}
		c.emit(OpArray, len(node.Elements))
		return nil

	case *MapLiteral:
		// Keys and values are pushed in pairs, so the map can pop them back
		// in the order they were written.
		for _, pair := range node.Pairs {
			if err := c.compileExpression(pair.Key); err != nil {
				return err
			}
			if err := c.compileExpression(pair.Value); err != nil {
				return err
			}
		}
		c.emit(OpBuildMap, len(node.Pairs))
		return nil

	case *FunctionLiteral:
		return c.compileFunction(node.Parameters, node.Body, "")

	case *MemberExpression:
		c.at(node.Token)
		// u.name reaches a top-level name in the file u names. The alias was
		// checked when the file was loaded, so this can only fail when the
		// other file has no such name.
		if id, ok := node.Object.(*Identifier); ok && !c.namesAVariable(id.Value) {
			if mod, isModule := c.aliases[id.Value]; isModule {
				sym, ok := c.moduleName(mod, node.Property.Value)
				if !ok {
					return c.fail(node.Token, "%s has no name %q", id.Value, node.Property.Value)
				}
				c.emitGet(sym)
				return nil
			}
		}
		// Anything else is a field of a value: push the value, then read the
		// field by name. The compiler has no types, so whether the value is
		// really a record, and really has that field, is settled when it runs
		// — which the type checker has usually already done in advance.
		if err := c.compileExpression(node.Object); err != nil {
			return err
		}
		c.at(node.Token)
		c.emit(OpGetField, c.addConstant(&String{Value: node.Property.Value}))
		return nil
	}

	return fmt.Errorf("cannot compile %T", expr)
}

func (c *Compiler) compileInfix(node *InfixExpression) error {
	op := node.Operator

	// && and || only evaluate the right side when they have to.
	if op == "&&" || op == "||" {
		if err := c.compileExpression(node.Left); err != nil {
			return err
		}
		var jump int
		if op == "&&" {
			jump = c.emit(OpJumpNotTruthy, 0)
		} else {
			jump = c.emit(OpJumpIfTruthy, 0)
		}
		c.emit(OpPop)
		if err := c.compileExpression(node.Right); err != nil {
			return err
		}
		c.patchJump(jump)
		return nil
	}

	if err := c.compileExpression(node.Left); err != nil {
		return err
	}
	if err := c.compileExpression(node.Right); err != nil {
		return err
	}
	// Compiling the operands moved the position; point back at the operator.
	c.at(node.Token)

	switch op {
	case "+":
		c.emit(OpAdd)
	case "-":
		c.emit(OpSub)
	case "*":
		c.emit(OpMul)
	case "/":
		c.emit(OpDiv)
	case "%":
		c.emit(OpMod)
	case "==":
		c.emit(OpEqual)
	case "!=":
		c.emit(OpNotEqual)
	case ">":
		c.emit(OpGreater)
	case ">=":
		c.emit(OpGreaterEqual)
	case "<":
		c.emit(OpLess)
	case "<=":
		c.emit(OpLessEqual)
	default:
		return c.fail(node.Token, "unknown operator %s", op)
	}
	return nil
}

func (c *Compiler) compileAssign(node *AssignExpression) error {
	switch target := node.Target.(type) {

	case *Identifier:
		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		sym, ok := c.resolve(target.Value)
		if !ok {
			return c.fail(node.Token, "undefined variable %q; use 'let' to declare it first", target.Value)
		}
		if sym.Scope == BuiltinScope {
			return c.fail(node.Token, "cannot assign to built-in %q", target.Value)
		}
		c.emit(OpDup)
		c.emitSet(sym)
		return nil

	case *IndexExpression:
		if err := c.compileExpression(target.Left); err != nil {
			return err
		}
		if err := c.compileExpression(target.Index); err != nil {
			return err
		}
		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		c.at(target.Token)
		c.emit(OpSetIndex)
		return nil

	case *MemberExpression:
		if id, ok := target.Object.(*Identifier); ok && !c.namesAVariable(id.Value) {
			if _, isModule := c.aliases[id.Value]; isModule {
				return c.fail(target.Token, "cannot assign to %s.%s; another file's names are read-only",
					id.Value, target.Property.Value)
			}
		}
		// p.x = value: push the record, then the value, then write the field.
		// The value is pushed second so it ends up on top, where the write
		// expects it, and the write leaves the value behind as its result.
		if err := c.compileExpression(target.Object); err != nil {
			return err
		}
		if err := c.compileExpression(node.Value); err != nil {
			return err
		}
		c.at(target.Token)
		c.emit(OpSetField, c.addConstant(&String{Value: target.Property.Value}))
		return nil
	}

	return c.fail(node.Token, "cannot assign to this expression")
}

// compileString turns interpolation into bytecode: the text pieces become
// constants and the {expressions} are compiled like any other expression.
func (c *Compiler) compileString(node *StringLiteral) error {
	c.at(node.Token)

	if !strings.Contains(node.Value, "{") {
		c.emit(OpConstant, c.addConstant(&String{Value: node.Value}))
		return nil
	}

	segs, err := splitInterpolation(node.Value)
	if err != nil {
		return c.fail(node.Token, "%s", err)
	}

	count := 0
	for _, seg := range segs {
		if seg.Expr == "" {
			if seg.Literal == "" {
				continue
			}
			c.emit(OpConstant, c.addConstant(&String{Value: seg.Literal}))
			count++
			continue
		}

		p := NewParser(NewLexerAt(seg.Expr, node.Token.Line, node.Token.File))
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return c.fail(node.Token, "bad interpolation {%s}: %s", seg.Expr, strings.Join(p.Errors(), "; "))
		}
		if len(program.Statements) != 1 {
			return c.fail(node.Token, "interpolation {%s} must be one expression", seg.Expr)
		}
		stmt, ok := program.Statements[0].(*ExpressionStatement)
		if !ok {
			return c.fail(node.Token, "interpolation {%s} must be an expression", seg.Expr)
		}
		if err := c.compileExpression(stmt.Expression); err != nil {
			return err
		}
		c.emit(OpInspect)
		count++
	}

	if count == 0 {
		c.emit(OpConstant, c.addConstant(&String{Value: ""}))
		return nil
	}
	c.emit(OpBuildString, count)
	return nil
}

// compileFunction compiles a function body into its own instruction stream and
// emits the code that builds the closure.
func (c *Compiler) compileFunction(params []*Parameter, body *BlockStatement, name string) error {
	child := &Compiler{
		pool:         c.pool,
		symbols:      NewSymbolTable(false),
		root:         c.root,
		enclosing:    c,
		functionName: name,
		modulePrefix: c.modulePrefix,
		aliases:      c.aliases,
		structTypes:  c.structTypes,
	}
	child.position = Position{Line: body.Token.Line, Col: body.Token.Column, File: body.Token.File}

	for _, p := range params {
		child.symbols.Define(p.Name.Value)
	}

	if err := child.compileBlock(body, true); err != nil {
		return err
	}
	child.emit(OpReturnValue)

	freeRefs := child.symbols.FreeRefs()
	fn := &CompiledFunction{
		Name:         name,
		Instructions: child.instructions,
		Positions:    child.positions,
		NumLocals:    child.symbols.NumLocals(),
		NumParams:    len(params),
	}

	c.emit(OpClosure, c.addConstant(fn), len(freeRefs))
	for _, ref := range freeRefs {
		kind := byte(0) // captured from the enclosing frame's locals
		if !ref.fromLocal {
			kind = 1 // captured from the enclosing closure's own captures
		}
		c.emitPair(kind, byte(ref.index))
	}
	return nil
}

// builtinIndexMap maps a built-in's name to the slot OpGetBuiltin uses.
var builtinIndexMap = func() map[string]int {
	m := make(map[string]int, len(builtinNames))
	for i, name := range builtinNames {
		m[name] = i
	}
	return m
}()

func builtinIndex(name string) (int, bool) {
	i, ok := builtinIndexMap[name]
	return i, ok
}
