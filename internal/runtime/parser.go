package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind string

const (
	tokenEOF     tokenKind = "eof"
	tokenIdent   tokenKind = "identifier"
	tokenInteger tokenKind = "integer"
	tokenString  tokenKind = "string"
	tokenSymbol  tokenKind = "symbol"
)

type token struct {
	kind tokenKind
	text string
	line int
	col  int
}

type lexer struct {
	source []rune
	index  int
	line   int
	col    int
}

func newLexer(source string) *lexer {
	return &lexer{source: []rune(source), line: 1, col: 1}
}

func (l *lexer) next() (token, error) {
	for l.index < len(l.source) {
		current := l.source[l.index]
		if current == ' ' || current == '\t' || current == '\r' || current == '\n' {
			l.advance(current)
			continue
		}
		if current == '/' && l.peek() == '/' {
			for l.index < len(l.source) && l.source[l.index] != '\n' {
				l.advance(l.source[l.index])
			}
			continue
		}
		line, col := l.line, l.col
		if unicode.IsLetter(current) || current == '_' {
			start := l.index
			for l.index < len(l.source) && (unicode.IsLetter(l.source[l.index]) || unicode.IsDigit(l.source[l.index]) || l.source[l.index] == '_') {
				l.advance(l.source[l.index])
			}
			return token{kind: tokenIdent, text: string(l.source[start:l.index]), line: line, col: col}, nil
		}
		if unicode.IsDigit(current) || (current == '-' && unicode.IsDigit(l.peek())) {
			start := l.index
			l.advance(current)
			for l.index < len(l.source) && unicode.IsDigit(l.source[l.index]) {
				l.advance(l.source[l.index])
			}
			return token{kind: tokenInteger, text: string(l.source[start:l.index]), line: line, col: col}, nil
		}
		if current == '"' {
			start := l.index
			l.advance(current)
			for l.index < len(l.source) {
				value := l.source[l.index]
				l.advance(value)
				if value == '\\' && l.index < len(l.source) {
					l.advance(l.source[l.index])
					continue
				}
				if value == '"' {
					text := string(l.source[start:l.index])
					decoded, err := strconv.Unquote(text)
					if err != nil {
						return token{}, l.errorf("invalid string literal: %v", err)
					}
					return token{kind: tokenString, text: decoded, line: line, col: col}, nil
				}
			}
			return token{}, l.errorf("unterminated string literal")
		}
		if strings.ContainsRune("{}():=,;", current) {
			l.advance(current)
			return token{kind: tokenSymbol, text: string(current), line: line, col: col}, nil
		}
		return token{}, l.errorf("unexpected character %q", current)
	}
	return token{kind: tokenEOF, line: l.line, col: l.col}, nil
}

func (l *lexer) peek() rune {
	if l.index+1 >= len(l.source) {
		return 0
	}
	return l.source[l.index+1]
}

func (l *lexer) advance(value rune) {
	l.index++
	if value == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

func (l *lexer) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d, column %d: %s", l.line, l.col, fmt.Sprintf(format, args...))
}

type parser struct {
	lexer   *lexer
	current token
	peeked  token
	loaded  bool
}

func newParser(source string) *parser {
	return &parser{lexer: newLexer(source)}
}

func (p *parser) advance() error {
	if p.loaded {
		p.current = p.peeked
		p.loaded = false
		return nil
	}
	next, err := p.lexer.next()
	if err != nil {
		return err
	}
	p.current = next
	return nil
}

func (p *parser) peek() (token, error) {
	if !p.loaded {
		next, err := p.lexer.next()
		if err != nil {
			return token{}, err
		}
		p.peeked = next
		p.loaded = true
	}
	return p.peeked, nil
}

func (p *parser) expect(kind tokenKind, text string) (token, error) {
	if err := p.advance(); err != nil {
		return token{}, err
	}
	if p.current.kind != kind || (text != "" && p.current.text != text) {
		return token{}, fmt.Errorf("line %d, column %d: expected %s %q, got %s %q", p.current.line, p.current.col, kind, text, p.current.kind, p.current.text)
	}
	return p.current, nil
}

func (p *parser) parseProgram() (Program, error) {
	if _, err := p.expect(tokenIdent, "program"); err != nil {
		return Program{}, err
	}
	name, err := p.expect(tokenIdent, "")
	if err != nil {
		return Program{}, err
	}
	if _, err := p.expect(tokenSymbol, "{"); err != nil {
		return Program{}, err
	}
	var body []Stmt
	for {
		next, err := p.peek()
		if err != nil {
			return Program{}, err
		}
		if next.kind == tokenSymbol && next.text == "}" {
			break
		}
		statement, err := p.parseStatement()
		if err != nil {
			return Program{}, err
		}
		body = append(body, statement)
	}
	if _, err := p.expect(tokenSymbol, "}"); err != nil {
		return Program{}, err
	}
	if next, err := p.peek(); err != nil {
		return Program{}, err
	} else if next.kind != tokenEOF {
		return Program{}, fmt.Errorf("line %d, column %d: trailing input after program", next.line, next.col)
	}
	if len(body) == 0 || (body[len(body)-1].Kind != "result" && (body[len(body)-1].Kind != "if" || body[len(body)-1].Then == nil || body[len(body)-1].Else == nil || body[len(body)-1].Then.Result == nil || body[len(body)-1].Else.Result == nil)) {
		return Program{}, fmt.Errorf("program %q must end with result", name.text)
	}
	return Program{Name: name.text, Body: body}, nil
}

func (p *parser) parseStatement() (Stmt, error) {
	keyword, err := p.expect(tokenIdent, "")
	if err != nil {
		return Stmt{}, err
	}
	switch keyword.text {
	case "let":
		name, err := p.expect(tokenIdent, "")
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, ":"); err != nil {
			return Stmt{}, err
		}
		declared, err := p.expect(tokenIdent, "")
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, "="); err != nil {
			return Stmt{}, err
		}
		expression, err := p.parseExpression()
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return Stmt{}, err
		}
		return Stmt{Kind: "let", Name: name.text, DeclType: declared.text, Expr: expression}, nil
	case "effect":
		effect, err := p.expect(tokenIdent, "")
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return Stmt{}, err
		}
		expression, err := p.parseExpression()
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return Stmt{}, err
		}
		return Stmt{Kind: "effect", Effect: effect.text, Expr: expression}, nil
	case "if":
		condition, err := p.parseExpression()
		if err != nil {
			return Stmt{}, err
		}
		thenBlock, err := p.parseBlock()
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenIdent, "else"); err != nil {
			return Stmt{}, err
		}
		elseBlock, err := p.parseBlock()
		if err != nil {
			return Stmt{}, err
		}
		return Stmt{Kind: "if", Expr: condition, Then: &thenBlock, Else: &elseBlock}, nil
	case "result":
		expression, err := p.parseExpression()
		if err != nil {
			return Stmt{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return Stmt{}, err
		}
		return Stmt{Kind: "result", Expr: expression}, nil
	default:
		return Stmt{}, fmt.Errorf("line %d, column %d: unknown statement %q", keyword.line, keyword.col, keyword.text)
	}
}

func (p *parser) parseBlock() (Block, error) {
	if _, err := p.expect(tokenSymbol, "{"); err != nil {
		return Block{}, err
	}
	var statements []Stmt
	for {
		next, err := p.peek()
		if err != nil {
			return Block{}, err
		}
		if next.kind == tokenSymbol && next.text == "}" {
			break
		}
		statement, err := p.parseStatement()
		if err != nil {
			return Block{}, err
		}
		statements = append(statements, statement)
	}
	if _, err := p.expect(tokenSymbol, "}"); err != nil {
		return Block{}, err
	}
	if len(statements) == 0 || statements[len(statements)-1].Kind != "result" {
		return Block{}, fmt.Errorf("branch must end with result")
	}
	result := statements[len(statements)-1].Expr
	return Block{Statements: statements[:len(statements)-1], Result: result}, nil
}

func (p *parser) parseExpression() (*Expr, error) {
	next, err := p.peek()
	if err != nil {
		return nil, err
	}
	switch next.kind {
	case tokenInteger:
		value, err := p.expect(tokenInteger, "")
		if err != nil {
			return nil, err
		}
		parsed, err := strconv.ParseInt(value.text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q", value.text)
		}
		return &Expr{Kind: "int", Int: parsed}, nil
	case tokenString:
		value, err := p.expect(tokenString, "")
		if err != nil {
			return nil, err
		}
		return &Expr{Kind: "string", Text: value.text}, nil
	case tokenIdent:
		value, err := p.expect(tokenIdent, "")
		if err != nil {
			return nil, err
		}
		if value.text == "true" || value.text == "false" {
			return &Expr{Kind: "bool", Bool: value.text == "true"}, nil
		}
		next, err := p.peek()
		if err != nil {
			return nil, err
		}
		if next.kind != tokenSymbol || next.text != "(" {
			return &Expr{Kind: "var", Name: value.text}, nil
		}
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return nil, err
		}
		var args []*Expr
		next, err = p.peek()
		if err != nil {
			return nil, err
		}
		if next.kind != tokenSymbol || next.text != ")" {
			for {
				argument, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				args = append(args, argument)
				next, err = p.peek()
				if err != nil {
					return nil, err
				}
				if next.kind == tokenSymbol && next.text == ")" {
					break
				}
				if _, err := p.expect(tokenSymbol, ","); err != nil {
					return nil, err
				}
			}
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return nil, err
		}
		return &Expr{Kind: "call", Name: value.text, Args: args}, nil
	default:
		return nil, fmt.Errorf("line %d, column %d: expected expression, got %s %q", next.line, next.col, next.kind, next.text)
	}
}
