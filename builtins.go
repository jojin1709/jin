package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// builtinNames lists the built-ins in the order that OpGetBuiltin indexes
// them. The compiler and the VM both read this, so they cannot disagree.
var builtinNames = []string{
	"print", "len", "push", "str", "int", "float", "type", "range",
	"upper", "lower", "trim", "split", "join", "contains", "abs", "min", "max",
	"has", "get", "delete", "keys", "values",
	"sort", "reverse", "slice",
	"replace", "find", "starts_with", "ends_with", "repeat", "chars", "lines",
	"floor", "ceil", "round", "sqrt", "pow", "random", "error",
	"read_file", "write_file", "append_file", "file_exists", "delete_file",
	"args",
	"input", "read_all_stdin",
	"map", "filter", "reduce", "any", "all", "sort_by", "find_index",
}

// newBuiltins returns the functions Jin provides out of the box. They are
// shared by the tree-walking interpreter and the bytecode VM, so they only
// depend on where output goes and on the arguments the program was run with.
func newBuiltins(in io.Reader, out io.Writer, programArgs []string, caller Caller) map[string]*Builtin {
	b := map[string]*Builtin{}

	if in == nil {
		in = os.Stdin
	}
	stdinReader := bufio.NewReader(in)

	// args() is the arguments the program was run with, so a Jin program can
	// be used as a command.
	b["args"] = &Builtin{Name: "args", Fn: func(args []Object) (Object, error) {
		if err := arity("args", args, 0); err != nil {
			return nil, err
		}
		elems := make([]Object, 0, len(programArgs))
		for _, arg := range programArgs {
			elems = append(elems, &String{Value: arg})
		}
		return &Array{Elements: elems}, nil
	}}

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
		n, err := lengthOf(args[0])
		if err != nil {
			return nil, err
		}
		return newInteger(n), nil
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
		case *Map:
			_, found, err := c.Get(args[1])
			if err != nil {
				return nil, err
			}
			return newBoolean(found), nil
		}
		return nil, typeError("contains", args[0], "str, array or map")
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

	// ------------------------------------------------------------- maps

	b["has"] = &Builtin{Name: "has", Fn: func(args []Object) (Object, error) {
		if err := arity("has", args, 2); err != nil {
			return nil, err
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, typeError("has", args[0], "map")
		}
		_, found, err := m.Get(args[1])
		if err != nil {
			return nil, err
		}
		return newBoolean(found), nil
	}}

	b["get"] = &Builtin{Name: "get", Fn: func(args []Object) (Object, error) {
		if err := arity("get", args, 3); err != nil {
			return nil, err
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, typeError("get", args[0], "map")
		}
		value, found, err := m.Get(args[1])
		if err != nil {
			return nil, err
		}
		if !found {
			return args[2], nil
		}
		return value, nil
	}}

	b["delete"] = &Builtin{Name: "delete", Fn: func(args []Object) (Object, error) {
		if err := arity("delete", args, 2); err != nil {
			return nil, err
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, typeError("delete", args[0], "map")
		}
		if _, err := m.Delete(args[1]); err != nil {
			return nil, err
		}
		return nilObject, nil
	}}

	b["keys"] = &Builtin{Name: "keys", Fn: func(args []Object) (Object, error) {
		if err := arity("keys", args, 1); err != nil {
			return nil, err
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, typeError("keys", args[0], "map")
		}
		return &Array{Elements: m.Keys()}, nil
	}}

	b["values"] = &Builtin{Name: "values", Fn: func(args []Object) (Object, error) {
		if err := arity("values", args, 1); err != nil {
			return nil, err
		}
		m, ok := args[0].(*Map)
		if !ok {
			return nil, typeError("values", args[0], "map")
		}
		values := make([]Object, len(m.Entries))
		for i, e := range m.Entries {
			values[i] = e.Value
		}
		return &Array{Elements: values}, nil
	}}

	// ------------------------------------------------------------- collections

	b["sort"] = &Builtin{Name: "sort", Fn: func(args []Object) (Object, error) {
		if err := arity("sort", args, 1); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("sort", args[0], "array")
		}
		sorted, err := sortedCopy(arr.Elements)
		if err != nil {
			return nil, err
		}
		return &Array{Elements: sorted}, nil
	}}

	b["reverse"] = &Builtin{Name: "reverse", Fn: func(args []Object) (Object, error) {
		if err := arity("reverse", args, 1); err != nil {
			return nil, err
		}
		switch v := args[0].(type) {
		case *Array:
			out := make([]Object, len(v.Elements))
			for i, el := range v.Elements {
				out[len(out)-1-i] = el
			}
			return &Array{Elements: out}, nil
		case *String:
			runes := []rune(v.Value)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return &String{Value: string(runes)}, nil
		}
		return nil, typeError("reverse", args[0], "array or str")
	}}

	b["slice"] = &Builtin{Name: "slice", Fn: func(args []Object) (Object, error) {
		if err := arity("slice", args, 3); err != nil {
			return nil, err
		}
		start, err := intArg("slice", args, 1)
		if err != nil {
			return nil, err
		}
		end, err := intArg("slice", args, 2)
		if err != nil {
			return nil, err
		}

		switch v := args[0].(type) {
		case *Array:
			from, to, err := sliceBounds(start, end, int64(len(v.Elements)))
			if err != nil {
				return nil, err
			}
			out := make([]Object, to-from)
			copy(out, v.Elements[from:to])
			return &Array{Elements: out}, nil
		case *String:
			runes := []rune(v.Value)
			from, to, err := sliceBounds(start, end, int64(len(runes)))
			if err != nil {
				return nil, err
			}
			return &String{Value: string(runes[from:to])}, nil
		}
		return nil, typeError("slice", args[0], "array or str")
	}}

	// ------------------------------------------------------------- strings

	b["replace"] = &Builtin{Name: "replace", Fn: func(args []Object) (Object, error) {
		if err := arity("replace", args, 3); err != nil {
			return nil, err
		}
		s, err := strArg("replace", args, 0)
		if err != nil {
			return nil, err
		}
		old, err := strArg("replace", args, 1)
		if err != nil {
			return nil, err
		}
		replacement, err := strArg("replace", args, 2)
		if err != nil {
			return nil, err
		}
		return &String{Value: strings.ReplaceAll(s, old, replacement)}, nil
	}}

	b["find"] = &Builtin{Name: "find", Fn: func(args []Object) (Object, error) {
		if err := arity("find", args, 2); err != nil {
			return nil, err
		}
		s, err := strArg("find", args, 0)
		if err != nil {
			return nil, err
		}
		sub, err := strArg("find", args, 1)
		if err != nil {
			return nil, err
		}
		// The result counts characters, not bytes, and -1 means "not there".
		at := strings.Index(s, sub)
		if at < 0 {
			return newInteger(-1), nil
		}
		return newInteger(int64(utf8.RuneCountInString(s[:at]))), nil
	}}

	for _, spec := range []struct {
		name string
		test func(s, affix string) bool
	}{
		{"starts_with", strings.HasPrefix},
		{"ends_with", strings.HasSuffix},
	} {
		name, test := spec.name, spec.test
		b[name] = &Builtin{Name: name, Fn: func(args []Object) (Object, error) {
			if err := arity(name, args, 2); err != nil {
				return nil, err
			}
			s, err := strArg(name, args, 0)
			if err != nil {
				return nil, err
			}
			affix, err := strArg(name, args, 1)
			if err != nil {
				return nil, err
			}
			return newBoolean(test(s, affix)), nil
		}}
	}

	b["repeat"] = &Builtin{Name: "repeat", Fn: func(args []Object) (Object, error) {
		if err := arity("repeat", args, 2); err != nil {
			return nil, err
		}
		s, err := strArg("repeat", args, 0)
		if err != nil {
			return nil, err
		}
		n, err := intArg("repeat", args, 1)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, fmt.Errorf("repeat() needs a count of 0 or more, got %d", n)
		}
		return &String{Value: strings.Repeat(s, int(n))}, nil
	}}

	b["chars"] = &Builtin{Name: "chars", Fn: func(args []Object) (Object, error) {
		if err := arity("chars", args, 1); err != nil {
			return nil, err
		}
		s, err := strArg("chars", args, 0)
		if err != nil {
			return nil, err
		}
		items := make([]Object, 0, len(s))
		for _, r := range s {
			items = append(items, &String{Value: string(r)})
		}
		return &Array{Elements: items}, nil
	}}

	b["lines"] = &Builtin{Name: "lines", Fn: func(args []Object) (Object, error) {
		if err := arity("lines", args, 1); err != nil {
			return nil, err
		}
		s, err := strArg("lines", args, 0)
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(s, "\r\n", "\n")
		text = strings.TrimSuffix(text, "\n")
		if text == "" {
			return &Array{Elements: []Object{}}, nil
		}
		parts := strings.Split(text, "\n")
		items := make([]Object, len(parts))
		for i, p := range parts {
			items[i] = &String{Value: p}
		}
		return &Array{Elements: items}, nil
	}}

	// ------------------------------------------------------------- numbers

	for _, spec := range []struct {
		name     string
		wholeNum bool // the answer is a whole number
		fn       func(float64) float64
	}{
		{"floor", true, math.Floor},
		{"ceil", true, math.Ceil},
		{"round", true, math.Round},
		{"sqrt", false, math.Sqrt},
	} {
		name, wholeNum, fn := spec.name, spec.wholeNum, spec.fn
		b[name] = &Builtin{Name: name, Fn: func(args []Object) (Object, error) {
			if err := arity(name, args, 1); err != nil {
				return nil, err
			}
			switch v := args[0].(type) {
			case *Integer:
				// A whole number is already whole, so floor, ceil and round
				// have nothing to do.
				if wholeNum {
					return v, nil
				}
				return newFloat(fn(float64(v.Value))), nil
			case *Float:
				if wholeNum {
					return newInteger(int64(fn(v.Value))), nil
				}
				return newFloat(fn(v.Value)), nil
			}
			return nil, typeError(name, args[0], "int or float")
		}}
	}

	b["pow"] = &Builtin{Name: "pow", Fn: func(args []Object) (Object, error) {
		if err := arity("pow", args, 2); err != nil {
			return nil, err
		}
		base, ok := numberValue(args[0])
		if !ok {
			return nil, typeError("pow", args[0], "int or float")
		}
		exponent, ok := numberValue(args[1])
		if !ok {
			return nil, typeError("pow", args[1], "int or float")
		}
		result := math.Pow(base, exponent)

		// Whole numbers raised to a whole power stay whole, so 2 to the 10
		// does not come back as 1024.0000000000002.
		if isWhole(args[0]) && isWhole(args[1]) && exponent >= 0 {
			return newInteger(int64(result)), nil
		}
		return newFloat(result), nil
	}}

	b["random"] = &Builtin{Name: "random", Fn: func(args []Object) (Object, error) {
		switch len(args) {
		case 0:
			return newFloat(rand.Float64()), nil
		case 1:
			bound, err := intArg("random", args, 0)
			if err != nil {
				return nil, err
			}
			if bound <= 0 {
				return nil, fmt.Errorf("random() needs a bound above 0, got %d", bound)
			}
			return newInteger(rand.Int63n(bound)), nil
		case 2:
			low, err := intArg("random", args, 0)
			if err != nil {
				return nil, err
			}
			high, err := intArg("random", args, 1)
			if err != nil {
				return nil, err
			}
			if high <= low {
				return nil, fmt.Errorf("random() needs a range with room in it, got %d up to %d", low, high)
			}
			return newInteger(low + rand.Int63n(high-low)), nil
		}
		return nil, fmt.Errorf("random() takes 0, 1 or 2 argument(s), got %d", len(args))
	}}

	b["error"] = &Builtin{Name: "error", Fn: func(args []Object) (Object, error) {
		if err := arity("error", args, 1); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", args[0].Inspect())
	}}

	// ------------------------------------------------------------- files

	b["read_file"] = &Builtin{Name: "read_file", Fn: func(args []Object) (Object, error) {
		if err := arity("read_file", args, 1); err != nil {
			return nil, err
		}
		path, err := strArg("read_file", args, 0)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read_file() could not read %s: %s", path, fileReason(err))
		}
		return &String{Value: string(data)}, nil
	}}

	b["write_file"] = &Builtin{Name: "write_file", Fn: func(args []Object) (Object, error) {
		return writeFileLike("write_file", args, func(path, text string) error {
			return os.WriteFile(path, []byte(text), 0o644)
		})
	}}

	b["append_file"] = &Builtin{Name: "append_file", Fn: func(args []Object) (Object, error) {
		return writeFileLike("append_file", args, func(path, text string) error {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.WriteString(text)
			return err
		})
	}}

	b["file_exists"] = &Builtin{Name: "file_exists", Fn: func(args []Object) (Object, error) {
		if err := arity("file_exists", args, 1); err != nil {
			return nil, err
		}
		path, err := strArg("file_exists", args, 0)
		if err != nil {
			return nil, err
		}
		_, statErr := os.Stat(path)
		if statErr == nil {
			return newBoolean(true), nil
		}
		if os.IsNotExist(statErr) {
			return newBoolean(false), nil
		}
		// Anything else means the path is there but cannot be looked at.
		return nil, fmt.Errorf("file_exists() could not look at %s: %s", path, fileReason(statErr))
	}}

	b["delete_file"] = &Builtin{Name: "delete_file", Fn: func(args []Object) (Object, error) {
		if err := arity("delete_file", args, 1); err != nil {
			return nil, err
		}
		path, err := strArg("delete_file", args, 0)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("delete_file() could not delete %s: %s", path, fileReason(err))
		}
		return nilObject, nil
	}}

	// ------------------------------------------------------------- stdin & io

	b["input"] = &Builtin{Name: "input", Fn: func(args []Object) (Object, error) {
		if err := arity("input", args, 1); err != nil {
			return nil, err
		}
		prompt, ok := args[0].(*String)
		if !ok {
			return nil, typeError("input", args[0], "str")
		}
		if out != nil {
			fmt.Fprint(out, prompt.Value)
			if f, ok := out.(interface{ Flush() error }); ok {
				_ = f.Flush()
			}
		}
		line, err := stdinReader.ReadString('\n')
		if err == io.EOF && len(line) == 0 {
			return nil, errors.New("end of input")
		}
		if err != nil && err != io.EOF {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		return &String{Value: line}, nil
	}}

	b["read_all_stdin"] = &Builtin{Name: "read_all_stdin", Fn: func(args []Object) (Object, error) {
		if err := arity("read_all_stdin", args, 0); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			return nil, err
		}
		return &String{Value: string(data)}, nil
	}}

	// ------------------------------------------------------------- functional builtins

	b["map"] = &Builtin{Name: "map", Fn: func(args []Object) (Object, error) {
		if err := arity("map", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("map", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("map() has no function caller")
		}
		res := make([]Object, len(arr.Elements))
		for idx, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{el})
			if err != nil {
				return nil, err
			}
			res[idx] = v
		}
		return &Array{Elements: res}, nil
	}}

	b["filter"] = &Builtin{Name: "filter", Fn: func(args []Object) (Object, error) {
		if err := arity("filter", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("filter", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("filter() has no function caller")
		}
		res := make([]Object, 0)
		for _, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{el})
			if err != nil {
				return nil, err
			}
			b, ok := v.(*Boolean)
			if !ok {
				return nil, fmt.Errorf("filter() predicate must return bool, got %s", v.Type())
			}
			if b.Value {
				res = append(res, el)
			}
		}
		return &Array{Elements: res}, nil
	}}

	b["reduce"] = &Builtin{Name: "reduce", Fn: func(args []Object) (Object, error) {
		if err := arity("reduce", args, 3); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("reduce", args[0], "array")
		}
		fn := args[1]
		acc := args[2]
		if caller == nil {
			return nil, errors.New("reduce() has no function caller")
		}
		for _, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{acc, el})
			if err != nil {
				return nil, err
			}
			acc = v
		}
		return acc, nil
	}}

	b["any"] = &Builtin{Name: "any", Fn: func(args []Object) (Object, error) {
		if err := arity("any", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("any", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("any() has no function caller")
		}
		for _, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{el})
			if err != nil {
				return nil, err
			}
			b, ok := v.(*Boolean)
			if !ok {
				return nil, fmt.Errorf("any() predicate must return bool, got %s", v.Type())
			}
			if b.Value {
				return trueObject, nil
			}
		}
		return falseObject, nil
	}}

	b["all"] = &Builtin{Name: "all", Fn: func(args []Object) (Object, error) {
		if err := arity("all", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("all", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("all() has no function caller")
		}
		for _, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{el})
			if err != nil {
				return nil, err
			}
			b, ok := v.(*Boolean)
			if !ok {
				return nil, fmt.Errorf("all() predicate must return bool, got %s", v.Type())
			}
			if !b.Value {
				return falseObject, nil
			}
		}
		return trueObject, nil
	}}

	b["sort_by"] = &Builtin{Name: "sort_by", Fn: func(args []Object) (Object, error) {
		if err := arity("sort_by", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("sort_by", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("sort_by() has no function caller")
		}
		elems := make([]Object, len(arr.Elements))
		copy(elems, arr.Elements)
		var sortErr error
		sort.SliceStable(elems, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			v, err := caller.Call(fn, []Object{elems[i], elems[j]})
			if err != nil {
				sortErr = err
				return false
			}
			num, ok := v.(*Integer)
			if !ok {
				sortErr = fmt.Errorf("sort_by() comparator must return int, got %s", v.Type())
				return false
			}
			return num.Value < 0
		})
		if sortErr != nil {
			return nil, sortErr
		}
		return &Array{Elements: elems}, nil
	}}

	b["find_index"] = &Builtin{Name: "find_index", Fn: func(args []Object) (Object, error) {
		if err := arity("find_index", args, 2); err != nil {
			return nil, err
		}
		arr, ok := args[0].(*Array)
		if !ok {
			return nil, typeError("find_index", args[0], "array")
		}
		fn := args[1]
		if caller == nil {
			return nil, errors.New("find_index() has no function caller")
		}
		for idx, el := range arr.Elements {
			v, err := caller.Call(fn, []Object{el})
			if err != nil {
				return nil, err
			}
			b, ok := v.(*Boolean)
			if !ok {
				return nil, fmt.Errorf("find_index() predicate must return bool, got %s", v.Type())
			}
			if b.Value {
				return newInteger(int64(idx)), nil
			}
		}
		return newInteger(-1), nil
	}}

	return b
}

// writeFileLike shares the argument handling between write_file and
// append_file, which differ only in how the text is put on disk.
func writeFileLike(name string, args []Object, write func(path, text string) error) (Object, error) {
	if err := arity(name, args, 2); err != nil {
		return nil, err
	}
	path, err := strArg(name, args, 0)
	if err != nil {
		return nil, err
	}
	text, err := strArg(name, args, 1)
	if err != nil {
		return nil, err
	}
	if err := write(path, text); err != nil {
		return nil, fmt.Errorf("%s() could not write %s: %s", name, path, fileReason(err))
	}
	return nilObject, nil
}

// fileReason drops the file name Go puts in front of its own message, because
// the caller has already named the file.
func fileReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
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

// intArg reads the argument at position at as an int.
func intArg(fn string, args []Object, at int) (int64, error) {
	return asInt(fn, args[at])
}

// strArg reads the argument at position at as a string.
func strArg(fn string, args []Object, at int) (string, error) {
	s, ok := args[at].(*String)
	if !ok {
		return "", typeError(fn, args[at], "str")
	}
	return s.Value, nil
}

// numberValue reads an int or a float as a float, so both can be compared and
// used in floating point maths without the caller caring which it was.
func numberValue(o Object) (float64, bool) {
	switch v := o.(type) {
	case *Integer:
		return float64(v.Value), true
	case *Float:
		return v.Value, true
	}
	return 0, false
}

// isWhole reports whether a number has no fractional part.
func isWhole(o Object) bool {
	if _, ok := o.(*Integer); ok {
		return true
	}
	if f, ok := o.(*Float); ok {
		return f.Value == math.Trunc(f.Value)
	}
	return false
}

// sortedCopy returns a sorted copy of an array's elements. The caller's array is
// left alone, matching push(). Every element has to be a number or every
// element has to be a string; a mixture is refused rather than guessed at.
func sortedCopy(elements []Object) ([]Object, error) {
	out := make([]Object, len(elements))
	copy(out, elements)

	kind := ""
	for _, el := range out {
		switch el.(type) {
		case *Integer, *Float:
			if kind == "" {
				kind = "number"
			} else if kind != "number" {
				return nil, fmt.Errorf("sort() needs a single kind of value, but this array mixes %s and numbers", kind)
			}
		case *String:
			if kind == "" {
				kind = "str"
			} else if kind != "str" {
				return nil, fmt.Errorf("sort() needs a single kind of value, but this array mixes %s and strings", kind)
			}
		default:
			return nil, fmt.Errorf("sort() cannot put %s values in order", el.Type())
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if kind == "str" {
			return out[i].(*String).Value < out[j].(*String).Value
		}
		a, _ := numberValue(out[i])
		b, _ := numberValue(out[j])
		return a < b
	})
	return out, nil
}

// sliceBounds checks a pair of slice ends and returns them as ints.
func sliceBounds(start, end, length int64) (int, int, error) {
	if start < 0 || start > length {
		return 0, 0, fmt.Errorf("slice() start %d is out of range (length %d)", start, length)
	}
	if end < start || end > length {
		return 0, 0, fmt.Errorf("slice() end %d is out of range (start %d, length %d)", end, start, length)
	}
	return int(start), int(end), nil
}

func minMax(name string, args []Object, wantMin bool) (Object, error) {
	// A single array means "the smallest of these", which saves writing
	// sort(a)[0] or a loop.
	if len(args) == 1 {
		if arr, ok := args[0].(*Array); ok {
			args = arr.Elements
		}
	}
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
