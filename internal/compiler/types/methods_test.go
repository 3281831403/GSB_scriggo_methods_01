// Copyright 2019 The Scriggo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"reflect"
	"testing"
)

type methodsInner struct{}

func (methodsInner) M() int { return 1 }

type methodsMser interface{ M() int }
type methodsMser2 interface{ M() string }

// structOf is a helper that creates a struct type with the given embedded
// fields.
func structOf(types *Types, fields ...reflect.Type) reflect.Type {
	sf := make([]reflect.StructField, len(fields))
	for i, f := range fields {
		sf[i] = reflect.StructField{Name: "F" + string(rune('A'+i)), Type: f, Anonymous: true}
	}
	return types.StructOf(sf)
}

func TestPromotedMethodByName(t *testing.T) {
	types := NewTypes()
	inner := reflect.TypeFor[methodsInner]()
	mser := reflect.TypeFor[methodsMser]()
	mser2 := reflect.TypeFor[methodsMser2]()
	errType := reflect.TypeFor[error]()

	// type T struct { error }
	tT := types.DefinedOf("T", structOf(types, errType))
	if _, ok := tT.MethodByName("Error"); !ok {
		t.Error("T should have method Error")
	}
	if _, ok := tT.MethodByName("M"); ok {
		t.Error("T should not have method M")
	}

	// type U struct { T }: multi-level promotion.
	tU := types.DefinedOf("U", structOf(types, tT))
	if _, ok := tU.MethodByName("Error"); !ok {
		t.Error("U should have promoted method Error")
	}

	// type V struct { *T }: promotion through an embedded pointer.
	tV := types.DefinedOf("V", structOf(types, types.PointerTo(tT)))
	if _, ok := tV.MethodByName("Error"); !ok {
		t.Error("V should have promoted method Error")
	}

	// type W struct { Inner }: promotion of a gc compiled type's methods.
	tW := types.DefinedOf("W", structOf(types, inner))
	m, ok := tW.MethodByName("M")
	if !ok {
		t.Error("W should have promoted method M")
	} else if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.Int {
		t.Errorf("unexpected signature of W.M: %s", m.Type)
	}

	// *W has the same method set as W.
	if _, ok := types.PointerTo(tW).MethodByName("M"); !ok {
		t.Error("*W should have promoted method M")
	}

	// type A struct { Mser; Mser2 }: ambiguous method M.
	tA := types.DefinedOf("A", structOf(types, mser, mser2))
	if _, ok := tA.MethodByName("M"); ok {
		t.Error("A should not have method M: it is ambiguous")
	}

	// type B struct { Mser2 }, type C struct { B; Mser }: the method M of
	// C is Mser.M because it is at a shallower depth than B's M.
	tB := types.DefinedOf("B", structOf(types, mser2))
	tC := types.DefinedOf("C", structOf(types, tB, mser))
	m, ok = tC.MethodByName("M")
	if !ok {
		t.Error("C should have promoted method M")
	} else if m.Type.Out(0).Kind() != reflect.Int {
		t.Errorf("C.M should have signature func() int, got %s", m.Type)
	}

	// A defined type with a non-struct underlying type has the method set
	// of the underlying type.
	tD := types.DefinedOf("D", errType)
	if _, ok := tD.MethodByName("Error"); !ok {
		t.Error("D should have the method Error of its underlying interface type")
	}

	// A defined type without embedded fields has no methods.
	tE := types.DefinedOf("E", reflect.TypeFor[int]())
	if _, ok := tE.MethodByName("M"); ok {
		t.Error("E should not have methods")
	}
}

func TestImplementsWithPromotedMethods(t *testing.T) {
	types := NewTypes()
	mser := reflect.TypeFor[methodsMser]()
	mser2 := reflect.TypeFor[methodsMser2]()
	errType := reflect.TypeFor[error]()

	// type T struct { error } implements error.
	tT := types.DefinedOf("T", structOf(types, errType))
	if !Implements(tT, errType) {
		t.Error("T should implement error")
	}

	// type U struct { T } implements error.
	tU := types.DefinedOf("U", structOf(types, tT))
	if !Implements(tU, errType) {
		t.Error("U should implement error")
	}

	// type W struct { Inner } implements Mser, and *W implements Mser.
	tW := types.DefinedOf("W", structOf(types, reflect.TypeFor[methodsInner]()))
	if !Implements(tW, mser) {
		t.Error("W should implement Mser")
	}
	if !Implements(types.PointerTo(tW), mser) {
		t.Error("*W should implement Mser")
	}
	if Implements(tW, mser2) {
		t.Error("W should not implement Mser2")
	}

	// A defined type without promoted methods implements only empty
	// interfaces, as before.
	tE := types.DefinedOf("E", reflect.TypeFor[int]())
	if Implements(tE, mser) {
		t.Error("E should not implement Mser")
	}
	if !Implements(tE, reflect.TypeFor[interface{}]()) {
		t.Error("E should implement interface{}")
	}
}
