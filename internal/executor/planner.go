package executor

import (
	"fmt"
	"strings"

	csvpkg "csvsql/internal/csv"
	"csvsql/internal/parser"
	"csvsql/internal/types"
)

func (e *Executor) planQuery(stmt *parser.SelectStmt) (*plan, error) {
	p := &plan{distinct: stmt.Distinct}

	var baseTables []planTable
	if stmt.From != nil {
		for _, tr := range stmt.From.Tables {
			t, err := e.lookupTable(tr.Name)
			if err != nil {
				return nil, err
			}
			alias := tr.Alias
			if alias == "" {
				alias = t.Name
			}
			baseTables = append(baseTables, planTable{name: t.Name, alias: alias, table: t})
		}
	}
	p.tables = baseTables

	if stmt.From != nil {
		for _, jc := range stmt.From.Joins {
			t, err := e.lookupTable(jc.Table.Name)
			if err != nil {
				return nil, err
			}
			alias := jc.Table.Alias
			if alias == "" {
				alias = t.Name
			}
			pj := planJoin{
				typ:   jc.Type,
				right: planTable{name: t.Name, alias: alias, table: t},
			}
			if len(baseTables) > 0 {
				pj.left = baseTables[0].alias
			}
			scope := scopeForTables(append(p.tables, pj.right))
			onRef, err := e.compileExpr(jc.On, scope, false)
			if err != nil {
				return nil, err
			}
			pj.on = onRef
			p.joins = append(p.joins, pj)
			p.tables = append(p.tables, pj.right)
		}
	}

	scope := scopeForTables(p.tables)

	if stmt.Where != nil {
		filter, err := e.compileExpr(stmt.Where, scope, false)
		if err != nil {
			return nil, err
		}
		p.filter = filter
	}

	if len(stmt.GroupBy) > 0 {
		for _, ge := range stmt.GroupBy {
			ref, err := e.compileExpr(ge, scope, false)
			if err != nil {
				return nil, err
			}
			p.groupBy = append(p.groupBy, ref)
		}
	}

	stmtHasAgg := false
	for _, sc := range stmt.Columns {
		if !sc.Star && containsAggregate(sc.Expr) {
			stmtHasAgg = true
			break
		}
	}
	if !stmtHasAgg && stmt.Having != nil {
		stmtHasAgg = containsAggregate(stmt.Having)
	}

	allowAgg := len(p.groupBy) > 0 || stmtHasAgg

	for _, sc := range stmt.Columns {
		if sc.Star {
			p.selects = append(p.selects, selectItem{
				star:  true,
				table: sc.Table,
			})
			continue
		}
		ref, err := e.compileExpr(sc.Expr, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		p.selects = append(p.selects, selectItem{
			expr:  ref,
			alias: sc.Alias,
		})
	}

	if stmt.Having != nil {
		ref, err := e.compileExpr(stmt.Having, scope, true)
		if err != nil {
			return nil, err
		}
		p.having = ref
	}

	for _, oi := range stmt.OrderBy {
		ref, err := e.compileExpr(oi.Expr, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		p.orderBy = append(p.orderBy, orderItemRef{expr: ref, desc: oi.Desc})
	}

	if stmt.Limit != nil {
		p.limit = stmt.Limit
	}
	if stmt.Offset != nil {
		p.offset = stmt.Offset
	}

	return p, nil
}

func scopeForTables(tables []planTable) map[string]*csvpkg.Table {
	scope := make(map[string]*csvpkg.Table)
	for _, pt := range tables {
		scope[strings.ToLower(pt.alias)] = pt.table
		scope[strings.ToLower(pt.name)] = pt.table
	}
	return scope
}

func containsAggregate(expr parser.Expr) bool {
	switch e := expr.(type) {
	case *parser.FuncCall:
		name := strings.ToUpper(e.Name)
		if name == "COUNT" || name == "SUM" || name == "AVG" || name == "MIN" || name == "MAX" {
			return true
		}
		for _, a := range e.Args {
			if containsAggregate(a) {
				return true
			}
		}
		return false
	case *parser.BinaryExpr:
		return containsAggregate(e.Left) || containsAggregate(e.Right)
	case *parser.UnaryExpr:
		return containsAggregate(e.Expr)
	case *parser.InList:
		if containsAggregate(e.Expr) {
			return true
		}
		for _, v := range e.Values {
			if containsAggregate(v) {
				return true
			}
		}
		return false
	case *parser.BetweenExpr:
		return containsAggregate(e.Expr) || containsAggregate(e.Low) || containsAggregate(e.High)
	case *parser.CaseExpr:
		if e.Expr != nil && containsAggregate(e.Expr) {
			return true
		}
		for _, w := range e.Whens {
			if containsAggregate(w.Cond) || containsAggregate(w.Then) {
				return true
			}
		}
		if e.Else != nil && containsAggregate(e.Else) {
			return true
		}
		return false
	}
	return false
}

func (e *Executor) compileExpr(expr parser.Expr, scope map[string]*csvpkg.Table, allowAgg bool) (ExprRef, error) {
	switch ex := expr.(type) {
	case *parser.Literal:
		return compileLiteral(ex)
	case *parser.ColumnRef:
		return e.compileColumnRef(ex, scope)
	case *parser.BinaryExpr:
		l, err := e.compileExpr(ex.Left, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		r, err := e.compileExpr(ex.Right, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		return &binOpRef{op: ex.Op, left: l, right: r}, nil
	case *parser.UnaryExpr:
		inner, err := e.compileExpr(ex.Expr, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		return &unaryOpRef{op: ex.Op, expr: inner}, nil
	case *parser.FuncCall:
		return e.compileFunc(ex, scope, allowAgg)
	case *parser.InList:
		ee, err := e.compileExpr(ex.Expr, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		vals := make([]ExprRef, len(ex.Values))
		for i, v := range ex.Values {
			vals[i], err = e.compileExpr(v, scope, allowAgg)
			if err != nil {
				return nil, err
			}
		}
		return &inListRef{expr: ee, not: ex.Not, values: vals}, nil
	case *parser.BetweenExpr:
		ee, err := e.compileExpr(ex.Expr, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		lo, err := e.compileExpr(ex.Low, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		hi, err := e.compileExpr(ex.High, scope, allowAgg)
		if err != nil {
			return nil, err
		}
		return &betweenRef{expr: ee, not: ex.Not, low: lo, high: hi}, nil
	case *parser.CaseExpr:
		cr := &caseRef{}
		if ex.Expr != nil {
			var err error
			cr.expr, err = e.compileExpr(ex.Expr, scope, allowAgg)
			if err != nil {
				return nil, err
			}
		}
		for _, w := range ex.Whens {
			c, err := e.compileExpr(w.Cond, scope, allowAgg)
			if err != nil {
				return nil, err
			}
			t, err := e.compileExpr(w.Then, scope, allowAgg)
			if err != nil {
				return nil, err
			}
			cr.whens = append(cr.whens, whenRef{cond: c, then: t})
		}
		if ex.Else != nil {
			var err error
			cr.else_, err = e.compileExpr(ex.Else, scope, allowAgg)
			if err != nil {
				return nil, err
			}
		}
		return cr, nil
	}
	return nil, fmt.Errorf("unsupported expression type %T", expr)
}

func compileLiteral(l *parser.Literal) (ExprRef, error) {
	switch l.Kind {
	case parser.LitNull:
		return &constRef{val: types.NullValue()}, nil
	case parser.LitInt:
		return &constRef{val: types.IntValue(l.Value.(int64))}, nil
	case parser.LitFloat:
		return &constRef{val: types.FloatValue(l.Value.(float64))}, nil
	case parser.LitString:
		return &constRef{val: types.TextValue(l.Value.(string))}, nil
	case parser.LitBool:
		return &constRef{val: types.BoolValue(l.Value.(bool))}, nil
	}
	return nil, fmt.Errorf("unknown literal kind %d", l.Kind)
}

func (e *Executor) compileColumnRef(cr *parser.ColumnRef, scope map[string]*csvpkg.Table) (ExprRef, error) {
	if cr.Star {
		return nil, fmt.Errorf("* not allowed in this context")
	}
	if cr.Table != "" {
		t, ok := scope[strings.ToLower(cr.Table)]
		if !ok {
			return nil, fmt.Errorf("table %q not found", cr.Table)
		}
		idx, err := t.ColumnIndex(cr.Name)
		if err != nil {
			return nil, err
		}
		return &colRef{table: cr.Table, name: cr.Name, idx: idx, resolvedTable: t.Name}, nil
	}
	var found *colRef
	foundCount := 0
	for alias, t := range scope {
		if idx, err := t.ColumnIndex(cr.Name); err == nil {
			found = &colRef{name: cr.Name, idx: idx, resolvedTable: t.Name, table: alias}
			foundCount++
		}
	}
	if foundCount == 0 {
		return nil, fmt.Errorf("column %q not found in any table", cr.Name)
	}
	if foundCount > 1 {
		return nil, fmt.Errorf("column %q is ambiguous", cr.Name)
	}
	return found, nil
}

func (e *Executor) compileFunc(fc *parser.FuncCall, scope map[string]*csvpkg.Table, allowAgg bool) (ExprRef, error) {
	name := strings.ToUpper(fc.Name)
	isAgg := name == "COUNT" || name == "SUM" || name == "AVG" || name == "MIN" || name == "MAX"
	if isAgg && !allowAgg {
		return nil, fmt.Errorf("aggregate function %s not allowed in this context", name)
	}
	args := make([]ExprRef, len(fc.Args))
	var err error
	for i, a := range fc.Args {
		args[i], err = e.compileExpr(a, scope, allowAgg)
		if err != nil {
			return nil, err
		}
	}
	return &funcRef{name: name, args: args, star: fc.Star, distinct: fc.Distinct}, nil
}
