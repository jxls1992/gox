/*
 Copyright 2021 The GoPlus Authors (goplus.org)
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gox_test

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"testing"

	"github.com/goplus/gox"
)

var (
	gopNamePrefix = "Gop_"
)

func toIndex(c byte) int {
	if c >= 'a' && c <= 'z' {
		return int(c - ('a' - 10))
	}
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	panic("TODO: invalid character out of [0-9,a-z]")
}

func initGopBuiltin(pkg gox.PkgImporter, builtin *types.Package) {
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	big.Ref("Gop_bigint") // to ensure the package is loaded
	scope := big.Types.Scope()
	overloads := make(map[string][]types.Object)
	for i, n := 0, scope.Len(); i < n; i++ {
		names := scope.Names()
		for _, name := range names {
			o := scope.Lookup(name)
			builtin.Scope().Insert(o)
			if n := len(name); n > 3 && name[n-3:n-1] == "__" { // overload function
				key := name[:n-3]
				overloads[key] = append(overloads[key], o)
			}
		}
	}
	for key, items := range overloads {
		off := len(key) + 2
		fns := make([]types.Object, len(items))
		for _, item := range items {
			idx := toIndex(item.Name()[off])
			if idx >= len(items) {
				panic("TODO: overload function must be from 0 to N")
			}
			fns[idx] = item
		}
		builtin.Scope().Insert(gox.NewOverloadFunc(token.NoPos, builtin, key, fns...))
	}
}

func newGopBuiltinDefault(pkg gox.PkgImporter, prefix string, contracts *gox.BuiltinContracts) *types.Package {
	builtin := types.NewPackage("", "")
	initGopBuiltin(pkg, builtin)
	gox.InitBuiltinOps(builtin, prefix, contracts)
	gox.InitBuiltinFuncs(builtin)
	return builtin
}

func newGopMainPackage() *gox.Package {
	conf := &gox.Config{
		Fset:       gblFset,
		LoadPkgs:   gblLoadPkgs,
		NewBuiltin: newGopBuiltinDefault,
		Prefix:     gopNamePrefix,
	}
	return gox.NewPackage("", "main", conf)
}

type untypedRatType struct {
	bigint bool
}

func (p *untypedRatType) Underlying() types.Type {
	panic("untypedRatType")
}

func (p *untypedRatType) String() string {
	panic("untypedRatType")
}

type gopTypeExtend struct {
	bigint types.Type
	bigrat types.Type
}

func newGopTypeSys(pkg *gox.Package) *gopTypeExtend {
	bigint := pkg.Builtin().Ref("Gop_bigint").Type()
	bigrat := pkg.Builtin().Ref("Gop_bigrat").Type()
	return &gopTypeExtend{bigint: bigint, bigrat: bigrat}
}

func (p *gopTypeExtend) Default(typ types.Type) types.Type {
	if t, ok := typ.(*untypedRatType); ok {
		if t.bigint {
			return p.bigint
		}
		return p.bigrat
	}
	return types.Default(typ)
}

func untypedRat(pkg *gox.Package, v int) *gox.Element {
	return &gox.Element{
		Val:  nil, // TODO: generate expr AST
		Type: &untypedRatType{true},
		CVal: constant.Make(big.NewRat(int64(v), 1)),
	}
}

func ident(name string) *ast.Ident {
	return &ast.Ident{Name: name}
}

// ----------------------------------------------------------------------------

func TestBigRatConstant(t *testing.T) {
	a := constant.Make(new(big.Rat).SetInt64(1))
	b := constant.Make(new(big.Rat).SetInt64(2))
	c := constant.BinaryOp(a, token.ADD, b)
	if c.Kind() != constant.Float {
		t.Fatal("c.Kind() != constant.Float -", c)
	}
	d := constant.BinaryOp(a, token.QUO, b)
	if !constant.Compare(constant.Make(big.NewRat(1, 2)), token.EQL, d) {
		t.Fatal("d != 1/2r")
	}

	e := constant.MakeFromLiteral("1.2", token.FLOAT, 0)
	if _, ok := constant.Val(e).(*big.Rat); !ok {
		t.Fatal("constant.MakeFromLiteral 1.2 not *big.Rat", constant.Val(e))
	}
}

func TestBigIntVar(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.NewVar(big.Ref("Gop_bigint").Type(), "a")
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a builtin.Gop_bigint
`)
}

func TestBigInt(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.NewVar(big.Ref("Gop_bigint").Type(), "a", "b")
	pkg.NewVarStart(big.Ref("Gop_bigint").Type(), "c").
		Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.ADD).EndInit(1)
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a, b builtin.Gop_bigint
var c builtin.Gop_bigint = a.Gop_Add(b)
`)
}

func TestBigRat(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Builtin()
	pkg.NewVar(big.Ref("Gop_bigrat").Type(), "a", "b")
	pkg.NewVarStart(big.Ref("Gop_bigrat").Type(), "c").
		Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.QUO).EndInit(1)
	pkg.NewVarStart(big.Ref("Gop_bigrat").Type(), "d").
		Val(ctxRef(pkg, "a")).UnaryOp(token.SUB).EndInit(1)
	pkg.NewVarStart(big.Ref("Gop_bigrat").Type(), "e").
		Val(big.Ref("Gop_bigrat_Cast")).Call(0).EndInit(1)
	pkg.NewVarStart(big.Ref("Gop_bigrat").Type(), "f").
		Val(big.Ref("Gop_bigrat_Cast")).Val(1).Val(2).Call(2).EndInit(1)
	pkg.NewVarStart(big.Ref("Gop_bigint").Type(), "g")
	pkg.NewVarStart(big.Ref("Gop_bigrat").Type(), "h").
		Val(big.Ref("Gop_bigrat_Cast")).Val(ctxRef(pkg, "g")).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a, b builtin.Gop_bigrat
var c builtin.Gop_bigrat = a.Gop_Quo(b)
var d builtin.Gop_bigrat = a.Gop_Neg()
var e builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__0()
var f builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__3(1, 2)
var g builtin.Gop_bigint
var h builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__1(g)
`)
}

// ----------------------------------------------------------------------------