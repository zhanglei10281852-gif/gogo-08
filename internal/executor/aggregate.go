package executor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"csvsql/internal/types"
)

type groupKey struct {
	values []types.Value
}

func (g groupKey) String() string {
	var sb strings.Builder
	for i, v := range g.values {
		if i > 0 {
			sb.WriteByte(0)
		}
		sb.WriteString(v.AsText())
		sb.WriteByte(0)
		sb.WriteString(v.Type.String())
	}
	return sb.String()
}

func (e *Executor) runAggregation(p *plan, rows []*joinRow) (*QueryResult, error) {
	groupMap := make(map[string]*groupState)
	var groupOrder []string

	aggCalls := e.collectAggregates(p)

	ensureGroup := func(key string) *groupState {
		if gs, ok := groupMap[key]; ok {
			return gs
		}
		gs := &groupState{aggVals: make([]*aggregateState, len(aggCalls))}
		for i := range gs.aggVals {
			gs.aggVals[i] = &aggregateState{
				funcName: aggCalls[i].ref.name,
				distinct: aggCalls[i].ref.distinct,
				seen:     make(map[string]bool),
			}
		}
		groupMap[key] = gs
		groupOrder = append(groupOrder, key)
		return gs
	}

	groupKeyVals := make(map[string][]types.Value)

	for _, jr := range rows {
		keyVals := make([]types.Value, len(p.groupBy))
		for i, gexpr := range p.groupBy {
			v, err := e.evalExpr(gexpr, jr, nil)
			if err != nil {
				return nil, err
			}
			keyVals[i] = v
		}
		gk := groupKey{values: keyVals}
		keyStr := gk.String()
		gs := ensureGroup(keyStr)
		groupKeyVals[keyStr] = keyVals

		for i, call := range aggCalls {
			state := gs.aggVals[i]
			if err := e.updateAggregate(state, call.ref, jr); err != nil {
				return nil, err
			}
		}
	}

	if len(groupMap) == 0 && len(p.groupBy) == 0 {
		ensureGroup("")
		groupKeyVals[""] = nil
	}

	var aggRows []aggRow
	for _, key := range groupOrder {
		gs := groupMap[key]
		aggRows = append(aggRows, aggRow{gs: gs, keyVals: groupKeyVals[key]})
	}

	var havingFiltered []aggRow
	for _, ar := range aggRows {
		if p.having != nil {
			v, err := e.evalAggregateExpr(p.having, ar, p, aggCalls)
			if err != nil {
				return nil, err
			}
			if !v.AsBool() {
				continue
			}
		}
		havingFiltered = append(havingFiltered, ar)
	}

	result := &QueryResult{}
	first := true
	var outRows []outRow

	for _, ar := range havingFiltered {
		var (
			cols   []string
			ctypes []types.ValueType
			row    []types.Value
		)
		seenCols := make(map[string]bool)

		for _, si := range p.selects {
			if si.star {
				for _, pt := range p.tables {
					if si.table != "" && !strings.EqualFold(si.table, pt.alias) && !strings.EqualFold(si.table, pt.name) {
						continue
					}
					for ci, col := range pt.table.Columns {
						colName := col.Name
						key := strings.ToLower(pt.alias + "." + colName)
						if seenCols[key] {
							continue
						}
						seenCols[key] = true
						cols = append(cols, colName)
						ctypes = append(ctypes, col.Type)

						var v types.Value
						found := false
						for gi, gexpr := range p.groupBy {
							if cr, ok := gexpr.(*colRef); ok {
								if strings.EqualFold(cr.name, col.Name) && (cr.table == "" || strings.EqualFold(cr.table, pt.alias) || strings.EqualFold(cr.table, pt.name)) {
									if gi < len(ar.keyVals) {
										v = ar.keyVals[gi]
										found = true
										break
									}
								}
							}
						}
						if !found {
							_ = ci
							v = types.NullValue()
						}
						row = append(row, v)
					}
				}
				continue
			}

			val, err := e.evalAggregateExpr(si.expr, ar, p, aggCalls)
			if err != nil {
				return nil, err
			}
			alias := si.alias
			if alias == "" {
				alias = exprDisplayName(si.expr)
			}
			cols = append(cols, alias)
			ctypes = append(ctypes, val.Type)
			row = append(row, val)
		}

		if first {
			result.Columns = cols
			result.Types = ctypes
			first = false
		}
		outRows = append(outRows, outRow{vals: row, ar: ar})
	}

	if len(outRows) == 0 && !first {
		for i := range result.Columns {
			_ = i
		}
	}

	for _, or_ := range outRows {
		result.Rows = append(result.Rows, or_.vals)
	}

	if p.distinct {
		result = distinctRows(result)
	}

	e.applyOrderByAgg(p, result, outRows, aggCalls)
	applyLimitOffset(p, result)

	return result, nil
}

func (e *Executor) collectAggregates(p *plan) []aggCallInfo {
	var calls []aggCallInfo
	index := make(map[string]int)

	walk := func(r ExprRef) {
		walkExprRef(r, func(rf ExprRef) bool {
			fr, ok := rf.(*funcRef)
			if !ok {
				return true
			}
			switch fr.name {
			case "COUNT", "SUM", "AVG", "MIN", "MAX":
				key := aggKey(fr)
				if _, exists := index[key]; !exists {
					index[key] = len(calls)
					calls = append(calls, aggCallInfo{ref: fr})
				}
				return false
			}
			return true
		})
	}

	for _, s := range p.selects {
		if !s.star {
			walk(s.expr)
		}
	}
	walk(p.having)
	for _, o := range p.orderBy {
		walk(o.expr)
	}
	return calls
}

func walkExprRef(r ExprRef, fn func(ExprRef) bool) {
	if r == nil {
		return
	}
	if !fn(r) {
		return
	}
	switch x := r.(type) {
	case *binOpRef:
		walkExprRef(x.left, fn)
		walkExprRef(x.right, fn)
	case *unaryOpRef:
		walkExprRef(x.expr, fn)
	case *funcRef:
		for _, a := range x.args {
			walkExprRef(a, fn)
		}
	case *inListRef:
		walkExprRef(x.expr, fn)
		for _, v := range x.values {
			walkExprRef(v, fn)
		}
	case *betweenRef:
		walkExprRef(x.expr, fn)
		walkExprRef(x.low, fn)
		walkExprRef(x.high, fn)
	case *caseRef:
		walkExprRef(x.expr, fn)
		for _, w := range x.whens {
			walkExprRef(w.cond, fn)
			walkExprRef(w.then, fn)
		}
		walkExprRef(x.else_, fn)
	}
}

func aggKey(fr *funcRef) string {
	var sb strings.Builder
	sb.WriteString(fr.name)
	sb.WriteByte(0)
	if fr.distinct {
		sb.WriteByte(1)
	}
	sb.WriteByte(0)
	if fr.star {
		sb.WriteString("*")
	}
	for i, a := range fr.args {
		if i > 0 {
			sb.WriteByte(0)
		}
		sb.WriteString(exprRefKey(a))
	}
	return sb.String()
}

func exprRefKey(r ExprRef) string {
	var sb strings.Builder
	switch x := r.(type) {
	case *constRef:
		sb.WriteString("C:")
		sb.WriteString(x.val.AsText())
		sb.WriteString(x.val.Type.String())
	case *colRef:
		sb.WriteString("R:")
		sb.WriteString(x.table)
		sb.WriteString(".")
		sb.WriteString(x.name)
	case *binOpRef:
		sb.WriteString("B:")
		sb.WriteString(strconv.Itoa(int(x.op)))
		sb.WriteString("(")
		sb.WriteString(exprRefKey(x.left))
		sb.WriteString(",")
		sb.WriteString(exprRefKey(x.right))
		sb.WriteString(")")
	case *unaryOpRef:
		sb.WriteString("U:")
		sb.WriteString(strconv.Itoa(int(x.op)))
		sb.WriteString("(")
		sb.WriteString(exprRefKey(x.expr))
		sb.WriteString(")")
	case *funcRef:
		sb.WriteString("F:")
		sb.WriteString(x.name)
		sb.WriteString("(")
		for i, a := range x.args {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(exprRefKey(a))
		}
		sb.WriteString(")")
	}
	return sb.String()
}

func (e *Executor) updateAggregate(state *aggregateState, fr *funcRef, jr *joinRow) error {
	switch fr.name {
	case "COUNT":
		if fr.star {
			state.count++
			return nil
		}
		if len(fr.args) != 1 {
			return fmt.Errorf("COUNT requires 1 argument")
		}
		v, err := e.evalExpr(fr.args[0], jr, nil)
		if err != nil {
			return err
		}
		if v.IsNull() {
			return nil
		}
		if state.distinct {
			k := v.AsText() + "|" + v.Type.String()
			if state.seen[k] {
				return nil
			}
			state.seen[k] = true
		}
		state.count++
	case "SUM", "AVG":
		if len(fr.args) != 1 {
			return fmt.Errorf("%s requires 1 argument", fr.name)
		}
		v, err := e.evalExpr(fr.args[0], jr, nil)
		if err != nil {
			return err
		}
		if v.IsNull() {
			return nil
		}
		f, ok := v.AsNumber()
		if !ok {
			return fmt.Errorf("%s requires numeric argument, got %s", fr.name, v.Type)
		}
		if state.distinct {
			k := v.AsText() + "|" + v.Type.String()
			if state.seen[k] {
				return nil
			}
			state.seen[k] = true
		}
		state.sum += f
		state.sumValid = true
		state.count++
	case "MIN", "MAX":
		if len(fr.args) != 1 {
			return fmt.Errorf("%s requires 1 argument", fr.name)
		}
		v, err := e.evalExpr(fr.args[0], jr, nil)
		if err != nil {
			return err
		}
		if v.IsNull() {
			return nil
		}
		if state.distinct {
			k := v.AsText() + "|" + v.Type.String()
			if state.seen[k] {
				return nil
			}
			state.seen[k] = true
		}
		if state.min.IsNull() || compareValues(v, state.min) < 0 {
			state.min = v
		}
		if state.max.IsNull() || compareValues(v, state.max) > 0 {
			state.max = v
		}
		state.count++
	}
	return nil
}

func (e *Executor) evalAggregateExpr(r ExprRef, ar aggRow, p *plan, calls []aggCallInfo) (types.Value, error) {
	aggMap := make(map[string]*aggregateState)
	for i, call := range calls {
		key := aggKey(call.ref)
		aggMap[key] = ar.gs.aggVals[i]
	}
	var err error
	substituted := substituteAggregates(r, aggMap, &err, ar, p)
	if err != nil {
		return types.NullValue(), err
	}
	return e.evalExpr(substituted, &joinRow{views: make(map[string]*rowView)}, nil)
}

func substituteAggregates(r ExprRef, aggMap map[string]*aggregateState, perr *error, ar aggRow, p *plan) ExprRef {
	if r == nil {
		return nil
	}
	if cr, ok := r.(*colRef); ok {
		for gi, gexpr := range p.groupBy {
			if gcr, ok := gexpr.(*colRef); ok {
				if strings.EqualFold(gcr.name, cr.name) && (cr.table == "" || strings.EqualFold(gcr.table, cr.table)) {
					if gi < len(ar.keyVals) {
						return &constRef{val: ar.keyVals[gi]}
					}
				}
			}
		}
	}
	if fr, ok := r.(*funcRef); ok {
		switch fr.name {
		case "COUNT", "SUM", "AVG", "MIN", "MAX":
			key := aggKey(fr)
			state, ok := aggMap[key]
			if !ok {
				*perr = fmt.Errorf("aggregate not found: %s", fr.name)
				return &constRef{val: types.NullValue()}
			}
			v := finalizeAggregate(state)
			return &constRef{val: v}
		}
	}
	switch x := r.(type) {
	case *binOpRef:
		nl := substituteAggregates(x.left, aggMap, perr, ar, p)
		nr := substituteAggregates(x.right, aggMap, perr, ar, p)
		return &binOpRef{op: x.op, left: nl, right: nr}
	case *unaryOpRef:
		return &unaryOpRef{op: x.op, expr: substituteAggregates(x.expr, aggMap, perr, ar, p)}
	case *funcRef:
		args := make([]ExprRef, len(x.args))
		for i, a := range x.args {
			args[i] = substituteAggregates(a, aggMap, perr, ar, p)
		}
		return &funcRef{name: x.name, args: args, star: x.star, distinct: x.distinct}
	case *inListRef:
		ee := substituteAggregates(x.expr, aggMap, perr, ar, p)
		vals := make([]ExprRef, len(x.values))
		for i, v := range x.values {
			vals[i] = substituteAggregates(v, aggMap, perr, ar, p)
		}
		return &inListRef{expr: ee, not: x.not, values: vals}
	case *betweenRef:
		return &betweenRef{
			expr: substituteAggregates(x.expr, aggMap, perr, ar, p),
			not:  x.not,
			low:  substituteAggregates(x.low, aggMap, perr, ar, p),
			high: substituteAggregates(x.high, aggMap, perr, ar, p),
		}
	case *caseRef:
		cr := &caseRef{expr: substituteAggregates(x.expr, aggMap, perr, ar, p)}
		for _, w := range x.whens {
			cr.whens = append(cr.whens, whenRef{
				cond: substituteAggregates(w.cond, aggMap, perr, ar, p),
				then: substituteAggregates(w.then, aggMap, perr, ar, p),
			})
		}
		cr.else_ = substituteAggregates(x.else_, aggMap, perr, ar, p)
		return cr
	}
	return r
}

func finalizeAggregate(state *aggregateState) types.Value {
	switch state.funcName {
	case "COUNT":
		return types.IntValue(state.count)
	case "SUM":
		if !state.sumValid || state.count == 0 {
			return types.NullValue()
		}
		return types.FloatValue(state.sum)
	case "AVG":
		if !state.sumValid || state.count == 0 {
			return types.NullValue()
		}
		return types.FloatValue(state.sum / float64(state.count))
	case "MIN":
		if state.min.IsNull() {
			return types.NullValue()
		}
		return state.min
	case "MAX":
		if state.max.IsNull() {
			return types.NullValue()
		}
		return state.max
	}
	return types.NullValue()
}

type outRow struct {
	vals []types.Value
	ar   aggRow
}

func (e *Executor) applyOrderByAgg(p *plan, result *QueryResult, outRows []outRow, calls []aggCallInfo) {
	if len(p.orderBy) == 0 {
		return
	}
	type item struct {
		idx   int
		vals  []types.Value
		ar    aggRow
	}
	items := make([]item, len(result.Rows))
	for i := range result.Rows {
		or_ := outRows[i]
		items[i] = item{idx: i, vals: result.Rows[i], ar: or_.ar}
	}
	sort.SliceStable(items, func(a, b int) bool {
		for _, o := range p.orderBy {
			va, err1 := e.evalAggregateExpr(o.expr, items[a].ar, p, calls)
			vb, err2 := e.evalAggregateExpr(o.expr, items[b].ar, p, calls)
			if err1 != nil || err2 != nil {
				return false
			}
			c := compareValues(va, vb)
			if c != 0 {
				if o.desc {
					return c > 0
				}
				return c < 0
			}
		}
		return false
	})
	newRows := make([][]types.Value, len(result.Rows))
	for i, it := range items {
		newRows[i] = it.vals
	}
	result.Rows = newRows
}
