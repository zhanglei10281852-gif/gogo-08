package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenNumber
	TokenString
	TokenComma
	TokenLParen
	TokenRParen
	TokenDot
	TokenSemicolon
	TokenEq
	TokenNeq
	TokenLt
	TokenLte
	TokenGt
	TokenGte
	TokenPlus
	TokenMinus
	TokenStar
	TokenSlash
	TokenPercent

	KwSELECT
	KwFROM
	KwWHERE
	KwGROUP
	KwBY
	KwHAVING
	KwORDER
	KwLIMIT
	KwOFFSET
	KwJOIN
	KwINNER
	KwLEFT
	KwRIGHT
	KwOUTER
	KwON
	KwAND
	KwOR
	KwNOT
	KwIN
	KwLIKE
	KwIS
	KwNULL
	KwAS
	KwASC
	KwDESC
	KwDISTINCT
	KwBETWEEN
	KwTRUE
	KwFALSE
	KwCASE
	KwWHEN
	KwTHEN
	KwELSE
	KwEND
	KwCAST
)

var keywords = map[string]TokenType{
	"SELECT":   KwSELECT,
	"FROM":     KwFROM,
	"WHERE":    KwWHERE,
	"GROUP":    KwGROUP,
	"BY":       KwBY,
	"HAVING":   KwHAVING,
	"ORDER":    KwORDER,
	"LIMIT":    KwLIMIT,
	"OFFSET":   KwOFFSET,
	"JOIN":     KwJOIN,
	"INNER":    KwINNER,
	"LEFT":     KwLEFT,
	"RIGHT":    KwRIGHT,
	"OUTER":    KwOUTER,
	"ON":       KwON,
	"AND":      KwAND,
	"OR":       KwOR,
	"NOT":      KwNOT,
	"IN":       KwIN,
	"LIKE":     KwLIKE,
	"IS":       KwIS,
	"NULL":     KwNULL,
	"AS":       KwAS,
	"ASC":      KwASC,
	"DESC":     KwDESC,
	"DISTINCT": KwDISTINCT,
	"BETWEEN":  KwBETWEEN,
	"TRUE":     KwTRUE,
	"FALSE":    KwFALSE,
	"CASE":     KwCASE,
	"WHEN":     KwWHEN,
	"THEN":     KwTHEN,
	"ELSE":     KwELSE,
	"END":      KwEND,
	"CAST":     KwCAST,
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%v,%q,%d:%d)", t.Type, t.Literal, t.Line, t.Col)
}

type Lexer struct {
	input   []rune
	pos     int
	readPos int
	ch      rune
	line    int
	col     int
}

func New(input string) *Lexer {
	l := &Lexer{input: []rune(input), line: 1, col: 0}
	l.next()
	return l
}

func (l *Lexer) next() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peek() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.ch) {
		l.next()
	}
}

func (l *Lexer) skipComment() bool {
	if l.ch == '-' && l.peek() == '-' {
		for l.ch != '\n' && l.ch != 0 {
			l.next()
		}
		return true
	}
	if l.ch == '/' && l.peek() == '*' {
		l.next()
		l.next()
		for !(l.ch == '*' && l.peek() == '/') && l.ch != 0 {
			l.next()
		}
		if l.ch != 0 {
			l.next()
			l.next()
		}
		return true
	}
	return false
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isIdentPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

func (l *Lexer) readIdent() string {
	start := l.pos
	for isIdentPart(l.ch) {
		l.next()
	}
	return string(l.input[start:l.pos])
}

func (l *Lexer) readNumber() (string, bool) {
	start := l.pos
	isFloat := false
	for unicode.IsDigit(l.ch) {
		l.next()
	}
	if l.ch == '.' && unicode.IsDigit(l.peek()) {
		isFloat = true
		l.next()
		for unicode.IsDigit(l.ch) {
			l.next()
		}
	}
	if l.ch == 'e' || l.ch == 'E' {
		isFloat = true
		l.next()
		if l.ch == '+' || l.ch == '-' {
			l.next()
		}
		for unicode.IsDigit(l.ch) {
			l.next()
		}
	}
	return string(l.input[start:l.pos]), isFloat
}

func (l *Lexer) readString(quote rune) (string, error) {
	var sb strings.Builder
	l.next()
	for {
		if l.ch == 0 {
			return "", fmt.Errorf("unterminated string at line %d col %d", l.line, l.col)
		}
		if l.ch == quote {
			if l.peek() == quote {
				sb.WriteRune(quote)
				l.next()
				l.next()
				continue
			}
			l.next()
			return sb.String(), nil
		}
		if l.ch == '\\' {
			l.next()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '\\':
				sb.WriteRune('\\')
			case '\'':
				sb.WriteRune('\'')
			case '"':
				sb.WriteRune('"')
			default:
				sb.WriteRune(l.ch)
			}
			l.next()
			continue
		}
		sb.WriteRune(l.ch)
		l.next()
	}
}

func (l *Lexer) NextToken() (Token, error) {
	for {
		l.skipWhitespace()
		if l.ch == 0 {
			return Token{Type: TokenEOF, Line: l.line, Col: l.col}, nil
		}
		if !l.skipComment() {
			break
		}
	}

	startLine := l.line
	startCol := l.col

	ch := l.ch

	switch {
	case ch == ',':
		l.next()
		return Token{Type: TokenComma, Literal: ",", Line: startLine, Col: startCol}, nil
	case ch == '(':
		l.next()
		return Token{Type: TokenLParen, Literal: "(", Line: startLine, Col: startCol}, nil
	case ch == ')':
		l.next()
		return Token{Type: TokenRParen, Literal: ")", Line: startLine, Col: startCol}, nil
	case ch == '.':
		l.next()
		return Token{Type: TokenDot, Literal: ".", Line: startLine, Col: startCol}, nil
	case ch == ';':
		l.next()
		return Token{Type: TokenSemicolon, Literal: ";", Line: startLine, Col: startCol}, nil
	case ch == '=':
		l.next()
		return Token{Type: TokenEq, Literal: "=", Line: startLine, Col: startCol}, nil
	case ch == '!':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Type: TokenNeq, Literal: "!=", Line: startLine, Col: startCol}, nil
		}
		return Token{}, fmt.Errorf("unexpected '!' at line %d col %d", startLine, startCol)
	case ch == '<':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Type: TokenLte, Literal: "<=", Line: startLine, Col: startCol}, nil
		} else if l.peek() == '>' {
			l.next()
			l.next()
			return Token{Type: TokenNeq, Literal: "<>", Line: startLine, Col: startCol}, nil
		}
		l.next()
		return Token{Type: TokenLt, Literal: "<", Line: startLine, Col: startCol}, nil
	case ch == '>':
		if l.peek() == '=' {
			l.next()
			l.next()
			return Token{Type: TokenGte, Literal: ">=", Line: startLine, Col: startCol}, nil
		}
		l.next()
		return Token{Type: TokenGt, Literal: ">", Line: startLine, Col: startCol}, nil
	case ch == '+':
		l.next()
		return Token{Type: TokenPlus, Literal: "+", Line: startLine, Col: startCol}, nil
	case ch == '-':
		l.next()
		return Token{Type: TokenMinus, Literal: "-", Line: startLine, Col: startCol}, nil
	case ch == '*':
		l.next()
		return Token{Type: TokenStar, Literal: "*", Line: startLine, Col: startCol}, nil
	case ch == '/':
		l.next()
		return Token{Type: TokenSlash, Literal: "/", Line: startLine, Col: startCol}, nil
	case ch == '%':
		l.next()
		return Token{Type: TokenPercent, Literal: "%", Line: startLine, Col: startCol}, nil
	case ch == '\'' || ch == '"' || ch == '`':
		if ch == '`' {
			l.next()
			start := l.pos
			for l.ch != '`' && l.ch != 0 {
				l.next()
			}
			ident := string(l.input[start:l.pos])
			if l.ch == '`' {
				l.next()
			}
			return Token{Type: TokenIdent, Literal: ident, Line: startLine, Col: startCol}, nil
		}
		s, err := l.readString(ch)
		if err != nil {
			return Token{}, err
		}
		return Token{Type: TokenString, Literal: s, Line: startLine, Col: startCol}, nil
	case isIdentStart(ch):
		ident := l.readIdent()
		upper := strings.ToUpper(ident)
		if kw, ok := keywords[upper]; ok {
			return Token{Type: kw, Literal: ident, Line: startLine, Col: startCol}, nil
		}
		return Token{Type: TokenIdent, Literal: ident, Line: startLine, Col: startCol}, nil
	case unicode.IsDigit(ch) || (ch == '.' && unicode.IsDigit(l.peek())):
		numStr, _ := l.readNumber()
		return Token{Type: TokenNumber, Literal: numStr, Line: startLine, Col: startCol}, nil
	}

	return Token{}, fmt.Errorf("unexpected character %q at line %d col %d", ch, startLine, startCol)
}

func (l *Lexer) AllTokens() ([]Token, error) {
	var toks []Token
	for {
		t, err := l.NextToken()
		if err != nil {
			return nil, err
		}
		toks = append(toks, t)
		if t.Type == TokenEOF {
			break
		}
	}
	return toks, nil
}
