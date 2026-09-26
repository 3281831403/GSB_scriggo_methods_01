// Copyright 2019 The Scriggo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"reflect"
	"testing"
)

// Types used to test the promotion of methods in defined types.

type promotedInner struct{}

func (promotedInner) M() int        { return 1 }
func (promotedInner) Error() string { return "inner" }

type promotedMser interface{ M() int }
type promotedMser2 interface{ M() string }

// checkerPromotedMethods maps a source to whether it type-checks. It tests
// that the method set of a defined type includes the methods promoted from
// the embedded fields of its underlying struct type.
var checkerPromotedMethods = []struct {
	src string
	ok  bool
}{

	// Embedded interface.
	{`type T struct { error }; var t T; _ = t.Error`, true},
	{`type T struct { error }; var t T; _ = t.Error()`, true},
	{`type T struct { error }; _ = T.Error`, true},
	{`type T struct { error }; var _ error = T{}`, true},

	// Embedded struct.
	{`type T struct { Inner }; var t T; _ = t.M()`, true},
	{`type T struct { Inner }; var t T; _ = t.Error()`, true},
	{`type T struct { Inner }; _ = T.M`, true},
	{`type T struct { Inner }; var _ Mser = T{}`, true},
	{`type T struct { Inner }; var _ Mser = &T{}`, true},

	// Embedded pointer type.
	{`type T struct { *Inner }; var t T; _ = t.M()`, true},
	{`type T struct { error }; type U struct { *T }; var u U; _ = u.Error`, true},
	{`type T struct { Inner }; type U struct { *T }; var u U; _ = u.M()`, true},
	{`type T struct { error }; var t *T; _ = t.Error`, true},
	{`type T struct { Inner }; var t *T; _ = t.M()`, true},

	// Multi-level promotion.
	{`type T struct { error }; type U struct { T }; var u U; _ = u.Error`, true},
	{`type T struct { error }; type U struct { T }; type V struct { U }; var v V; _ = v.Error()`, true},
	{`type T struct { Inner }; type U struct { T }; var u U; _ = u.M()`, true},

	// Non-conflicting method names.
	{`type T struct { error; Mser }; var t T; _ = t.Error(); _ = t.M()`, true},

	// Conflicting method names at the same depth are ambiguous.
	{`type T struct { Mser; Mser2 }; var t T; _ = t.M()`, false},

	// Conflicting method names at different depths: the shallower wins.
	{`type T struct { Mser2 }; type U struct { T; Mser }; var u U; _ = u.M()`, true},
	{`type T struct { Mser2 }; type U struct { T; Mser }; var u U; var _ Mser = u`, true},
	{`type T struct { Mser2 }; type U struct { T; Mser }; var u U; var _ Mser2 = u`, false},

	// A field shadows a promoted method with the same name.
	{`type T struct { error; Error string }; var t T; _ = t.Error`, true},

	// The embedded field itself is selectable.
	{`type T struct { Inner }; var t T; _ = t.Inner`, true},

	// Types without embedded fields are unchanged.
	{`type T int; var t T; _ = t.M()`, false},
	{`type T struct { F int }; var t T; _ = t.M()`, false},
	{`type T struct { F int }; var t T; _ = t.F`, true},
	{`type T int; var _ interface{} = T(0)`, true},
}

func TestCheckerPromotedMethods(t *testing.T) {
	names := map[string]*typeInfo{
		"Inner": {Properties: propertyIsType, Type: reflect.TypeFor[promotedInner]()},
		"Mser":  {Properties: propertyIsType, Type: reflect.TypeFor[promotedMser]()},
		"Mser2": {Properties: propertyIsType, Type: reflect.TypeFor[promotedMser2]()},
	}
	for _, c := range checkerPromotedMethods {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(*CheckingError); ok {
						if c.ok {
							t.Errorf("source: '%s' should be 'ok' but got error: %q", c.src, err)
						}
					} else {
						panic(fmt.Errorf("source %q: %s", c.src, r))
					}
				} else if !c.ok {
					t.Errorf("source: '%s' expecting an error, but no errors have been returned by type-checker", c.src)
				}
			}()
			tree, err := parseSource([]byte(c.src), true)
			if err != nil {
				t.Errorf("source: %s returned parser error: %s", c.src, err.Error())
				return
			}
			compilation := newCompilation(nil)
			tc := newTypechecker(compilation, "", checkerOptions{allowGoStmt: true, mod: programMod}, nil)
			for name, ti := range names {
				tc.scopes.Declare(name, ti, nil, nil)
			}
			tc.scopes.Enter(tree)
			tree.Nodes = tc.checkNodes(tree.Nodes)
			tc.scopes.Exit()
			err = compilation.finalizeUsingStatements(tc)
			if err != nil {
				t.Fatal(err)
			}
		}()
	}
}
