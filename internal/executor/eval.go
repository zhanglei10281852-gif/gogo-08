package executor

import (
	"fmt"
	"math"
	"strings"
	"time"

	"csvsql/internal/parser"
	"csvsql/internal/types"
)

func (e *Executor) evalExpr(r ExprRef, jr *joinRow, _ *aggregateContext) (types.Value, error) {
	switch x := r.(type) {
	case *constRef:
		return x.val, nil
	case *colRef:
		rv, ok := jr.views[strings.ToLower(x.table)]
		if !ok {
			return types.NullValue(), fmt.Errorf("table %q not found in scope", x.table)
		}
		if x.idx >= 0 && x.idx < len(rv.values) {
			return rv.values[x.idx], nil
		}
		return types.NullValue(), nil
	case *binOpRef:
		return e.evalBinary(x, jr)
	case *unaryOpRef:
		return e.evalUnary(x, jr)
	case *funcRef:
		return e.evalFunc(x, jr)
	case *inListRef:
		return e.evalInList(x, jr)
	case *betweenRef:
		return e.evalBetween(x, jr)
	case *caseRef:
		return e.evalCase(x, jr)
	}
	return types.NullValue(), fmt.Errorf("unknown expr ref type %T", r)
}

func (e *Executor) evalBinary(b *binOpRef, jr *joinRow) (types.Value, error) {
	l, err := e.evalExpr(b.left, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	r, err := e.evalExpr(b.right, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	return applyBinaryOp(b.op, l, r)
}

func applyBinaryOp(op parser.BinaryOp, l, r types.Value) (types.Value, error) {
	switch op {
	case parser.OpAnd:
		return types.BoolValue(l.AsBool() && r.AsBool()), nil
	case parser.OpOr:
		return types.BoolValue(l.AsBool() || r.AsBool()), nil
	case parser.OpEq:
		c := compareValues(l, r)
		return types.BoolValue(c == 0 && !(l.IsNull() || r.IsNull())), nil
	case parser.OpNeq:
		c := compareValues(l, r)
		return types.BoolValue(c != 0 && !(l.IsNull() || r.IsNull())), nil
	case parser.OpLt:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(compareValues(l, r) < 0), nil
	case parser.OpLte:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(compareValues(l, r) <= 0), nil
	case parser.OpGt:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(compareValues(l, r) > 0), nil
	case parser.OpGte:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(compareValues(l, r) >= 0), nil
	case parser.OpLike:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(likeMatch(l.AsText(), r.AsText())), nil
	case parser.OpNotLike:
		if l.IsNull() || r.IsNull() {
			return types.BoolValue(false), nil
		}
		return types.BoolValue(!likeMatch(l.AsText(), r.AsText())), nil
	case parser.OpIs:
		return types.BoolValue(l.IsNull() && r.IsNull()), nil
	case parser.OpIsNot:
		return types.BoolValue(!(l.IsNull() && r.IsNull())), nil
	case parser.OpAdd, parser.OpSub, parser.OpMul, parser.OpDiv, parser.OpMod:
		return applyArithOp(op, l, r)
	}
	return types.NullValue(), fmt.Errorf("unsupported binary op %d", op)
}

func applyArithOp(op parser.BinaryOp, l, r types.Value) (types.Value, error) {
	if l.IsNull() || r.IsNull() {
		return types.NullValue(), nil
	}
	ln, lok := l.AsNumber()
	rn, rok := r.AsNumber()
	if !lok || !rok {
		return types.NullValue(), fmt.Errorf("arithmetic requires numeric operands")
	}
	var result float64
	switch op {
	case parser.OpAdd:
		result = ln + rn
	case parser.OpSub:
		result = ln - rn
	case parser.OpMul:
		result = ln * rn
	case parser.OpDiv:
		if rn == 0 {
			return types.NullValue(), nil
		}
		result = ln / rn
	case parser.OpMod:
		if rn == 0 {
			return types.NullValue(), nil
		}
		result = math.Mod(ln, rn)
	}
	if l.Type == types.TypeInt && r.Type == types.TypeInt && op != parser.OpDiv && op != parser.OpMod {
		return types.IntValue(int64(result)), nil
	}
	return types.FloatValue(result), nil
}

func compareValues(a, b types.Value) int {
	if a.IsNull() && b.IsNull() {
		return 0
	}
	if a.IsNull() {
		return -1
	}
	if b.IsNull() {
		return 1
	}

	if a.Type == b.Type {
		switch a.Type {
		case types.TypeInt:
			av := a.V.(int64)
			bv := b.V.(int64)
			if av < bv {
				return -1
			} else if av > bv {
				return 1
			}
			return 0
		case types.TypeFloat:
			av := a.V.(float64)
			bv := b.V.(float64)
			if av < bv {
				return -1
			} else if av > bv {
				return 1
			}
			return 0
		case types.TypeBool:
			av := a.V.(bool)
			bv := b.V.(bool)
			if av == bv {
				return 0
			}
			if av {
				return 1
			}
			return -1
		case types.TypeDate, types.TypeDateTime:
			av := a.V.(time.Time)
			bv := b.V.(time.Time)
			if av.Before(bv) {
				return -1
			} else if av.After(bv) {
				return 1
			}
			return 0
		case types.TypeText:
			return strings.Compare(a.V.(string), b.V.(string))
		}
	}

	af, aok := a.AsNumber()
	bf, bok := b.AsNumber()
	if aok && bok {
		if af < bf {
			return -1
		} else if af > bf {
			return 1
		}
		return 0
	}

	return strings.Compare(a.AsText(), b.AsText())
}

func likeMatch(str, pattern string) bool {
	pattern = strings.ReplaceAll(pattern, "%", "*")
	pattern = strings.ReplaceAll(pattern, "_", "?")
	return wildcardMatch(str, pattern)
}

func wildcardMatch(str, pattern string) bool {
	s := []rune(str)
	p := []rune(pattern)
	n := len(s)
	m := len(p)
	i, j := 0, 0
	starIdx := -1
	matchIdx := 0
	for i < n {
		if j < m && (p[j] == '?' || p[j] == s[i]) {
			i++
			j++
		} else if j < m && p[j] == '*' {
			starIdx = j
			matchIdx = i
			j++
		} else if starIdx != -1 {
			j = starIdx + 1
			matchIdx++
			i = matchIdx
		} else {
			return false
		}
	}
	for j < m && p[j] == '*' {
		j++
	}
	return j == m
}

func (e *Executor) evalUnary(u *unaryOpRef, jr *joinRow) (types.Value, error) {
	v, err := e.evalExpr(u.expr, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	switch u.op {
	case parser.OpNot:
		return types.BoolValue(!v.AsBool()), nil
	case parser.OpNeg:
		if v.IsNull() {
			return types.NullValue(), nil
		}
		switch v.Type {
		case types.TypeInt:
			return types.IntValue(-v.V.(int64)), nil
		case types.TypeFloat:
			return types.FloatValue(-v.V.(float64)), nil
		default:
			if f, ok := v.AsNumber(); ok {
				return types.FloatValue(-f), nil
			}
		}
	}
	return types.NullValue(), nil
}

func (e *Executor) evalFunc(f *funcRef, jr *joinRow) (types.Value, error) {
	switch f.name {
	case "COUNT", "SUM", "AVG", "MIN", "MAX":
		return types.NullValue(), fmt.Errorf("aggregate %s used outside aggregation context", f.name)
	case "UPPER":
		if len(f.args) != 1 {
			return types.NullValue(), fmt.Errorf("UPPER requires 1 arg")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		return types.TextValue(strings.ToUpper(v.AsText())), nil
	case "LOWER":
		if len(f.args) != 1 {
			return types.NullValue(), fmt.Errorf("LOWER requires 1 arg")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		return types.TextValue(strings.ToLower(v.AsText())), nil
	case "LENGTH", "LEN":
		if len(f.args) != 1 {
			return types.NullValue(), fmt.Errorf("LENGTH requires 1 arg")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		return types.IntValue(int64(len([]rune(v.AsText())))), nil
	case "TRIM":
		if len(f.args) != 1 {
			return types.NullValue(), fmt.Errorf("TRIM requires 1 arg")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		return types.TextValue(strings.TrimSpace(v.AsText())), nil
	case "SUBSTR", "SUBSTRING":
		if len(f.args) < 2 || len(f.args) > 3 {
			return types.NullValue(), fmt.Errorf("SUBSTR requires 2-3 args")
		}
		s, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		start, err := e.evalExpr(f.args[1], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if s.IsNull() || start.IsNull() {
			return types.NullValue(), nil
		}
		runes := []rune(s.AsText())
		si, _ := start.AsNumber()
		idx := int(si)
		if idx < 1 {
			idx = 1
		}
		idx--
		if idx >= len(runes) {
			return types.TextValue(""), nil
		}
		end := len(runes)
		if len(f.args) == 3 {
			lv, err := e.evalExpr(f.args[2], jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
			if !lv.IsNull() {
				ln, _ := lv.AsNumber()
				li := int(ln)
				if li < 0 {
					li = 0
				}
				if idx+li < end {
					end = idx + li
				}
			}
		}
		return types.TextValue(string(runes[idx:end])), nil
	case "CONCAT":
		var sb strings.Builder
		for _, a := range f.args {
			v, err := e.evalExpr(a, jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
			if !v.IsNull() {
				sb.WriteString(v.AsText())
			}
		}
		return types.TextValue(sb.String()), nil
	case "COALESCE":
		for _, a := range f.args {
			v, err := e.evalExpr(a, jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
			if !v.IsNull() {
				return v, nil
			}
		}
		return types.NullValue(), nil
	case "IFNULL":
		if len(f.args) != 2 {
			return types.NullValue(), fmt.Errorf("IFNULL requires 2 args")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if !v.IsNull() {
			return v, nil
		}
		return e.evalExpr(f.args[1], jr, nil)
	case "ABS":
		if len(f.args) != 1 {
			return types.NullValue(), fmt.Errorf("ABS requires 1 arg")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		fn, ok := v.AsNumber()
		if !ok {
			return types.NullValue(), nil
		}
		if v.Type == types.TypeInt {
			if fn < 0 {
				fn = -fn
			}
			return types.IntValue(int64(fn)), nil
		}
		return types.FloatValue(math.Abs(fn)), nil
	case "ROUND":
		if len(f.args) < 1 || len(f.args) > 2 {
			return types.NullValue(), fmt.Errorf("ROUND requires 1-2 args")
		}
		v, err := e.evalExpr(f.args[0], jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if v.IsNull() {
			return types.NullValue(), nil
		}
		fn, ok := v.AsNumber()
		if !ok {
			return types.NullValue(), nil
		}
		prec := 0
		if len(f.args) == 2 {
			pv, err := e.evalExpr(f.args[1], jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
			pn, _ := pv.AsNumber()
			prec = int(pn)
		}
		mult := math.Pow(10, float64(prec))
		rounded := math.Round(fn*mult) / mult
		return types.FloatValue(rounded), nil
	case "NOW", "CURRENT_TIMESTAMP":
		return types.DateTimeValue(time.Now()), nil
	case "CURRENT_DATE":
		t := time.Now()
		y, m, d := t.Date()
		return types.DateValue(time.Date(y, m, d, 0, 0, 0, 0, t.Location())), nil
	}
	return types.NullValue(), fmt.Errorf("unknown function %s", f.name)
}

func (e *Executor) evalInList(in *inListRef, jr *joinRow) (types.Value, error) {
	v, err := e.evalExpr(in.expr, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	if v.IsNull() {
		return types.BoolValue(false), nil
	}
	for _, valRef := range in.values {
		v2, err := e.evalExpr(valRef, jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
		if compareValues(v, v2) == 0 {
			return types.BoolValue(!in.not), nil
		}
	}
	return types.BoolValue(in.not), nil
}

func (e *Executor) evalBetween(b *betweenRef, jr *joinRow) (types.Value, error) {
	v, err := e.evalExpr(b.expr, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	lo, err := e.evalExpr(b.low, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	hi, err := e.evalExpr(b.high, jr, nil)
	if err != nil {
		return types.NullValue(), err
	}
	if v.IsNull() || lo.IsNull() || hi.IsNull() {
		return types.BoolValue(false), nil
	}
	ok := compareValues(v, lo) >= 0 && compareValues(v, hi) <= 0
	if b.not {
		ok = !ok
	}
	return types.BoolValue(ok), nil
}

func (e *Executor) evalCase(c *caseRef, jr *joinRow) (types.Value, error) {
	var caseVal types.Value
	var err error
	if c.expr != nil {
		caseVal, err = e.evalExpr(c.expr, jr, nil)
		if err != nil {
			return types.NullValue(), err
		}
	}
	for _, w := range c.whens {
		var condVal types.Value
		if c.expr != nil {
			whenVal, err := e.evalExpr(w.cond, jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
			condVal = types.BoolValue(compareValues(caseVal, whenVal) == 0 && !caseVal.IsNull())
		} else {
			condVal, err = e.evalExpr(w.cond, jr, nil)
			if err != nil {
				return types.NullValue(), err
			}
		}
		if condVal.AsBool() {
			return e.evalExpr(w.then, jr, nil)
		}
	}
	if c.else_ != nil {
		return e.evalExpr(c.else_, jr, nil)
	}
	return types.NullValue(), nil
}
