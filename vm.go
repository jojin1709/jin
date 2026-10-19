package main

import (
	"fmt"
	"io"
	"math"
	"unicode/utf8"
)

const (
	// stackSize is how deep the value stack starts out; it grows if a program
	// needs more.
	stackSize = 2048
	// maxFrames is how many calls may be nested at once.
	maxFrames = 1024
)

// VM runs bytecode. It keeps one value stack, a stack of call frames, and the
// globals, which survive between REPL lines.
type VM struct {
	constants []Object
	stack     []Object
	sp        int
	frames    []*Frame
	frameTop  int // number of frames in use

	globals    []Object
	builtins   []*Builtin
	lastPopped Object
	out        io.Writer

	// openCells holds one cell per local slot that some closure has captured
	// and whose frame is still running, in increasing slot order. Every closure
	// that captures the same slot shares the cell, which is what makes sibling
	// closures see each other's writes.
	openCells []*cell
}

// NewVM prepares a virtual machine that writes program output to out.
func NewVM(out io.Writer) *VM {
	return NewVMWithArgs(out, nil)
}

// NewVMWithArgs prepares a virtual machine that also knows the arguments the
// program was run with, which is what args() reports.
func NewVMWithArgs(out io.Writer, programArgs []string) *VM {
	vm := &VM{
		stack:   make([]Object, stackSize),
		frames:  make([]*Frame, maxFrames),
		out:     out,
		globals: []Object{},
	}
	// Frames are reused across calls, so a call allocates nothing.
	for i := range vm.frames {
		vm.frames[i] = &Frame{}
	}
	vm.builtins = orderedBuiltins(out, programArgs)
	return vm
}

// orderedBuiltins returns the built-ins in the order OpGetBuiltin expects.
func orderedBuiltins(out io.Writer, programArgs []string) []*Builtin {
	byName := newBuiltins(out, programArgs)
	list := make([]*Builtin, len(builtinNames))
	for i, name := range builtinNames {
		list[i] = byName[name]
	}
	return list
}

// Globals exposes the global slots, so a caller can carry them between runs.
func (vm *VM) Globals() []Object { return vm.globals }

// SetGlobals restores global slots saved from an earlier run.
func (vm *VM) SetGlobals(globals []Object) { vm.globals = globals }

// Run executes one compiled chunk and returns its value, which is whatever the
// last top-level statement produced.
func (vm *VM) Run(bc *Bytecode) (result Object, err error) {
	vm.constants = bc.Constants
	vm.frameTop = 0
	vm.sp = 0
	vm.lastPopped = nilObject

	// Every return closes the cells its frame opened, so the list should be
	// empty already; resetting makes that a guarantee rather than a hope.
	vm.openCells = vm.openCells[:0]

	// A run that stops early never unwinds its frames, so any cell still open
	// is closed here. The slots it points at are untouched by the failure:
	// locals sit below the operand stack, which is the only part that moves.
	// Without this, a closure that had reached a global would be left pointing
	// at a stack slot the next run reuses.
	defer func() {
		if err != nil {
			vm.closeCells(0)
		}
	}()

	script := &Closure{Fn: &CompiledFunction{
		Name:         "<script>",
		Instructions: bc.Instructions,
		Positions:    bc.Positions,
		NumLocals:    bc.NumLocals,
	}}

	vm.pushFrame(script, 0)
	for i := 0; i < bc.NumLocals; i++ {
		vm.push(nilObject)
	}

	// The running frame and its instructions are kept in locals: reading them
	// back out of vm.frames on every instruction is pure overhead. They are
	// refreshed whenever a call or a return changes the current frame.
	frame := vm.frames[0]
	ins := frame.cl.Fn.Instructions

	for vm.frameTop > 0 {
		if frame.ip >= len(ins) {
			return nil, fmt.Errorf("ran past the end of a function")
		}

		op := Opcode(ins[frame.ip])
		frame.ip++

		switch op {

		case OpConstant:
			index := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			vm.push(vm.constants[index])

		case OpTrue:
			vm.push(newBoolean(true))

		case OpFalse:
			vm.push(newBoolean(false))

		case OpNil:
			vm.push(nilObject)

		case OpPop:
			vm.pop()

		case OpPopStatement:
			vm.lastPopped = vm.pop()

		case OpDup:
			vm.push(vm.peek())

		case OpAdd:
			right := vm.pop()
			left := vm.pop()
			if result, ok := fastAdd(left, right); ok {
				vm.push(result)
				break
			}
			// The types did not work out; redo it to get the exact message.
			_, err := addValues(left, right, vm.tokenHere())
			return nil, err

		case OpSub, OpMul, OpDiv, OpMod:
			right := vm.pop()
			left := vm.pop()
			if result, ok := fastArith(op, left, right); ok {
				vm.push(result)
				break
			}
			_, err := arithValues(numericOpName(op), left, right, vm.tokenHere())
			return nil, err

		case OpEqual:
			right := vm.pop()
			left := vm.pop()
			vm.push(newBoolean(objectsEqual(left, right)))

		case OpNotEqual:
			right := vm.pop()
			left := vm.pop()
			vm.push(newBoolean(!objectsEqual(left, right)))

		case OpGreater, OpGreaterEqual, OpLess, OpLessEqual:
			right := vm.pop()
			left := vm.pop()
			if result, ok := fastCompare(op, left, right); ok {
				vm.push(result)
				break
			}
			_, err := compareValues(orderOpName(op), left, right, vm.tokenHere())
			return nil, err

		case OpMinus:
			operand := vm.pop()
			switch v := operand.(type) {
			case *Integer:
				vm.push(newInteger(-v.Value))
			case *Float:
				vm.push(newFloat(-v.Value))
			default:
				return nil, vm.errorf("'-' expects a number, got %s", operand.Type())
			}

		case OpBang:
			operand := vm.pop()
			b, ok := operand.(*Boolean)
			if !ok {
				return nil, vm.errorf("'!' expects a bool, got %s", operand.Type())
			}
			vm.push(newBoolean(!b.Value))

		case OpInspect:
			vm.push(&String{Value: vm.pop().Inspect()})

		case OpLen:
			operand := vm.pop()
			n, err := lengthOf(operand)
			if err != nil {
				return nil, vm.errorf("%s", err)
			}
			vm.push(newInteger(n))

		case OpBuildString:
			count := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			parts := make([]string, count)
			for i := count - 1; i >= 0; i-- {
				s, ok := vm.pop().(*String)
				if !ok {
					return nil, vm.errorf("cannot build a string here")
				}
				parts[i] = s.Value
			}
			vm.push(&String{Value: joinStrings(parts)})

		case OpJump:
			frame.ip = int(ReadUint16(ins[frame.ip:]))

		case OpJumpNotTruthy:
			target := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			cond := vm.peek()
			if !isTruthy(cond) {
				frame.ip = target
			}

		case OpJumpIfTruthy:
			target := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			cond := vm.peek()
			if isTruthy(cond) {
				frame.ip = target
			}

		case OpGetGlobal:
			index := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			vm.push(vm.getGlobal(index))

		case OpSetGlobal:
			index := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			vm.setGlobal(index, vm.pop())

		case OpGetLocal:
			index := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			vm.push(vm.stack[frame.basePointer+index])

		case OpSetLocal:
			index := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			vm.stack[frame.basePointer+index] = vm.pop()

		case OpGetFree:
			index := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			vm.push(frame.cl.Free[index].get())

		case OpSetFree:
			index := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			frame.cl.Free[index].set(vm.pop())

		case OpGetBuiltin:
			index := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			vm.push(vm.builtins[index])

		case OpCurrentClosure:
			vm.push(frame.cl)

		case OpClosure:
			constIndex := int(ReadUint16(ins[frame.ip:]))
			// The count is one byte, then a (kind, index) pair per variable.
			count := int(ins[frame.ip+2])
			frame.ip += 3

			fn, ok := vm.constants[constIndex].(*CompiledFunction)
			if !ok {
				return nil, vm.errorf("closure constant is not a function")
			}

			free := make([]*cell, count)
			for i := 0; i < count; i++ {
				kind := ins[frame.ip]
				index := int(ins[frame.ip+1])
				frame.ip += 2
				if kind == 0 {
					// A local of the frame that is running now.
					free[i] = vm.capture(frame.basePointer + index)
				} else {
					// A capture the enclosing closure already holds. Share
					// the cell itself, so a write from either level is seen
					// by both.
					free[i] = frame.cl.Free[index]
				}
			}
			vm.push(&Closure{Fn: fn, Free: free})

		case OpArray:
			count := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			elements := make([]Object, count)
			for i := count - 1; i >= 0; i-- {
				elements[i] = vm.pop()
			}
			vm.push(&Array{Elements: elements})

		case OpBuildMap:
			count := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			// Pairs went on in written order, so read them back the same way.
			pairs := make([]MapEntry, count)
			for i := count - 1; i >= 0; i-- {
				value := vm.pop()
				pairs[i] = MapEntry{Key: vm.pop(), Value: value}
			}
			m := &Map{}
			for _, pair := range pairs {
				if err := m.Set(pair.Key, pair.Value); err != nil {
					return nil, vm.errorf("%s", err)
				}
			}
			vm.push(m)

		case OpIterate:
			seq, err := iterateValues(vm.pop())
			if err != nil {
				return nil, vm.errorf("%s", err)
			}
			vm.push(seq)

		case OpIndex:
			index := vm.pop()
			container := vm.pop()
			value, err := indexInto(container, index)
			if err != nil {
				return nil, vm.errorf("%s", err)
			}
			vm.push(value)

		case OpSetIndex:
			value := vm.pop()
			index := vm.pop()
			container := vm.pop()
			if err := setInto(container, index, value); err != nil {
				return nil, vm.errorf("%s", err)
			}
			// Assignment is an expression, so the value stays on the stack.
			vm.push(value)

		case OpGetField:
			index := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			value, err := getField(vm.pop(), fieldName(vm.constants[index]))
			if err != nil {
				return nil, vm.errorf("%s", err)
			}
			vm.push(value)

		case OpSetField:
			index := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			value := vm.pop()
			container := vm.pop()
			if err := setField(container, fieldName(vm.constants[index]), value); err != nil {
				return nil, vm.errorf("%s", err)
			}
			// Assignment is an expression, so the value stays on the stack.
			vm.push(value)

		case OpDefineMethod:
			typeIndex := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			nameIndex := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			method := vm.pop()
			st := vm.constants[typeIndex].(*StructType)
			if st.Methods == nil {
				st.Methods = make(map[string]Object)
			}
			st.Methods[fieldName(vm.constants[nameIndex])] = method

		case OpCall:
			argc := int(ReadUint8(ins[frame.ip:]))
			frame.ip++
			if err := vm.call(argc); err != nil {
				return nil, err
			}
			// The call may have started a new frame.
			frame = vm.frames[vm.frameTop-1]
			ins = frame.cl.Fn.Instructions

		case OpCloseUpvalues:
			from := int(ReadUint16(ins[frame.ip:]))
			frame.ip += 2
			vm.closeCells(frame.basePointer + from)

		case OpReturnValue:
			result := vm.pop()
			// The frame is about to go, so any variable a closure captured has
			// to be copied off the stack before its slots are reused.
			vm.closeCells(frame.basePointer)
			done := vm.popFrame()
			if vm.frameTop == 0 {
				// The script has finished.
				vm.sp = 0
				continue
			}
			// Drop the callee and its arguments, then push the result.
			vm.sp = done.basePointer - 1
			vm.push(result)
			frame = vm.frames[vm.frameTop-1]
			ins = frame.cl.Fn.Instructions

		default:
			return nil, fmt.Errorf("unknown opcode %d", op)
		}
	}

	return vm.lastPopped, nil
}

// ---------------------------------------------------------------- captures

// capture returns the cell for one local slot, making one the first time a
// closure wants it. A second closure capturing the same slot gets the same
// cell, so either one sees what the other writes.
func (vm *VM) capture(slot int) *cell {
	// The list is short and kept in order, so one backwards scan finds a match
	// and, failing that, the place to insert.
	i := len(vm.openCells)
	for i > 0 && vm.openCells[i-1].index > slot {
		i--
	}
	if i > 0 && vm.openCells[i-1].index == slot {
		return vm.openCells[i-1]
	}

	vm.openCells = append(vm.openCells, nil)
	copy(vm.openCells[i+1:], vm.openCells[i:])
	vm.openCells[i] = &cell{vm: vm, index: slot}
	return vm.openCells[i]
}

// closeCells copies the values of every cell at or above a slot back off the
// stack and forgets them, so the closures that hold them keep working once the
// frame or block that owned those slots is gone. Only cells at or above the
// boundary are touched: anything below belongs to a scope that is still open.
func (vm *VM) closeCells(from int) {
	cut := len(vm.openCells)
	for cut > 0 && vm.openCells[cut-1].index >= from {
		cut--
	}
	for _, c := range vm.openCells[cut:] {
		c.close()
	}
	vm.openCells = vm.openCells[:cut]
}

// ---------------------------------------------------------------- calling

func (vm *VM) call(argc int) error {
	calleeIndex := vm.sp - 1 - argc
	callee := vm.stack[calleeIndex]

	switch fn := callee.(type) {

	case *Closure:
		if argc != fn.Fn.NumParams {
			name := fn.Fn.Name
			if name == "" {
				name = "fn"
			}
			return vm.errorf("%s() expects %d argument(s), got %d",
				name, fn.Fn.NumParams, argc)
		}

		basePointer := calleeIndex + 1
		if err := vm.pushFrame(fn, basePointer); err != nil {
			return err
		}

		// Make room for the frame's locals and start them empty.
		need := basePointer + fn.Fn.NumLocals
		for vm.sp < need {
			vm.push(nilObject)
		}
		return nil

	case *Builtin:
		args := make([]Object, argc)
		for i := argc - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop() // the built-in itself

		result, err := fn.Fn(args)
		if err != nil {
			return vm.errorf("%s", err)
		}
		vm.push(result)
		return nil

	case *StructType:
		// A record type is called to make an instance, the way a class is.
		args := make([]Object, argc)
		for i := argc - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop() // the type itself

		value, err := newStruct(fn, args)
		if err != nil {
			return vm.errorf("%s", err)
		}
		vm.push(value)
		return nil

	case *BoundMethod:
		vm.push(nilObject)
		for i := vm.sp - 1; i > calleeIndex+1; i-- {
			vm.stack[i] = vm.stack[i-1]
		}
		vm.stack[calleeIndex+1] = fn.Receiver
		vm.stack[calleeIndex] = fn.Method
		return vm.call(argc + 1)
	}

	return vm.errorf("%s is not callable", callee.Type())
}

// pushFrame starts a call. Frames come from the pre-allocated pool, so no call
// allocates one.
func (vm *VM) pushFrame(cl *Closure, basePointer int) error {
	if vm.frameTop >= len(vm.frames) {
		return vm.errorf("too many nested calls (more than %d)", maxFrames)
	}
	frame := vm.frames[vm.frameTop]
	frame.cl = cl
	frame.ip = 0
	frame.basePointer = basePointer
	vm.frameTop++
	return nil
}

func (vm *VM) popFrame() *Frame {
	vm.frameTop--
	return vm.frames[vm.frameTop]
}

// ---------------------------------------------------------------- stack

func (vm *VM) push(obj Object) {
	if vm.sp < len(vm.stack) {
		vm.stack[vm.sp] = obj
	} else {
		vm.stack = append(vm.stack, obj)
	}
	vm.sp++
}

func (vm *VM) pop() Object {
	if vm.sp == 0 {
		return nilObject
	}
	vm.sp--
	return vm.stack[vm.sp]
}

func (vm *VM) peek() Object {
	if vm.sp == 0 {
		return nilObject
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) getGlobal(index int) Object {
	if index >= len(vm.globals) {
		return nilObject
	}
	return vm.globals[index]
}

func (vm *VM) setGlobal(index int, obj Object) {
	for len(vm.globals) <= index {
		vm.globals = append(vm.globals, nilObject)
	}
	vm.globals[index] = obj
}

// ---------------------------------------------------------------- errors

// errorf builds an error that points at the instruction being run.
func (vm *VM) errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	if vm.frameTop == 0 {
		return fmt.Errorf("%s", msg)
	}

	frame := vm.frames[vm.frameTop-1]
	at := frame.ip - 1
	if at >= 0 && at < len(frame.cl.Fn.Positions) {
		pos := frame.cl.Fn.Positions[at]
		if pos.Line > 0 {
			if pos.File != "" {
				return fmt.Errorf("%s:%d:%d: %s", pos.File, pos.Line, pos.Col, msg)
			}
			return fmt.Errorf("%d:%d: %s", pos.Line, pos.Col, msg)
		}
	}
	return fmt.Errorf("%s", msg)
}

// ---------------------------------------------------------------- operators

// The fast helpers below handle the common cases without building a position
// or calling out to the message helpers. When they report no result, the
// caller falls back to the shared helpers in interpreter.go, which produce the
// exact error text the tree-walking back end produces.

// fastAdd mirrors addValues for the cases that succeed.
func fastAdd(left, right Object) (Object, bool) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return newInteger(l.Value + r.Value), true
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return newFloat(l.Value + r.Value), true
		}
	case *String:
		if r, ok := right.(*String); ok {
			return &String{Value: l.Value + r.Value}, true
		}
	case *Array:
		if r, ok := right.(*Array); ok {
			elements := make([]Object, 0, len(l.Elements)+len(r.Elements))
			elements = append(elements, l.Elements...)
			elements = append(elements, r.Elements...)
			return &Array{Elements: elements}, true
		}
	}
	return nil, false
}

// fastArith mirrors arithValues for the cases that succeed. Division by zero
// falls through so the shared helper can report it.
func fastArith(op Opcode, left, right Object) (Object, bool) {
	switch l := left.(type) {
	case *Integer:
		r, ok := right.(*Integer)
		if !ok {
			return nil, false
		}
		switch op {
		case OpSub:
			return newInteger(l.Value - r.Value), true
		case OpMul:
			return newInteger(l.Value * r.Value), true
		case OpDiv:
			if r.Value == 0 {
				return nil, false
			}
			return newInteger(l.Value / r.Value), true
		case OpMod:
			if r.Value == 0 {
				return nil, false
			}
			return newInteger(l.Value % r.Value), true
		}
	case *Float:
		r, ok := right.(*Float)
		if !ok {
			return nil, false
		}
		switch op {
		case OpSub:
			return newFloat(l.Value - r.Value), true
		case OpMul:
			return newFloat(l.Value * r.Value), true
		case OpDiv:
			if r.Value == 0 {
				return nil, false
			}
			return newFloat(l.Value / r.Value), true
		case OpMod:
			if r.Value == 0 {
				return nil, false
			}
			return newFloat(math.Mod(l.Value, r.Value)), true
		}
	}
	return nil, false
}

// fastCompare mirrors compareValues for the cases that succeed.
func fastCompare(op Opcode, left, right Object) (Object, bool) {
	switch l := left.(type) {
	case *Integer:
		if r, ok := right.(*Integer); ok {
			return newBoolean(compareOrder(orderOpName(op), float64(l.Value), float64(r.Value))), true
		}
	case *Float:
		if r, ok := right.(*Float); ok {
			return newBoolean(compareOrder(orderOpName(op), l.Value, r.Value)), true
		}
	case *String:
		if r, ok := right.(*String); ok {
			return newBoolean(compareStrings(orderOpName(op), l.Value, r.Value)), true
		}
	}
	return nil, false
}

// numericOpName turns an arithmetic opcode into the operator text the shared
// interpreter helpers use in their error messages.
func numericOpName(op Opcode) string {
	switch op {
	case OpSub:
		return "-"
	case OpMul:
		return "*"
	case OpDiv:
		return "/"
	}
	return "%"
}

// orderOpName does the same for the comparison opcodes.
func orderOpName(op Opcode) string {
	switch op {
	case OpGreater:
		return ">"
	case OpGreaterEqual:
		return ">="
	case OpLess:
		return "<"
	}
	return "<="
}

// tokenHere is the source position of the instruction being run, so the
// message helpers can report line and column just like the interpreter does.
func (vm *VM) tokenHere() Token {
	if vm.frameTop == 0 {
		return Token{}
	}
	frame := vm.frames[vm.frameTop-1]
	at := frame.ip - 1
	if at >= 0 && at < len(frame.cl.Fn.Positions) {
		pos := frame.cl.Fn.Positions[at]
		return Token{Line: pos.Line, Column: pos.Col, File: pos.File}
	}
	return Token{}
}

// checker only lets bools reach here, but the VM is still careful.
func isTruthy(obj Object) bool {
	b, ok := obj.(*Boolean)
	return ok && b.Value
}

// ---------------------------------------------------------------- collections

// indexInto reads container[index]. Both back ends go through this, so an
// indexed read and its error text cannot drift apart.
func indexInto(container, index Object) (Object, error) {
	if m, ok := container.(*Map); ok {
		value, found, err := m.Get(index)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("key %s is not in the map", repr(index))
		}
		return value, nil
	}

	n, ok := index.(*Integer)
	if !ok {
		return nil, fmt.Errorf("index must be an int, got %s", index.Type())
	}

	switch c := container.(type) {
	case *Array:
		if n.Value < 0 || n.Value >= int64(len(c.Elements)) {
			return nil, fmt.Errorf("index %d out of range (length %d)", n.Value, len(c.Elements))
		}
		return c.Elements[n.Value], nil
	case *String:
		runes := []rune(c.Value)
		if n.Value < 0 || n.Value >= int64(len(runes)) {
			return nil, fmt.Errorf("index %d out of range (length %d)", n.Value, len(runes))
		}
		return &String{Value: string(runes[n.Value])}, nil
	}

	return nil, fmt.Errorf("cannot index %s", container.Type())
}

// setInto carries out container[index] = value.
func setInto(container, index, value Object) error {
	switch c := container.(type) {
	case *Map:
		return c.Set(index, value)

	case *Array:
		n, ok := index.(*Integer)
		if !ok {
			return fmt.Errorf("index must be an int, got %s", index.Type())
		}
		if n.Value < 0 || n.Value >= int64(len(c.Elements)) {
			return fmt.Errorf("index %d out of range (length %d)", n.Value, len(c.Elements))
		}
		c.Elements[n.Value] = value
		return nil
	}

	return fmt.Errorf("cannot assign into %s", container.Type())
}

// iterateValues turns a collection into the values a for loop walks: an array
// stays as it is, a string becomes its characters, and a map becomes its keys
// in the order they were added.
func iterateValues(obj Object) (Object, error) {
	switch v := obj.(type) {
	case *Array:
		return v, nil
	case *String:
		runes := []rune(v.Value)
		items := make([]Object, len(runes))
		for i, r := range runes {
			items[i] = &String{Value: string(r)}
		}
		return &Array{Elements: items}, nil
	case *Map:
		return &Array{Elements: v.Keys()}, nil
	}
	return nil, fmt.Errorf("cannot loop over %s", obj.Type())
}

func lengthOf(obj Object) (int64, error) {
	switch v := obj.(type) {
	case *String:
		return int64(utf8.RuneCountInString(v.Value)), nil
	case *Array:
		return int64(len(v.Elements)), nil
	case *Map:
		return int64(v.Len()), nil
	}
	return 0, fmt.Errorf("cannot take the length of %s", obj.Type())
}

func joinStrings(parts []string) string {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	buf := make([]byte, 0, total)
	for _, p := range parts {
		buf = append(buf, p...)
	}
	return string(buf)
}
