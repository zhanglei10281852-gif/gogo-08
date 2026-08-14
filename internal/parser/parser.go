package parser

import (
	"fmt"
	"strconv"

	"csvsql/internal/lexer"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
}

func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) cur() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peek(n int) lexer.Token {
	i := p.pos + n
	if i >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[i]
}

func (p *Parser) advance() lexer.Token {
	t := p.cur()
	if t.Type != lexer.TokenEOF {
		p.pos++
	}
	return t
}

func (p *Parser) expect(types ...lexer.TokenType) (lexer.Token, error) {
	t := p.cur()
	for _, typ := range types {
		if t.Type == typ {
			p.advance()
			return t, nil
		}
	}
	return t, fmt.Errorf("expected %v, got %v (%q) at line %d col %d",
		types, t.Type, t.Literal, t.Line, t.Col)
}

func (p *Parser) match(types ...lexer.TokenType) bool {
	t := p.cur()
	for _, typ := range types {
		if t.Type == typ {
			return true
		}
	}
	return false
}

func (p *Parser) accept(types ...lexer.TokenType) (lexer.Token, bool) {
	if p.match(types...) {
		return p.advance(), true
	}
	return lexer.Token{}, false
}

func (p *Parser) Parse() (*SelectStmt, error) {
	for p.cur().Type == lexer.TokenSemicolon {
		p.advance()
	}
	if p.cur().Type == lexer.TokenEOF {
		return nil, fmt.Errorf("empty SQL statement")
	}
	if p.cur().Type != lexer.KwSELECT {
		return nil, fmt.Errorf("expected SELECT, got %q at line %d col %d",
			p.cur().Literal, p.cur().Line, p.cur().Col)
	}
	stmt, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	if _, ok := p.accept(lexer.TokenSemicolon); ok {
	}
	for p.cur().Type == lexer.TokenSemicolon {
		p.advance()
	}
	if p.cur().Type != lexer.TokenEOF {
		return nil, fmt.Errorf("unexpected token %q at line %d col %d",
			p.cur().Literal, p.cur().Line, p.cur().Col)
	}
	return stmt, nil
}

func (p *Parser) parseSelect() (*SelectStmt, error) {
	stmt := &SelectStmt{}
	if _, err := p.expect(lexer.KwSELECT); err != nil {
		return nil, err
	}
	if _, ok := p.accept(lexer.KwDISTINCT); ok {
		stmt.Distinct = true
	}
	cols, err := p.parseSelectColumns()
	if err != nil {
		return nil, err
	}
	stmt.Columns = cols

	if p.cur().Type == lexer.KwFROM {
		from, err := p.parseFrom()
		if err != nil {
			return nil, err
		}
		stmt.From = from
	}

	if p.cur().Type == lexer.KwWHERE {
		p.advance()
		where, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	if p.cur().Type == lexer.KwGROUP {
		p.advance()
		if _, err := p.expect(lexer.KwBY); err != nil {
			return nil, err
		}
		exprs, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = exprs
	}

	if p.cur().Type == lexer.KwHAVING {
		p.advance()
		having, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Having = having
	}

	if p.cur().Type == lexer.KwORDER {
		p.advance()
		if _, err := p.expect(lexer.KwBY); err != nil {
			return nil, err
		}
		items, err := p.parseOrderByItems()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = items
	}

	if p.cur().Type == lexer.KwLIMIT {
		p.advance()
		n, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		stmt.Limit = &n
		if _, ok := p.accept(lexer.KwOFFSET, lexer.TokenComma); ok {
			o, err := p.parseNumber()
			if err != nil {
				return nil, err
			}
			stmt.Offset = &o
		}
	} else if p.cur().Type == lexer.KwOFFSET {
		p.advance()
		o, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		stmt.Offset = &o
		if p.cur().Type == lexer.KwLIMIT {
			p.advance()
			n, err := p.parseNumber()
			if err != nil {
				return nil, err
			}
			stmt.Limit = &n
		}
	}

	return stmt, nil
}

func (p *Parser) parseSelectColumns() ([]SelectColumn, error) {
	var cols []SelectColumn
	for {
		if p.cur().Type == lexer.TokenStar {
			p.advance()
			cols = append(cols, SelectColumn{Star: true})
		} else {
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			col := SelectColumn{Expr: expr}
			if _, ok := p.accept(lexer.KwAS); ok {
				t, err := p.expect(lexer.TokenIdent, lexer.TokenString)
				if err != nil {
					return nil, err
				}
				col.Alias = t.Literal
			} else if p.cur().Type == lexer.TokenIdent {
				col.Alias = p.cur().Literal
				p.advance()
			}
			cols = append(cols, col)
		}
		if _, ok := p.accept(lexer.TokenComma); !ok {
			break
		}
	}
	return cols, nil
}

func (p *Parser) parseFrom() (*FromClause, error) {
	if _, err := p.expect(lexer.KwFROM); err != nil {
		return nil, err
	}
	from := &FromClause{}

	first, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	from.Tables = append(from.Tables, first)

	for p.cur().Type == lexer.TokenComma {
		p.advance()
		tr, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		from.Tables = append(from.Tables, tr)
	}

	for {
		jt := JoinInner
		switch p.cur().Type {
		case lexer.KwJOIN:
			p.advance()
		case lexer.KwINNER:
			p.advance()
			if _, err := p.expect(lexer.KwJOIN); err != nil {
				return nil, err
			}
		case lexer.KwLEFT:
			p.advance()
			p.accept(lexer.KwOUTER)
			if _, err := p.expect(lexer.KwJOIN); err != nil {
				return nil, err
			}
			jt = JoinLeft
		case lexer.KwRIGHT:
			p.advance()
			p.accept(lexer.KwOUTER)
			if _, err := p.expect(lexer.KwJOIN); err != nil {
				return nil, err
			}
			jt = JoinRight
		default:
			return from, nil
		}
		tr, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.KwON); err != nil {
			return nil, err
		}
		onExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		from.Joins = append(from.Joins, JoinClause{
			Type:  jt,
			Table: tr,
			On:    onExpr,
		})
	}
}

func (p *Parser) parseTableRef() (TableRef, error) {
	t, err := p.expect(lexer.TokenIdent, lexer.TokenString)
	if err != nil {
		return TableRef{}, err
	}
	tr := TableRef{Name: t.Literal}
	if _, ok := p.accept(lexer.KwAS); ok {
		a, err := p.expect(lexer.TokenIdent, lexer.TokenString)
		if err != nil {
			return TableRef{}, err
		}
		tr.Alias = a.Literal
	} else if p.cur().Type == lexer.TokenIdent {
		tr.Alias = p.cur().Literal
		p.advance()
	}
	return tr, nil
}

func (p *Parser) parseExprList() ([]Expr, error) {
	var list []Expr
	for {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		list = append(list, e)
		if _, ok := p.accept(lexer.TokenComma); !ok {
			break
		}
	}
	return list, nil
}

func (p *Parser) parseOrderByItems() ([]OrderItem, error) {
	var items []OrderItem
	for {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		item := OrderItem{Expr: e}
		if _, ok := p.accept(lexer.KwDESC); ok {
			item.Desc = true
		} else {
			p.accept(lexer.KwASC)
		}
		items = append(items, item)
		if _, ok := p.accept(lexer.TokenComma); !ok {
			break
		}
	}
	return items, nil
}

func (p *Parser) parseNumber() (int64, error) {
	t, err := p.expect(lexer.TokenNumber)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(t.Literal, 10, 64)
}

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.cur().Type == lexer.KwOR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: OpOr, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.cur().Type == lexer.KwAND {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: OpAnd, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (Expr, error) {
	count := 0
	for p.cur().Type == lexer.KwNOT {
		p.advance()
		count++
	}
	e, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		e = &UnaryExpr{Op: OpNot, Expr: e}
	}
	return e, nil
}

func (p *Parser) parseComparison() (Expr, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Type {
		case lexer.TokenEq:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpEq, Left: left, Right: right}
		case lexer.TokenNeq:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpNeq, Left: left, Right: right}
		case lexer.TokenLt:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpLt, Left: left, Right: right}
		case lexer.TokenLte:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpLte, Left: left, Right: right}
		case lexer.TokenGt:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpGt, Left: left, Right: right}
		case lexer.TokenGte:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpGte, Left: left, Right: right}
		case lexer.KwLIKE:
			p.advance()
			right, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpLike, Left: left, Right: right}
		case lexer.KwIS:
			p.advance()
			not := false
			if _, ok := p.accept(lexer.KwNOT); ok {
				not = true
			}
			if p.cur().Type == lexer.KwNULL {
				p.advance()
				right := &Literal{Kind: LitNull}
				if not {
					left = &BinaryExpr{Op: OpIsNot, Left: left, Right: right}
				} else {
					left = &BinaryExpr{Op: OpIs, Left: left, Right: right}
				}
			} else {
				right, err := p.parseAddSub()
				if err != nil {
					return nil, err
				}
				if not {
					left = &BinaryExpr{Op: OpNeq, Left: left, Right: right}
				} else {
					left = &BinaryExpr{Op: OpEq, Left: left, Right: right}
				}
			}
		case lexer.KwIN:
			p.advance()
			return p.parseInExpr(left, false)
		case lexer.KwNOT:
			if p.peek(1).Type == lexer.KwIN {
				p.advance()
				p.advance()
				return p.parseInExpr(left, true)
			}
			if p.peek(1).Type == lexer.KwLIKE {
				p.advance()
				p.advance()
				right, err := p.parseAddSub()
				if err != nil {
					return nil, err
				}
				left = &BinaryExpr{Op: OpNotLike, Left: left, Right: right}
			} else if p.peek(1).Type == lexer.KwBETWEEN {
				p.advance()
				p.advance()
				return p.parseBetweenExpr(left, true)
			} else {
				return left, nil
			}
		case lexer.KwBETWEEN:
			p.advance()
			return p.parseBetweenExpr(left, false)
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseInExpr(left Expr, not bool) (Expr, error) {
	if _, err := p.expect(lexer.TokenLParen); err != nil {
		return nil, err
	}
	vals, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenRParen); err != nil {
		return nil, err
	}
	return &InList{Expr: left, Not: not, Values: vals}, nil
}

func (p *Parser) parseBetweenExpr(left Expr, not bool) (Expr, error) {
	low, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.KwAND); err != nil {
		return nil, err
	}
	high, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	return &BetweenExpr{Expr: left, Not: not, Low: low, High: high}, nil
}

func (p *Parser) parseAddSub() (Expr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Type {
		case lexer.TokenPlus:
			p.advance()
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpAdd, Left: left, Right: right}
		case lexer.TokenMinus:
			p.advance()
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpSub, Left: left, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseMulDiv() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Type {
		case lexer.TokenStar:
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpMul, Left: left, Right: right}
		case lexer.TokenSlash:
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpDiv, Left: left, Right: right}
		case lexer.TokenPercent:
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: OpMod, Left: left, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseUnary() (Expr, error) {
	switch p.cur().Type {
	case lexer.TokenMinus:
		p.advance()
		e, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: OpNeg, Expr: e}, nil
	case lexer.TokenPlus:
		p.advance()
		return p.parseUnary()
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expr, error) {
	switch p.cur().Type {
	case lexer.TokenLParen:
		p.advance()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return e, nil
	case lexer.TokenNumber:
		t := p.advance()
		return parseNumberLiteral(t.Literal)
	case lexer.TokenString:
		t := p.advance()
		return &Literal{Kind: LitString, Value: t.Literal, Text: t.Literal}, nil
	case lexer.KwNULL:
		p.advance()
		return &Literal{Kind: LitNull}, nil
	case lexer.KwTRUE:
		p.advance()
		return &Literal{Kind: LitBool, Value: true}, nil
	case lexer.KwFALSE:
		p.advance()
		return &Literal{Kind: LitBool, Value: false}, nil
	case lexer.TokenIdent:
		return p.parseIdentOrFunc()
	case lexer.TokenStar:
		p.advance()
		return &ColumnRef{Star: true}, nil
	case lexer.KwCASE:
		return p.parseCase()
	}
	return nil, fmt.Errorf("unexpected token %q at line %d col %d",
		p.cur().Literal, p.cur().Line, p.cur().Col)
}

func parseNumberLiteral(s string) (*Literal, error) {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &Literal{Kind: LitInt, Value: i, Text: s}, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &Literal{Kind: LitFloat, Value: f, Text: s}, nil
	}
	return nil, fmt.Errorf("invalid number literal: %s", s)
}

func (p *Parser) parseIdentOrFunc() (Expr, error) {
	t := p.advance()
	name := t.Literal

	if p.cur().Type == lexer.TokenDot {
		p.advance()
		if p.cur().Type == lexer.TokenStar {
			p.advance()
			return &ColumnRef{Table: name, Star: true}, nil
		}
		colTok, err := p.expect(lexer.TokenIdent)
		if err != nil {
			return nil, err
		}
		return &ColumnRef{Table: name, Name: colTok.Literal}, nil
	}

	if p.cur().Type == lexer.TokenLParen {
		p.advance()
		fc := &FuncCall{Name: name}
		if p.cur().Type == lexer.TokenStar {
			p.advance()
			fc.Star = true
		} else {
			if _, ok := p.accept(lexer.KwDISTINCT); ok {
				fc.Distinct = true
			}
			if p.cur().Type != lexer.TokenRParen {
				args, err := p.parseExprList()
				if err != nil {
					return nil, err
				}
				fc.Args = args
			}
		}
		if _, err := p.expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return fc, nil
	}

	return &ColumnRef{Name: name}, nil
}

func (p *Parser) parseCase() (Expr, error) {
	if _, err := p.expect(lexer.KwCASE); err != nil {
		return nil, err
	}
	ce := &CaseExpr{}
	if p.cur().Type != lexer.KwWHEN {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.Expr = e
	}
	for p.cur().Type == lexer.KwWHEN {
		p.advance()
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.KwTHEN); err != nil {
			return nil, err
		}
		then, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.Whens = append(ce.Whens, WhenClause{Cond: cond, Then: then})
	}
	if _, ok := p.accept(lexer.KwELSE); ok {
		elseExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.Else = elseExpr
	}
	if _, err := p.expect(lexer.KwEND); err != nil {
		return nil, err
	}
	return ce, nil
}
