package main

import (
	"fmt"
	"strconv"
)

// ObjectType names a runtime value's type.
type ObjectType string

const (
	INTEGER_OBJ  ObjectType = "int"
	FLOAT_OBJ    ObjectType = "float"
	STRING_OBJ   ObjectType = "str"
	BOOLEAN_OBJ  ObjectType = "bool"
	NIL_OBJ      ObjectType = "nil"
	ARRAY_OBJ    ObjectType = "array"
	MAP_OBJ      ObjectType = "map"
	FUNCTION_OBJ ObjectType = "fn"
	BUILTIN_OBJ  ObjectType = "builtin"
	RETURN_OBJ   ObjectType = "return"
	BREAK_OBJ    ObjectType = "break"
	CONTINUE_OBJ ObjectType = "continue"
)

// Object is any value produced while a Jin program runs.
type Object interface {
	Type() ObjectType
	Inspect() string
}

// Small values are shared instead of allocated. Arithmetic produces a value on
// every step, so allocating each one is the single biggest cost in a loop.

var (
	nilObject   = &Nil{}
	trueObject  = &Boolean{Value: true}
	falseObject = &Boolean{Value: false}
)

const (
	smallIntMin = -1024
	smallIntMax = 8192
)

var smallInts [smallIntMax - smallIntMin + 1]Integer

func init() {
	for i := range smallInts {
		smallInts[i].Value = int64(i + smallIntMin)
	}
}

// newInteger returns a shared value for small numbers, and a fresh one for the
// rest. Integers never change, so sharing them is safe.
func newInteger(v int64) *Integer {
	if v >= smallIntMin && v <= smallIntMax {
		return &smallInts[v-smallIntMin]
	}
	return &Integer{Value: v}
}

// newFloat returns a new float value. Floats are not pooled: most programs
// produce more distinct values than any pool would cover.
func newFloat(v float64) *Float { return &Float{Value: v} }

// newBoolean returns the shared true or false.
func newBoolean(b bool) *Boolean {
	if b {
		return trueObject
	}
	return falseObject
}

// newNil returns the shared nil.
func newNil() *Nil { return nilObject }

// Integer is a whole number.
type Integer struct{ Value int64 }

func (o *Integer) Type() ObjectType { return INTEGER_OBJ }
func (o *Integer) Inspect() string  { return strconv.FormatInt(o.Value, 10) }

// Float is a decimal number.
type Float struct{ Value float64 }

func (o *Float) Type() ObjectType { return FLOAT_OBJ }
func (o *Float) Inspect() string  { return strconv.FormatFloat(o.Value, 'g', -1, 64) }

// String is text.
type String struct {
	Value   string
	IsError bool // set when this string originated from a caught runtime error
}

func (o *String) Type() ObjectType { return STRING_OBJ }
func (o *String) Inspect() string  { return o.Value }

// LocatedError wraps an error that already has a source position prefix,
// preventing duplicate prefixes when errors propagate or are re-raised.
type LocatedError struct {
	Msg string
}

func (e *LocatedError) Error() string { return e.Msg }

// Boolean is true or false.
type Boolean struct{ Value bool }

func (o *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (o *Boolean) Inspect() string  { return strconv.FormatBool(o.Value) }

// Nil is the absence of a value.
type Nil struct{}

func (o *Nil) Type() ObjectType { return NIL_OBJ }
func (o *Nil) Inspect() string  { return "nil" }

// Array is an ordered list of values.
type Array struct{ Elements []Object }

func (o *Array) Type() ObjectType { return ARRAY_OBJ }
func (o *Array) Inspect() string {
	parts := make([]string, 0, len(o.Elements))
	for _, el := range o.Elements {
		parts = append(parts, repr(el))
	}
	return "[" + joinWith(parts, ", ") + "]"
}

// MapEntry is one key and value in a map.
type MapEntry struct {
	Key   Object
	Value Object
}

// Map is a dictionary. Entries keep the order they were added in, so printing
// a map and looping over one are predictable; a small index on the side keeps
// lookups from scanning every entry.
type Map struct {
	Entries []MapEntry
	index   map[string]int
}

func (o *Map) Type() ObjectType { return MAP_OBJ }
func (o *Map) Inspect() string {
	parts := make([]string, 0, len(o.Entries))
	for _, e := range o.Entries {
		parts = append(parts, repr(e.Key)+": "+repr(e.Value))
	}
	return "{" + joinWith(parts, ", ") + "}"
}

// Len is how many entries the map holds.
func (o *Map) Len() int { return len(o.Entries) }

// Keys returns the keys in insertion order.
func (o *Map) Keys() []Object {
	keys := make([]Object, len(o.Entries))
	for i, e := range o.Entries {
		keys[i] = e.Key
	}
	return keys
}

// Get looks up a key. The second result reports whether the key was there.
func (o *Map) Get(key Object) (Object, bool, error) {
	text, err := mapKeyText(key)
	if err != nil {
		return nil, false, err
	}
	if o.index == nil {
		return nil, false, nil
	}
	at, ok := o.index[text]
	if !ok {
		return nil, false, nil
	}
	return o.Entries[at].Value, true, nil
}

// Set stores a value under a key, replacing any previous one.
func (o *Map) Set(key, value Object) error {
	text, err := mapKeyText(key)
	if err != nil {
		return err
	}
	if o.index == nil {
		o.index = map[string]int{}
	}
	if at, ok := o.index[text]; ok {
		o.Entries[at].Value = value
		return nil
	}
	o.index[text] = len(o.Entries)
	o.Entries = append(o.Entries, MapEntry{Key: key, Value: value})
	return nil
}

// Delete removes a key. The result reports whether the key was there.
func (o *Map) Delete(key Object) (bool, error) {
	text, err := mapKeyText(key)
	if err != nil {
		return false, err
	}
	at, ok := o.index[text]
	if !ok {
		return false, nil
	}

	o.Entries = append(o.Entries[:at], o.Entries[at+1:]...)
	delete(o.index, text)
	// Everything after the hole moved down one place.
	for i := at; i < len(o.Entries); i++ {
		moved, err := mapKeyText(o.Entries[i].Key)
		if err != nil {
			return false, err
		}
		o.index[moved] = i
	}
	return true, nil
}

// mapKeyText turns a key into the string the internal index is built from.
// Only values that can be compared exactly are allowed as keys.
func mapKeyText(key Object) (string, error) {
	switch k := key.(type) {
	case *Integer:
		return "i" + strconv.FormatInt(k.Value, 10), nil
	case *String:
		return "s" + k.Value, nil
	case *Boolean:
		return "b" + strconv.FormatBool(k.Value), nil
	}
	return "", fmt.Errorf("cannot use %s as a map key; keys must be int, str or bool", key.Type())
}

// Function is a user-defined function together with its captured scope.
type Function struct {
	Name       string
	Parameters []*Parameter
	Body       *BlockStatement
	Env        *Environment
}

func (o *Function) Type() ObjectType { return FUNCTION_OBJ }
func (o *Function) Inspect() string {
	if o.Name == "" {
		return "<fn>"
	}
	return "<fn " + o.Name + ">"
}

// Caller is an interface for calling a Jin function from a Go built-in.
type Caller interface {
	Call(fn Object, args []Object) (Object, error)
}

// Builtin is a function implemented in Go.
type Builtin struct {
	Name string
	Fn   func(args []Object) (Object, error)
}

func (o *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (o *Builtin) Inspect() string  { return "<builtin " + o.Name + ">" }

// ---- compiled code

// CompiledFunction is a function compiled to bytecode. It holds no captured
// state of its own, so one CompiledFunction can back many closures.
type CompiledFunction struct {
	Name         string
	Instructions Instructions
	Positions    []Position // where each byte came from, for error messages
	NumLocals    int        // slots the frame needs, parameters included
	NumParams    int
}

func (o *CompiledFunction) Type() ObjectType { return FUNCTION_OBJ }
func (o *CompiledFunction) Inspect() string {
	if o.Name == "" {
		return "<fn>"
	}
	return "<fn " + o.Name + ">"
}

// Closure pairs a compiled function with the variables it captured, so a
// function can keep using values from the scope where it was written.
type Closure struct {
	Fn   *CompiledFunction
	Free []*cell
}

func (o *Closure) Type() ObjectType { return FUNCTION_OBJ }
func (o *Closure) Inspect() string  { return o.Fn.Inspect() }

// cell is one captured variable, kept where every closure that captured it can
// reach the same copy. While the frame that owns the variable is still on the
// stack the cell points straight at that frame's slot, so an ordinary read or
// write of the local is seen by every closure. When the scope ends the value is
// copied out of the slot, which is what lets a closure outlive its frame.
//
// This is what makes two closures made in one scope share a variable, and lets
// a closure capture a variable that has not been assigned yet.
type cell struct {
	// The machine holding the slot. The slot is reached through it rather than
	// cached, because the stack slice is replaced whenever the stack grows.
	vm *VM

	index  int    // where in that stack the variable lives
	value  Object // the value, once the slot is gone
	closed bool
}

func (c *cell) get() Object {
	if c.closed {
		return c.value
	}
	return c.vm.stack[c.index]
}

func (c *cell) set(value Object) {
	if c.closed {
		c.value = value
		return
	}
	c.vm.stack[c.index] = value
}

// close copies the value out of the slot, so the cell keeps working after the
// frame that owned the variable has gone.
func (c *cell) close() {
	if c.closed {
		return
	}
	c.value = c.vm.stack[c.index]
	c.vm = nil
	c.closed = true
}

// ---- control-flow signals. These travel up the tree as ordinary values so
// that return/break/continue are not confused with real errors.

// ReturnValue wraps a value travelling out of a function body.
type ReturnValue struct{ Value Object }

func (o *ReturnValue) Type() ObjectType { return RETURN_OBJ }
func (o *ReturnValue) Inspect() string  { return o.Value.Inspect() }

// BreakSignal exits the nearest loop.
type BreakSignal struct{}

func (o *BreakSignal) Type() ObjectType { return BREAK_OBJ }
func (o *BreakSignal) Inspect() string  { return "break" }

// ContinueSignal skips to the next loop iteration.
type ContinueSignal struct{}

func (o *ContinueSignal) Type() ObjectType { return CONTINUE_OBJ }
func (o *ContinueSignal) Inspect() string  { return "continue" }

// ---- helpers

// repr renders a value the way it should appear inside a collection, so
// strings keep their quotes.
func repr(o Object) string {
	if s, ok := o.(*String); ok {
		return strconv.Quote(s.Value)
	}
	return o.Inspect()
}

func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// isControlFlow reports whether an object is a return/break/continue signal.
func isControlFlow(o Object) bool {
	switch o.(type) {
	case *ReturnValue, *BreakSignal, *ContinueSignal:
		return true
	}
	return false
}

// controlFlowError explains a misplaced return/break/continue.
func controlFlowError(o Object) error {
	switch o.(type) {
	case *BreakSignal:
		return fmt.Errorf("'break' is only allowed inside a loop")
	case *ContinueSignal:
		return fmt.Errorf("'continue' is only allowed inside a loop")
	case *ReturnValue:
		return fmt.Errorf("'return' is only allowed inside a function")
	}
	return fmt.Errorf("unexpected control flow")
}
