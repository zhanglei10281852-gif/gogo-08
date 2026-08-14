package executor

import (
	csvpkg "csvsql/internal/csv"
	"csvsql/internal/parser"
	"csvsql/internal/types"
)

type plan struct {
	tables   []planTable
	joins    []planJoin
	filter   ExprRef
	groupBy  []ExprRef
	having   ExprRef
	selects  []selectItem
	orderBy  []orderItemRef
	limit    *int64
	offset   *int64
	distinct bool
}

type planTable struct {
	name  string
	alias string
	table *csvpkg.Table
}

type planJoin struct {
	typ   parser.JoinType
	left  string
	right planTable
	on    ExprRef
}

type selectItem struct {
	expr  ExprRef
	alias string
	star  bool
	table string
}

type orderItemRef struct {
	expr ExprRef
	desc bool
}

type rowView struct {
	table  *csvpkg.Table
	rowIdx int
	values []types.Value
}

type joinRow struct {
	views map[string]*rowView
	match bool
}

type aggregateContext struct{}

type ExprRef interface {
	isExprRef()
}

type constRef struct {
	val types.Value
}

func (*constRef) isExprRef() {}

type colRef struct {
	table         string
	name          string
	idx           int
	resolvedTable string
}

func (*colRef) isExprRef() {}

type binOpRef struct {
	op    parser.BinaryOp
	left  ExprRef
	right ExprRef
}

func (*binOpRef) isExprRef() {}

type unaryOpRef struct {
	op   parser.UnaryOp
	expr ExprRef
}

func (*unaryOpRef) isExprRef() {}

type funcRef struct {
	name     string
	args     []ExprRef
	star     bool
	distinct bool
}

func (*funcRef) isExprRef() {}

type inListRef struct {
	expr   ExprRef
	not    bool
	values []ExprRef
}

func (*inListRef) isExprRef() {}

type betweenRef struct {
	expr ExprRef
	not  bool
	low  ExprRef
	high ExprRef
}

func (*betweenRef) isExprRef() {}

type caseRef struct {
	expr  ExprRef
	whens []whenRef
	else_ ExprRef
}

type whenRef struct {
	cond ExprRef
	then ExprRef
}

func (*caseRef) isExprRef() {}

type aggRow struct {
	gs      *groupState
	keyVals []types.Value
}

type groupState struct {
	aggVals []*aggregateState
}

type aggCallInfo struct {
	ref   *funcRef
	state *aggregateState
}

type aggregateState struct {
	funcName string
	distinct bool
	count    int64
	sum      float64
	sumValid bool
	min      types.Value
	max      types.Value
	seen     map[string]bool
}
