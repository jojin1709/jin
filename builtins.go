package main

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// builtinNames lists the built-ins in the order that OpGetBuiltin indexes
// them. The compiler and the VM both read this, so they cannot disagree.
var builtinNames = []string{
	"print", "len", "push", "str", "int", "float", "type", "range",
	"upper", "lower", "trim", "split", "join", "contains", "abs", "min", "max",
}

// newBuiltins returns the functions Jin provides out of the box. They are
// shared by the tree-walking interpreter and the bytecode VM, so they only
// depend on where output goes.
func newBuiltins(out io.Writer) map[string]*Builtin {
	b := map[string]*Builtin{}

	b["print"] = &Builtin{Name: "print", Fn: func(args []Object) (Object, error) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, a.Inspect())
		}
		fmt.Fprintln(out, strings.Join(parts, " "))
		return &Nil{}, nil
	}}

	b["len"] = &Builtin{Name: "len", Fn: func(args []Object) (Object, error) {
		if err := arity("len", args, 1); err != nil {
			return nil, err
		}
		switch v := args[0].(type) {
		case *String:
			return newInteger(int64(utf8.RuneCountInString(v.Value))), nil
		case *Array:
			return newInteger(int64(len(v.Elements))), nil
		}
		return nil, typeError("len", args[0], "str or array")
	}}

	b["push"] = &Builtin{Name: "push", Fn: func(args []Object) (Object, error) {
		if err := arity("push", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("push", args[0], "array")
		}
		elems := make([]Object, 0, len(arr.Elements)+1)
		elems = append(elems, arr.Elements...)
		elems = append(elems, args[1])
		return &Array{Elements: elems}, nil
	}}

	b["str"] = &Builtin{Name: "str", Fn: func(args []Object) (Object, error) {
		if err := arity("str", args, 1); err != nil {
			return nil, err
		}
		return &String{Value: args[0].Inspect()}, nil
	}}

	b["int"] = &Builtin{Name: "int", Fn: func(args []Object) (Object, error) {
		if err := arity("int", args, 1); err != nil {
			return nil, err
		}
		switch v := args[0].(type) {
		case *Integer:
			return v, nil
		case *Float:
			return newInteger(int64(v.Value)), nil
		case *Boolean:
			if v.Value {
				return newInteger(1), nil
			}
			return newInteger(0), nil
		case *String:
			n, err := strconv.ParseInt(strings.TrimSpace(v.Value), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("int(): cannot convert %q to an int", v.Value)
			}
			return newInteger(n), nil
		}
		return nil, typeError("int", args[0], "int, float, bool or str")
	}}

	b["float"] = &Builtin{Name: "float", Fn: func(args []Object) (Object, error) {
		if err := arity("float", args, 1); err != nil {
			return nil, err
		}
		switch v := args[0].(type) {
		case *Float:
			return v, nil
		case *Integer:
			return newFloat(float64(v.Value)), nil
		case *String:
			f, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64)
			if err != nil {
				return nil, fmt.Errorf("float(): cannot convert %q to a float", v.Value)
			}
			return newFloat(f), nil
		}
		return nil, typeError("float", args[0], "int, float or str")
	}}

	b["type"] = &Builtin{Name: "type", Fn: func(args []Object) (Object, error) {
		if err := arity("type", args, 1); err != nil {
			return nil, err
		}
		return &String{Value: string(args[0].Type())}, nil
	}}

	b["range"] = &Builtin{Name: "range", Fn: func(args []Object) (Object, error) {
		var start, end int64
		switch len(args) {
		case 1:
			n, err := asInt("range", args[0])
			if err != nil {
				return nil, err
			}
			start, end = 0, n
		case 2:
			a, err := asInt("range", args[0])
			if err != nil {
				return nil, err
			}
			bb, err := asInt("range", args[1])
			if err != nil {
				return nil, err
			}
			start, end = a, bb
		default:
			return nil, fmt.Errorf("range() takes 1 or 2 arguments, got %d", len(args))
		}
		elems := []Object{}
		for n := start; n < end; n++ {
			elems = append(elems, newInteger(n))
		}
		return &Array{Elements: elems}, nil
	}}

	b["upper"] = &Builtin{Name: "upper", Fn: func(args []Object) (Object, error) {
		if err := arity("upper", args, 1); err != nil {
			return nil, err
		}
		s, ok := args[0].(*String)
		if !ok {
			return nil, typeError("upper", args[0], "str")
		}
		return &String{Value: strings.ToUpper(s.Value)}, nil
	}}

	b["lower"] = &Builtin{Name: "lower", Fn: func(args []Object) (Object, error) {
		if err := arity("lower", args, 1); err != nil {
			return nil, err
		}
		s, ok := args[0].(*String)
		if !ok {
			return nil, typeError("lower", args[0], "str")
		}
		return &String{Value: strings.ToLower(s.Value)}, nil
	}}

	b["trim"] = &Builtin{Name: "trim", Fn: func(args []Object) (Object, error) {
		if err := arity("trim", args, 1); err != nil {
			return nil, err
		}
		s, ok := args[0].(*String)
		if !ok {
			return nil, typeError("trim", args[0], "str")
		}
		return &String{Value: strings.TrimSpace(s.Value)}, nil
	}}

	b["split"] = &Builtin{Name: "split", Fn: func(args []Object) (Object, error) {
		if err := arity("split", args, 2); err != nil {
			return nil, err
		}
		s, ok := args[0].(*String)
		if !ok {
			return nil, typeError("split", args[0], "str")
		}
		sep, ok := args[1].(*String)
		if !ok {
			return nil, typeError("split", args[1], "str")
		}
		parts := strings.Split(s.Value, sep.Value)
		elems := make([]Object, 0, len(parts))
		for _, p := range parts {
			elems = append(elems, &String{Value: p})
		}
		return &Array{Elements: elems}, nil
	}}

	b["join"] = &Builtin{Name: "join", Fn: func(args []Object) (Object, error) {
		if err := arity("join", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("join", args[0], "array")
		}
		sep, ok := args[1].(*String)
		if !ok {
			return nil, typeError("join", args[1], "str")
		}
		parts := make([]string, 0, len(arr.Elements))
		for _, el := range arr.Elements {
			parts = append(parts, el.Inspect())
		}
		return &String{Value: strings.Join(parts, sep.Value)}, nil
	}}

	b["contains"] = &Builtin{Name: "contains", Fn: func(args []Object) (Object, error) {
		if err := arity("contains", args, 2); err != nil {
			return nil, err
		}
		switch c := args[0].(type) {
		case *String:
			sub, ok := args[1].(*String)
			if !ok {
				return nil, typeError("contains", args[1], "str")
			}
			return newBoolean(strings.Contains(c.Value, sub.Value)), nil
		case *Array:
			for _, el := range c.Elements {
				if objectsEqual(el, args[1]) {
					return newBoolean(true), nil
				}
			}
			return newBoolean(false), nil
		}
		return nil, typeError("contains", args[0], "str or array")
	}}

	b["abs"] = &Builtin{Name: "abs", Fn: func(args []Object) (Object, error) {
		if err := arity("abs", args, 1); err != nil {
			return nil, err
		}
		switch v := args[0].(type) {
		case *Integer:
			if v.Value < 0 {
				return newInteger(-v.Value), nil
			}
			return v, nil
		case *Float:
			return newFloat(math.Abs(v.Value)), nil
		}
		return nil, typeError("abs", args[0], "int or float")
	}}

	b["min"] = &Builtin{Name: "min", Fn: func(args []Object) (Object, error) {
		return minMax("min", args, true)
	}}

	b["max"] = &Builtin{Name: "max", Fn: func(args []Object) (Object, error) {
		return minMax("max", args, false)
	}}

	return b
}

// ---------------------------------------------------------------- helpers

func arity(name string, args []Object, want int) error {
	if len(args) != want {
		return fmt.Errorf("%s() takes %d argument(s), got %d", name, want, len(args))
	}
	return nil
}

func typeError(fn string, got Object, want string) error {
	return fmt.Errorf("%s() expects %s, got %s", fn, want, got.Type())
}

func asInt(fn string, o Object) (int64, error) {
	if n, ok := o.(*Integer); ok {
		return n.Value, nil
	}
	return 0, typeError(fn, o, "int")
}

func minMax(name string, args []Object, wantMin bool) (Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%s() needs at least one argument", name)
	}

	allInts := true
	for _, a := range args {
		if _, ok := a.(*Integer); !ok {
			allInts = false
			break
		}
	}

	if allInts {
		best := args[0].(*Integer).Value
		for _, a := range args[1:] {
			v := a.(*Integer).Value
			if (wantMin && v < best) || (!wantMin && v > best) {
				best = v
			}
		}
		return newInteger(best), nil
	}

	best := 0.0
	for idx, a := range args {
		var v float64
		switch n := a.(type) {
		case *Float:
			v = n.Value
		case *Integer:
			v = float64(n.Value)
		default:
			return nil, typeError(name, a, "int or float")
		}
		if idx == 0 || (wantMin && v < best) || (!wantMin && v > best) {
			best = v
		}
	}
	return newFloat(best), nil
}
