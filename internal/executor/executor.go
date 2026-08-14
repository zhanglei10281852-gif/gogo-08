package executor

import (
	"fmt"
	"sort"
	"strings"

	csvpkg "csvsql/internal/csv"
	"csvsql/internal/parser"
	"csvsql/internal/types"
)

type Executor struct {
	tables map[string]*csvpkg.Table
}

func New(tables []*csvpkg.Table) *Executor {
	m := make(map[string]*csvpkg.Table)
	for _, t := range tables {
		m[strings.ToLower(t.Name)] = t
	}
	return &Executor{tables: m}
}

type QueryResult struct {
	Columns []string
	Types   []types.ValueType
	Rows    [][]types.Value
}

func (e *Executor) Execute(stmt *parser.SelectStmt) (*QueryResult, error) {
	p, err := e.planQuery(stmt)
	if err != nil {
		return nil, err
	}
	return e.runPlan(p)
}

func (e *Executor) lookupTable(name string) (*csvpkg.Table, error) {
	t, ok := e.tables[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("table %q not found", name)
	}
	return t, nil
}

func (e *Executor) runPlan(p *plan) (*QueryResult, error) {
	if len(p.tables) == 0 {
		return e.runNoTables(p)
	}

	joinRows, err := e.materializeJoin(p)
	if err != nil {
		return nil, err
	}

	filtered := make([]*joinRow, 0, len(joinRows))
	for _, jr := range joinRows {
		if p.filter != nil {
			v, err := e.evalExpr(p.filter, jr, nil)
			if err != nil {
				return nil, err
			}
			if !v.AsBool() {
				continue
			}
		}
		filtered = append(filtered, jr)
	}

	if len(p.groupBy) > 0 || p.hasAggregate() {
		return e.runAggregation(p, filtered)
	}

	return e.runProjection(p, filtered)
}

func (p *plan) hasAggregate() bool {
	for _, s := range p.selects {
		if hasAgg(s.expr) {
			return true
		}
	}
	if hasAgg(p.having) {
		return true
	}
	for _, o := range p.orderBy {
		if hasAgg(o.expr) {
			return true
		}
	}
	return false
}

func hasAgg(r ExprRef) bool {
	if r == nil {
		return false
	}
	switch x := r.(type) {
	case *funcRef:
		switch x.name {
		case "COUNT", "SUM", "AVG", "MIN", "MAX":
			return true
		}
		for _, a := range x.args {
			if hasAgg(a) {
				return true
			}
		}
	case *binOpRef:
		return hasAgg(x.left) || hasAgg(x.right)
	case *unaryOpRef:
		return hasAgg(x.expr)
	case *inListRef:
		if hasAgg(x.expr) {
			return true
		}
		for _, v := range x.values {
			if hasAgg(v) {
				return true
			}
		}
	case *betweenRef:
		return hasAgg(x.expr) || hasAgg(x.low) || hasAgg(x.high)
	case *caseRef:
		if hasAgg(x.expr) {
			return true
		}
		for _, w := range x.whens {
			if hasAgg(w.cond) || hasAgg(w.then) {
				return true
			}
		}
		if hasAgg(x.else_) {
			return true
		}
	}
	return false
}

func (e *Executor) runNoTables(p *plan) (*QueryResult, error) {
	jr := &joinRow{views: make(map[string]*rowView)}
	cols, types, rows, err := e.projectRow(p, jr, nil)
	if err != nil {
		return nil, err
	}
	return &QueryResult{Columns: cols, Types: types, Rows: rows}, nil
}

func (e *Executor) materializeJoin(p *plan) ([]*joinRow, error) {
	base := p.tables[0]
	rows := make([]*joinRow, 0, len(base.table.Rows))
	for i, r := range base.table.Rows {
		views := make(map[string]*rowView)
		rv := &rowView{table: base.table, rowIdx: i, values: r}
		views[strings.ToLower(base.alias)] = rv
		views[strings.ToLower(base.name)] = rv
		rows = append(rows, &joinRow{views: views, match: true})
	}

	for _, pj := range p.joins {
		newRows := make([]*joinRow, 0)
		rightTbl := pj.right
		for _, jr := range rows {
			matched := false
			for i, rr := range rightTbl.table.Rows {
				newViews := make(map[string]*rowView)
				for k, v := range jr.views {
					newViews[k] = v
				}
				rv := &rowView{table: rightTbl.table, rowIdx: i, values: rr}
				newViews[strings.ToLower(rightTbl.alias)] = rv
				newViews[strings.ToLower(rightTbl.name)] = rv
				newJR := &joinRow{views: newViews, match: true}
				if pj.on != nil {
					v, err := e.evalExpr(pj.on, newJR, nil)
					if err != nil {
						return nil, err
					}
					if !v.AsBool() {
						continue
					}
				}
				matched = true
				newRows = append(newRows, newJR)
			}
			if !matched && pj.typ != parser.JoinInner {
				newViews := make(map[string]*rowView)
				for k, v := range jr.views {
					newViews[k] = v
				}
				nullRow := make([]types.Value, len(rightTbl.table.Columns))
				for j := range nullRow {
					nullRow[j] = types.NullValue()
				}
				rv := &rowView{table: rightTbl.table, rowIdx: -1, values: nullRow}
				newViews[strings.ToLower(rightTbl.alias)] = rv
				newViews[strings.ToLower(rightTbl.name)] = rv
				newRows = append(newRows, &joinRow{views: newViews, match: false})
			}
		}
		rows = newRows
	}

	return rows, nil
}

func (e *Executor) projectRow(p *plan, jr *joinRow, _ *aggregateContext) ([]string, []types.ValueType, [][]types.Value, error) {
	var columns []string
	var colTypes []types.ValueType
	var row []types.Value

	seenCols := make(map[string]bool)

	for _, si := range p.selects {
		if si.star {
			for _, pt := range p.tables {
				if si.table != "" && !strings.EqualFold(si.table, pt.alias) && !strings.EqualFold(si.table, pt.name) {
					continue
				}
				rv := jr.views[strings.ToLower(pt.alias)]
				if rv == nil {
					continue
				}
				for ci, col := range rv.table.Columns {
					colName := col.Name
					key := strings.ToLower(pt.alias + "." + colName)
					if seenCols[key] {
						continue
					}
					seenCols[key] = true
					columns = append(columns, colName)
					colTypes = append(colTypes, col.Type)
					if ci < len(rv.values) {
						row = append(row, rv.values[ci])
					} else {
						row = append(row, types.NullValue())
					}
				}
			}
			continue
		}
		val, err := e.evalExpr(si.expr, jr, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		alias := si.alias
		if alias == "" {
			alias = exprDisplayName(si.expr)
		}
		columns = append(columns, alias)
		colTypes = append(colTypes, val.Type)
		row = append(row, val)
	}

	return columns, colTypes, [][]types.Value{row}, nil
}

func exprDisplayName(r ExprRef) string {
	switch x := r.(type) {
	case *colRef:
		return x.name
	case *constRef:
		return x.val.AsText()
	case *funcRef:
		return x.name + "(...)"
	default:
		return "expr"
	}
}

type projectedRow struct {
	values []types.Value
	source *joinRow
}

func (e *Executor) runProjection(p *plan, rows []*joinRow) (*QueryResult, error) {
	if len(rows) == 0 {
		emptyViews := make(map[string]*rowView)
		for _, pt := range p.tables {
			nullRow := make([]types.Value, len(pt.table.Columns))
			for j := range nullRow {
				nullRow[j] = types.NullValue()
			}
			rv := &rowView{table: pt.table, rowIdx: -1, values: nullRow}
			emptyViews[strings.ToLower(pt.alias)] = rv
			emptyViews[strings.ToLower(pt.name)] = rv
		}
		cols, types, _, err := e.projectRow(p, &joinRow{views: emptyViews}, nil)
		if err != nil {
			return nil, err
		}
		return &QueryResult{Columns: cols, Types: types, Rows: nil}, nil
	}

	result := &QueryResult{}
	projected := make([]projectedRow, 0, len(rows))
	first := true
	for _, jr := range rows {
		cols, tps, r, err := e.projectRow(p, jr, nil)
		if err != nil {
			return nil, err
		}
		if first {
			result.Columns = cols
			result.Types = tps
			first = false
		}
		projected = append(projected, projectedRow{values: r[0], source: jr})
	}

	if p.distinct {
		projected = distinctProjectedRows(projected)
	}
	if err := e.applyOrderBySimple(p, projected); err != nil {
		return nil, err
	}
	for _, output := range projected {
		result.Rows = append(result.Rows, output.values)
	}
	applyLimitOffset(p, result)

	return result, nil
}

func distinctProjectedRows(rows []projectedRow) []projectedRow {
	seen := make(map[string]bool)
	unique := make([]projectedRow, 0, len(rows))
	for _, row := range rows {
		key := rowKey(row.values)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, row)
		}
	}
	return unique
}

func rowKey(row []types.Value) string {
	var sb strings.Builder
	for i, v := range row {
		if i > 0 {
			sb.WriteByte(0)
		}
		sb.WriteString(v.AsText())
		sb.WriteByte(0)
		sb.WriteString(v.Type.String())
	}
	return sb.String()
}

func applyLimitOffset(p *plan, result *QueryResult) {
	start := int64(0)
	if p.offset != nil {
		start = *p.offset
	}
	if start > int64(len(result.Rows)) {
		result.Rows = nil
		return
	}
	result.Rows = result.Rows[start:]
	if p.limit != nil {
		lim := *p.limit
		if lim < int64(len(result.Rows)) {
			result.Rows = result.Rows[:lim]
		}
	}
}

func (e *Executor) applyOrderBySimple(p *plan, rows []projectedRow) error {
	if len(p.orderBy) == 0 {
		return nil
	}
	type item struct {
		row       projectedRow
		orderKeys []types.Value
	}
	items := make([]item, len(rows))
	for i, row := range rows {
		items[i] = item{row: row, orderKeys: make([]types.Value, len(p.orderBy))}
		for j, order := range p.orderBy {
			value, err := e.evalExpr(order.expr, row.source, nil)
			if err != nil {
				return err
			}
			items[i].orderKeys[j] = value
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		for k, order := range p.orderBy {
			comparison := compareValues(items[i].orderKeys[k], items[j].orderKeys[k])
			if comparison != 0 {
				if order.desc {
					return comparison > 0
				}
				return comparison < 0
			}
		}
		return false
	})
	for i, item := range items {
		rows[i] = item.row
	}
	return nil
}
