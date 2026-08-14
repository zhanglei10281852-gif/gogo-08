package parser

type Node interface {
	node()
}

type Expr interface {
	Node
	exprNode()
}

type SelectStmt struct {
	Distinct bool
	Columns  []SelectColumn
	From     *FromClause
	Where    Expr
	GroupBy  []Expr
	Having   Expr
	OrderBy  []OrderItem
	Limit    *int64
	Offset   *int64
}

func (*SelectStmt) node() {}

type SelectColumn struct {
	Expr  Expr
	Alias string
	Star  bool
	Table string
}

type FromClause struct {
	Tables []TableRef
	Joins  []JoinClause
}

type TableRef struct {
	Name  string
	Alias string
}

type JoinType int

const (
	JoinInner JoinType = iota
	JoinLeft
	JoinRight
)

type JoinClause struct {
	Type  JoinType
	Table TableRef
	On    Expr
}

type OrderItem struct {
	Expr  Expr
	Desc  bool
}

type ColumnRef struct {
	Table string
	Name  string
	Star  bool
}

func (*ColumnRef) node()     {}
func (*ColumnRef) exprNode() {}

type Literal struct {
	Kind  LiteralKind
	Value interface{}
	Text  string
}

type LiteralKind int

const (
	LitNull LiteralKind = iota
	LitInt
	LitFloat
	LitString
	LitBool
)

func (*Literal) node()     {}
func (*Literal) exprNode() {}

type BinaryExpr struct {
	Op    BinaryOp
	Left  Expr
	Right Expr
}

type BinaryOp int

const (
	OpAnd BinaryOp = iota
	OpOr
	OpEq
	OpNeq
	OpLt
	OpLte
	OpGt
	OpGte
	OpLike
	OpNotLike
	OpIn
	OpNotIn
	OpIs
	OpIsNot
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
)

func (*BinaryExpr) node()     {}
func (*BinaryExpr) exprNode() {}

type UnaryExpr struct {
	Op   UnaryOp
	Expr Expr
}

type UnaryOp int

const (
	OpNot UnaryOp = iota
	OpNeg
)

func (*UnaryExpr) node()     {}
func (*UnaryExpr) exprNode() {}

type FuncCall struct {
	Name     string
	Distinct bool
	Args     []Expr
	Star     bool
}

func (*FuncCall) node()     {}
func (*FuncCall) exprNode() {}

type InList struct {
	Expr   Expr
	Not    bool
	Values []Expr
}

func (*InList) node()     {}
func (*InList) exprNode() {}

type BetweenExpr struct {
	Expr    Expr
	Not     bool
	Low     Expr
	High    Expr
}

func (*BetweenExpr) node()     {}
func (*BetweenExpr) exprNode() {}

type CaseExpr struct {
	Expr  Expr
	Whens []WhenClause
	Else  Expr
}

type WhenClause struct {
	Cond Expr
	Then Expr
}

func (*CaseExpr) node()     {}
func (*CaseExpr) exprNode() {}

type CastExpr struct {
	Expr Expr
	Type string
}

func (*CastExpr) node()     {}
func (*CastExpr) exprNode() {}
