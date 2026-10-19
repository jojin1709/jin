package main

import "fmt"

// ---------------------------------------------------------------- record types

// StructType is a record type that a program declared, such as Point. The name
// the type was declared under holds one of these at run time, so calling that
// name — Point(1, 2) — is how an instance is made. A type is a value, which is
// what lets it be passed around, stored in variables and reached through an
// import alias as u.Point.
type StructType struct {
	Name   string
	Fields []string // the field names, in the order they were declared
}

// Type reports the type's own name, so a message about a Point reads the same
// whether it is about the type or about one of its instances.
func (o *StructType) Type() ObjectType { return ObjectType(o.Name) }

func (o *StructType) Inspect() string { return "struct " + o.Name }

// index says where a field sits, and whether the type has it at all. Records
// hold their fields in declaration order, so a lookup is a short scan; a
// compiled type has only a handful of fields.
func (o *StructType) index(name string) (int, bool) {
	for i, f := range o.Fields {
		if f == name {
			return i, true
		}
	}
	return 0, false
}

// Struct is one instance of a record type: the value of each field, in the
// order the type declared them. A record is a reference value, like a map or an
// array, so writing a field inside a function is seen by the caller.
type Struct struct {
	Def    *StructType // the type this is an instance of
	Fields []Object
}

// Type is the record type's name, so type(p) answers "Point" and an error
// message names the record rather than the general idea of one.
func (o *Struct) Type() ObjectType { return ObjectType(o.Def.Name) }

func (o *Struct) Inspect() string {
	parts := make([]string, 0, len(o.Fields))
	for i, name := range o.Def.Fields {
		parts = append(parts, name+": "+repr(o.Fields[i]))
	}
	return o.Def.Name + "{" + joinWith(parts, ", ") + "}"
}

// newStruct builds an instance, checking that the number of values matches the
// number of fields. Both back ends call it, so the message is the same either
// way, and neither can build a record with a field left empty.
func newStruct(t *StructType, values []Object) (Object, error) {
	if len(values) != len(t.Fields) {
		return nil, fmt.Errorf("%s(...) expects %d value(s), got %d", t.Name, len(t.Fields), len(values))
	}
	fields := make([]Object, len(values))
	copy(fields, values)
	return &Struct{Def: t, Fields: fields}, nil
}

// ---------------------------------------------------------------- field access

// fieldName reads the field name an OpGetField or OpSetField carries. It is
// always the constant the compiler put there, so a missing one would be a
// compiler bug rather than something a program can reach.
func fieldName(obj Object) string {
	if s, ok := obj.(*String); ok {
		return s.Value
	}
	return ""
}

// getField reads one field of one record. The message for a name the record
// does not have is built here and nowhere else, so the two back ends cannot
// report it differently.
func getField(obj Object, name string) (Object, error) {
	if s, ok := obj.(*Struct); ok {
		if i, found := s.Def.index(name); found {
			return s.Fields[i], nil
		}
		return nil, fmt.Errorf("%s has no field %q", s.Def.Name, name)
	}
	return nil, noFieldError(obj, name)
}

// setField writes one field. Records are reference values, so this is seen
// wherever the record is.
func setField(obj Object, name string, value Object) error {
	if s, ok := obj.(*Struct); ok {
		if i, found := s.Def.index(name); found {
			s.Fields[i] = value
			return nil
		}
		return fmt.Errorf("%s has no field %q", s.Def.Name, name)
	}
	if t, ok := obj.(*StructType); ok {
		return cannotWriteFieldOfType(t.Name)
	}
	return fmt.Errorf("cannot assign a field of %s", obj.Type())
}

// noFieldError explains why a dotted read cannot work on something that is not
// a record. A map is the common case, so it is told what to write instead.
func noFieldError(obj Object, name string) error {
	if t, ok := obj.(*StructType); ok {
		return cannotReadFieldOfType(t.Name)
	}
	if _, ok := obj.(*Map); ok {
		return fmt.Errorf("cannot read a field of map; use m[%q] to read a key", name)
	}
	return fmt.Errorf("cannot read a field of %s", obj.Type())
}

// cannotReadFieldOfType is the message for Point.x, where Point is the type
// itself and not an instance of it. The checker builds the same text.
func cannotReadFieldOfType(name string) error {
	return fmt.Errorf("cannot read a field of the type %s; make a %s first", name, name)
}

// cannotWriteFieldOfType is the same idea for Point.x = 1.
func cannotWriteFieldOfType(name string) error {
	return fmt.Errorf("cannot assign a field of the type %s; make a %s first", name, name)
}
