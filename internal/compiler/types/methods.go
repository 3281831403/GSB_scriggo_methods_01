// Copyright 2019 The Scriggo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"reflect"
)

// promotedMethodByName returns the method with the given name in the method
// set of a Scriggo type whose underlying type is the struct type s. goType
// is the gc compiled type of s.
//
// As Scriggo does not currently support method declarations on defined
// types, the method set consists of the methods promoted from the embedded
// fields of s. The promoted method is determined following the Go
// specification: a method is promoted if no field or method with the same
// name exists at a shallower depth and it is not ambiguous, that is, it is
// not promoted from two or more embedded fields at the same depth.
//
// The returned method is the method of goType whose signature, receiver
// aside, is the same as the promoted method, so its Func value is the
// wrapper method created by the reflect package that calls the promoted
// method through the embedded fields.
func promotedMethodByName(s, goType reflect.Type, name string) (reflect.Method, bool) {
	sig, ok := promotedMethodSignature(s, name)
	if !ok {
		return reflect.Method{}, false
	}
	n := goType.NumMethod()
	for i := 0; i < n; i++ {
		m := goType.Method(i)
		if m.Name != name {
			continue
		}
		if sameMethodSignature(m.Type, sig) {
			return m, true
		}
	}
	return reflect.Method{}, false
}

// promotedMethodSignature returns the signature, without the receiver, of
// the method with the given name promoted from the embedded fields of the
// struct type s. It returns false if the method is not promoted, is
// shadowed by a field at a shallower depth or is ambiguous.
func promotedMethodSignature(s reflect.Type, name string) (reflect.Type, bool) {
	current := []reflect.Type{s}
	for len(current) > 0 {
		var next []reflect.Type
		var sig reflect.Type
		count := 0
		for _, t := range current {
			n := t.NumField()
			for i := 0; i < n; i++ {
				f := t.Field(i)
				if f.Name == name {
					// A field at this depth shadows every method
					// promoted from a deeper level.
					return nil, false
				}
				if !f.Anonymous {
					continue
				}
				ft := f.Type
				if ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				switch ft.(type) {
				case definedType, structType:
					if ft.Kind() == reflect.Struct {
						// The methods promoted from a Scriggo struct
						// type are computed recursively at the next
						// depth. Note that the pointer and value method
						// sets of a Scriggo type are the same, because
						// Scriggo does not currently support method
						// declarations.
						next = append(next, ft)
						continue
					}
				}
				// For embedded interfaces, gc compiled types and Scriggo
				// defined types with a non-struct underlying type, the
				// method set is retrieved through the reflect package.
				if m, ok := f.Type.MethodByName(name); ok {
					sig = methodSignature(f.Type, m)
					count++
				}
			}
		}
		if count == 1 {
			return sig, true
		}
		if count > 1 {
			// Ambiguous method.
			return nil, false
		}
		current = next
	}
	return nil, false
}

// methodSignature returns the signature of the method m of the type t,
// without the receiver.
func methodSignature(t reflect.Type, m reflect.Method) reflect.Type {
	mt := m.Type
	if t.Kind() == reflect.Interface {
		// Methods of interface types do not have the receiver.
		return mt
	}
	in := make([]reflect.Type, mt.NumIn()-1)
	for i := range in {
		in[i] = mt.In(i + 1)
	}
	out := make([]reflect.Type, mt.NumOut())
	for i := range out {
		out[i] = mt.Out(i)
	}
	return reflect.FuncOf(in, out, mt.IsVariadic())
}

// sameMethodSignature reports whether the method type mt, that has the
// receiver as first parameter, has the same signature as sig, that has no
// receiver.
func sameMethodSignature(mt, sig reflect.Type) bool {
	if mt.NumIn()-1 != sig.NumIn() || mt.NumOut() != sig.NumOut() || mt.IsVariadic() != sig.IsVariadic() {
		return false
	}
	for i := 0; i < sig.NumIn(); i++ {
		if mt.In(i+1) != sig.In(i) {
			return false
		}
	}
	for i := 0; i < sig.NumOut(); i++ {
		if mt.Out(i) != sig.Out(i) {
			return false
		}
	}
	return true
}
