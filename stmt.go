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

package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gox/internal"
)

type controlFlow interface {
	Then(cb *CodeBuilder)
}

// ----------------------------------------------------------------------------
//
// if init; cond then
//   ...
// else
//   ...
// end
//
type ifStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
}

func (p *ifStmt) Then(cb *CodeBuilder) {
	cond := cb.stk.Pop()
	if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
		panic("TODO: if statement condition is not a boolean expr")
	}
	p.cond = cond.Val
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: if statement has too many init statements")
	}
}

func (p *ifStmt) Else(cb *CodeBuilder) {
	if p.body != nil {
		panic("TODO: else statement already exists")
	}
	p.body = &ast.BlockStmt{List: cb.clearBlockStmt()}
}

func (p *ifStmt) End(cb *CodeBuilder) {
	var blockStmt = &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	var el ast.Stmt
	if p.body != nil { // if..else
		el = blockStmt
	} else { // if without else
		p.body = blockStmt
	}
	cb.emitStmt(&ast.IfStmt{Init: p.init, Cond: p.cond, Body: p.body, Else: el})
}

// ----------------------------------------------------------------------------
//
// switch init; tag then
//   expr1, expr2, ..., exprN case(N)
//     ...
//     end
//   expr1, expr2, ..., exprM case(M)
//     ...
//     end
// end
//
type switchStmt struct {
	init ast.Stmt
	tag  internal.Elem
	old  codeBlockCtx
}

func (p *switchStmt) Then(cb *CodeBuilder) {
	p.tag = cb.stk.Pop()
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: switch statement has too many init statements")
	}
}

func (p *switchStmt) Case(cb *CodeBuilder, n int) {
	var list []ast.Expr
	if n > 0 {
		list = make([]ast.Expr, n)
		for i, arg := range cb.stk.GetArgs(n) {
			if p.tag.Val != nil { // switch tag {...}
				if !ComparableTo(arg.Type, p.tag.Type) {
					log.Panicf("TODO: case expr can't compare %v to %v\n", arg.Type, p.tag.Type)
				}
			} else { // switch {...}
				if !types.AssignableTo(arg.Type, types.Typ[types.Bool]) {
					log.Panicln("TODO: case expr is not a boolean expr")
				}
			}
			list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}
	stmt := &caseStmt{at: p, list: list}
	cb.startBlockStmt(stmt, "case statement", &stmt.old)
}

func (p *switchStmt) End(cb *CodeBuilder) {
	body := &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: p.tag.Val, Body: body})
}

type caseStmt struct {
	at   *switchStmt
	list []ast.Expr
	old  codeBlockCtx
}

func (p *caseStmt) Fallthrough(cb *CodeBuilder) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.FALLTHROUGH})
}

func (p *caseStmt) End(cb *CodeBuilder) {
	body := cb.endBlockStmt(p.old)
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

// ----------------------------------------------------------------------------
//
// for init; cond then
//   body
//   post
// end
//
type forStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
}

func (p *forStmt) Then(cb *CodeBuilder) {
	cond := cb.stk.Pop()
	if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
		panic("TODO: for statement condition is not a boolean expr")
	}
	p.cond = cond.Val
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: for condition has too many init statements")
	}
}

func (p *forStmt) Post(cb *CodeBuilder) {
	p.body = &ast.BlockStmt{List: cb.clearBlockStmt()}
}

func (p *forStmt) End(cb *CodeBuilder) {
	var stmts = cb.endBlockStmt(p.old)
	var post ast.Stmt
	if p.body != nil { // has post stmt
		if len(stmts) != 1 {
			panic("TODO: too many post statements")
		}
		post = stmts[0]
	} else { // no post
		p.body = &ast.BlockStmt{List: stmts}
	}
	cb.emitStmt(&ast.ForStmt{Init: p.init, Cond: p.cond, Post: post, Body: p.body})
}

// ----------------------------------------------------------------------------

type forRangeStmt struct {
	names []string
	stmt  *ast.RangeStmt
	old   codeBlockCtx
}

func (p *forRangeStmt) RangeAssignThen(cb *CodeBuilder) {
	if names := p.names; names != nil { // for k, v := range XXX {
		var val ast.Expr
		switch len(names) {
		case 1:
		case 2:
			val = ident(names[1])
		default:
			panic("TODO: invalid syntax of for range :=")
		}
		x := cb.stk.Pop()
		pkg, scope := cb.pkg, cb.current.scope
		typs := getKeyValTypes(x.Type)
		for i, name := range names {
			if name == "_" {
				continue
			}
			if scope.Insert(types.NewVar(token.NoPos, pkg.Types, name, typs[i])) != nil {
				log.Panicln("TODO: variable already defined -", name)
			}
		}
		p.stmt = &ast.RangeStmt{
			Key:   ident(names[0]),
			Value: val,
			Tok:   token.DEFINE,
			X:     x.Val,
		}
	} else { // for k, v = range XXX {
		var key, val, x internal.Elem
		n := cb.stk.Len() - cb.current.base
		args := cb.stk.GetArgs(n)
		switch n {
		case 1:
			x = args[0]
		case 2:
			key, x = args[0], args[1]
		case 3:
			key, val, x = args[0], args[1], args[2]
		default:
			panic("TODO: invalid syntax of for range =")
		}
		cb.stk.PopN(n)
		p.stmt = &ast.RangeStmt{
			Key:   key.Val,
			Value: val.Val,
			X:     x.Val,
		}
		if n > 1 {
			p.stmt.Tok = token.ASSIGN
			typs := getKeyValTypes(x.Type)
			assignMatchType(cb.pkg, key.Type, typs[0])
			if val.Val != nil {
				assignMatchType(cb.pkg, val.Type, typs[1])
			}
		}
	}
}

func getKeyValTypes(typ types.Type) []types.Type {
	typs := make([]types.Type, 2)
	switch t := typ.(type) {
	case *types.Map:
		typs[0], typs[1] = t.Key(), t.Elem()
	case *types.Slice:
		typs[0], typs[1] = types.Typ[types.Int], t.Elem()
	case *types.Array:
		typs[0], typs[1] = types.Typ[types.Int], t.Elem()
	case *types.Chan:
		typs[0] = t.Elem()
	default:
		log.Panicln("TODO: can't for range to type", t)
	}
	return typs
}

func (p *forRangeStmt) End(cb *CodeBuilder) {
	p.stmt.Body = &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(p.stmt)
}

// ----------------------------------------------------------------------------