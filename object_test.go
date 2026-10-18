package main

import "testing"

func newTestMap(t *testing.T, pairs ...Object) *Map {
	t.Helper()
	m := &Map{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if err := m.Set(pairs[i], pairs[i+1]); err != nil {
			t.Fatalf("Set(%s): %v", pairs[i].Inspect(), err)
		}
	}
	return m
}

func TestMapKeepsInsertionOrder(t *testing.T) {
	m := newTestMap(t,
		&String{Value: "z"}, newInteger(1),
		&String{Value: "a"}, newInteger(2),
		&String{Value: "m"}, newInteger(3),
	)

	if m.Inspect() != `{"z": 1, "a": 2, "m": 3}` {
		t.Errorf("Inspect() is %s", m.Inspect())
	}
	if got := len(m.Keys()); got != 3 {
		t.Fatalf("want 3 keys, got %d", got)
	}
	if m.Keys()[0].(*String).Value != "z" {
		t.Errorf("first key is %s, want \"z\"", m.Keys()[0].Inspect())
	}
}

func TestMapSetReplacesInPlace(t *testing.T) {
	m := newTestMap(t, &String{Value: "a"}, newInteger(1))
	if err := m.Set(&String{Value: "a"}, newInteger(9)); err != nil {
		t.Fatal(err)
	}

	// Replacing a value must not add a second entry or move the first one.
	if m.Len() != 1 {
		t.Errorf("want 1 entry, got %d", m.Len())
	}
	if m.Inspect() != `{"a": 9}` {
		t.Errorf("Inspect() is %s", m.Inspect())
	}
}

func TestMapLookupAfterDeletingFromTheMiddle(t *testing.T) {
	m := newTestMap(t,
		&String{Value: "a"}, newInteger(1),
		&String{Value: "b"}, newInteger(2),
		&String{Value: "c"}, newInteger(3),
		&String{Value: "d"}, newInteger(4),
	)

	found, err := m.Delete(&String{Value: "b"})
	if err != nil || !found {
		t.Fatalf("Delete(b) = %v, %v", found, err)
	}

	// The entries after the hole moved down, so their slots have to be right.
	for _, key := range []string{"c", "d"} {
		value, ok, err := m.Get(&String{Value: key})
		if err != nil {
			t.Fatalf("Get(%s): %v", key, err)
		}
		if !ok {
			t.Fatalf("%s went missing after the delete", key)
		}
		if value.(*Integer).Value == 0 {
			t.Errorf("%s has the wrong value", key)
		}
	}

	// A key that was deleted is gone, and deleting it twice reports false.
	_, ok, err := m.Get(&String{Value: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("b should be gone")
	}
	again, err := m.Delete(&String{Value: "b"})
	if err != nil || again {
		t.Errorf("second Delete(b) = %v, %v, want false", again, err)
	}
	if m.Inspect() != `{"a": 1, "c": 3, "d": 4}` {
		t.Errorf("Inspect() is %s", m.Inspect())
	}
}

func TestMapRejectsKeysItCannotHold(t *testing.T) {
	m := &Map{}
	err := m.Set(&Array{Elements: []Object{newInteger(1)}}, newInteger(2))
	if err == nil {
		t.Fatal("an array key should be refused")
	}
	if _, err := m.Delete(&Nil{}); err == nil {
		t.Fatal("a nil key should be refused")
	}
}

func TestMapKeysAreDistinguishable(t *testing.T) {
	// 1 as an int, "1" as a string and true all encode differently.
	m := newTestMap(t,
		newInteger(1), &String{Value: "int"},
		&String{Value: "1"}, &String{Value: "str"},
		newBoolean(true), &String{Value: "bool"},
	)
	if m.Len() != 3 {
		t.Fatalf("want 3 entries, got %d", m.Len())
	}

	for key, want := range map[string]Object{
		"int":  newInteger(1),
		"str":  &String{Value: "1"},
		"bool": newBoolean(true),
	} {
		value, ok, err := m.Get(want)
		if err != nil || !ok {
			t.Fatalf("Get(%s) = ok %v, err %v", key, ok, err)
		}
		if value.(*String).Value != key {
			t.Errorf("key %s found the value %s", key, value.Inspect())
		}
	}
}
