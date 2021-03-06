package main

import "fmt"

// Recursive-descent parser
//
// program        -> declaration* EOF ;
//
// declaration    -> funDecl
//                 | lambdaCall
//                 | varDecl
//                 | statement ;
//
// funDecl        -> "fun" function ;
// function       -> IDENTIFIER "(" parameters? ")" block ;
// parameters     -> IDENTIFIER ( "," IDENTIFIER )* ;
//
// lambdaCall     -> funExpr "(" arguments? ")" ";" ;
//
// varDecl        -> "var" IDENTIFIER ( "=" expression )? ";" ;
//
// statement      -> exprStmt
//                 | breakStmt
//                 | continueStmt
//                 | forStmt
//                 | ifStmt
//                 | printStmt
//                 | returnStmt
//                 | whileStmt
//				   | block ;
//
// block		  -> "{" declaration* "}" ;
// breakStmt      -> "break" ";" ;
// continueStmt   -> "continue" ";" ;
// exprStmt       -> expression ";" ;
// forStmt        -> "for" "(" ( varDecl | exprStmt | ";" )
//                   expression? ";"
//                   expression? ")" statement ;
// ifStmt         -> "if" "(" expression ")" statement ( "else" statement )? ;
// printStmt      -> "print" expression ";" ;
// returnStmt     -> "return" expression? ";" ;
// whileStmt      -> "while" "(" expression ")" statement ;
//
// expression     -> funExpr
//                 | assignment ;
// funExpr        -> "fun" "(" parameters? ")" block ;
// assignment     -> IDENTIFIER "=" assignment
//				   | logicOr ;
// logicOr        -> logicAnd ( "or" logicAnd )* ;
// logicAnd       -> equality ( "and" equality )* ;
// equality       -> comparison ( ( "!=" | "==" ) comparison )* ;
// comparison     -> term ( ( ">" | ">=" | "<" | "<=" ) term )* ;
// term           -> factor ( ( "-" | "+" ) factor )* ;
// factor         -> unary ( ( "/" | "*" ) unary )* ;
// unary          -> ( "!" | "-" ) unary | call ;
// call			  -> primary ( "(" arguments? ")" )* ;
// arguments      -> expression ( "," expression )* ;
// primary        -> NUMBER | STRING | "true" | "false" | "nil"
//                 | "(" expression ")"
//                 | IDENTIFIER ;
//

type parser struct {
	tokens  []*tokenObj
	current int
	errs    []error
	inLoop  int
}

func NewParser(tokens []*tokenObj) *parser {
	p := &parser{tokens, 0, make([]error, 0), 0}
	return p
}

// match advances pointer to the next token if current token matches
// any of toks and returns true
func (p *parser) match(toks ...token) bool {
	for _, t := range toks {
		if p.check(t) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *parser) advance() *tokenObj {
	if !p.atEnd() {
		p.current++
	}
	return p.prev()
}

func (p *parser) atEnd() bool {
	return p.peek().tok == EOF
}

func (p *parser) peek() *tokenObj {
	return p.tokens[p.current]
}

func (p *parser) prev() *tokenObj {
	return p.tokens[p.current-1]
}

func (p *parser) check(tok token) bool {
	if p.atEnd() {
		return false
	}
	// fmt.Printf("p.peek() = %+v\n", p.peek())
	return p.peek().tok == tok
}

func (p *parser) consume(expected token, msg string) *tokenObj {
	if p.check(expected) {
		return p.advance()
	}
	p.perror(p.peek(), msg)
	return nil
}

type ParsingError string

func (e ParsingError) Error() string {
	return string(e)
}

func (p *parser) perror(t *tokenObj, msg string) {
	e := ParsingError(errorAtToken(t, msg))
	p.errs = append(p.errs, e)
	panic(e)
}

func (p *parser) yerror(t *tokenObj, msg string) {
	e := ParsingError(errorAtToken(t, msg))
	p.errs = append(p.errs, e)
}

func (p *parser) sync() {
	fmt.Println("sync")
	p.advance()
	for !p.atEnd() {
		if p.prev().tok == Semicolon {
			return
		}
		switch p.peek().tok {
		case Class, Fun, Var, For, If, While, Print, Return:
			return
		}
		p.advance()
	}
}

// ---------------------------------------------------------
//

// parse returns an AST of parsed tokens, if it cannot parse then it returns
// the error.
func (p *parser) parse() (s []Stmt, errs []error) {
	s = make([]Stmt, 0)
	for !p.atEnd() {
		s = append(s, p.declaration())
	}

	return s, p.errs
}

func (p *parser) declaration() (s Stmt) {
	defer func() {
		if e := recover(); e != nil {
			_ = e.(ParsingError) // Panic for other errors
			p.sync()
			s = nil
		}
	}()
	if p.match(Fun) {
		if p.check(LeftParen) {
			return p.lambdaCall()
		}
		return p.funDecl("function")
	}
	if p.match(Var) {
		return p.varDecl()
	}
	return p.statement()
}

func (p *parser) funDecl(kind string) Stmt {
	name := p.consume(Identifier, "expected "+kind+" name")
	p.consume(LeftParen, "expected '(' after "+kind+" name")
	params := make([]*tokenObj, 0)
	if !p.check(RightParen) {
		for {
			if len(params) >= 255 {
				p.yerror(p.peek(), "can't have more than 255 parameters")
			}
			params = append(params, p.consume(Identifier, "expected parameter name"))
			if !p.match(Comma) {
				break
			}
		}
	}
	p.consume(RightParen, "expected ')' after parameters")
	p.consume(LeftBrace, "expected '{' after "+kind+" signature")
	body := p.block()
	return &FunStmt{name: name, params: params, body: body}
}

func (p *parser) varDecl() Stmt {
	name := p.consume(Identifier, "expected variable name")
	var init Expr

	if p.match(Equal) {
		init = p.expression()
	}
	p.consume(Semicolon, "expected ';' after variable declaration")
	return &VarStmt{name: name, init: init}
}

func (p *parser) statement() Stmt {
	if p.match(Break) {
		return p.breakStatement()
	}
	if p.match(Continue) {
		return p.continueStatement()
	}
	if p.match(For) {
		return p.forStatement()
	}
	if p.match(If) {
		return p.ifStatement()
	}
	if p.match(Print) {
		return p.printStatement()
	}
	if p.match(Return) {
		return p.returnStatement()
	}
	if p.match(While) {
		return p.whileStatement()
	}
	if p.match(LeftBrace) {
		return &BlockStmt{list: p.block()}
	}
	return p.exprStatement()
}

func (p *parser) breakStatement() Stmt {
	key := p.prev()
	if p.inLoop < 1 {
		p.perror(key, "expected inside the loop")
	}
	p.consume(Semicolon, "expected ';' after break")
	return &BreakStmt{keyword: key}
}

func (p *parser) continueStatement() Stmt {
	key := p.prev()
	if p.inLoop < 1 {
		p.perror(key, "expected inside the loop")
	}
	p.consume(Semicolon, "expected ';' after continue")
	return &ContinueStmt{keyword: key}
}

func (p *parser) forStatement() Stmt {
	p.consume(LeftParen, "expected '(' after 'for'")

	var initial Stmt
	switch {
	case p.match(Semicolon):
		initial = nil
	case p.match(Var):
		initial = p.varDecl()
	default:
		initial = p.exprStatement()
	}

	var cond Expr
	if !p.check(Semicolon) {
		cond = p.expression()
	}
	p.consume(Semicolon, "expected ';' after for condition")

	var incr Expr
	if !p.check(RightParen) {
		incr = p.expression()
	}
	p.consume(RightParen, "expected ')' after for clauses")

	p.inLoop += 1
	body := p.statement()
	p.inLoop -= 1

	if incr != nil {
		body = &BlockStmt{list: []Stmt{
			body,
			&ExprStmt{expression: incr}}}
	}
	if cond != nil {
		body = &WhileStmt{condition: cond, body: body}
	}
	if initial != nil {
		body = &BlockStmt{list: []Stmt{
			initial,
			body}}
	}
	return body
}

func (p *parser) ifStatement() Stmt {
	p.consume(LeftParen, "expected '(' after 'if'")
	e := p.expression()
	p.consume(RightParen, "expected ')' after if condition")
	a := p.statement()
	var b Stmt = nil
	if p.match(Else) {
		b = p.statement()
	}
	return &IfStmt{condition: e, block1: a, block2: b}
}

func (p *parser) printStatement() Stmt {
	e := p.expression()
	p.consume(Semicolon, "expected ';' after expression")
	return &PrintStmt{expression: e}
}

func (p *parser) returnStatement() Stmt {
	k := p.prev()
	var val Expr
	if !p.check(Semicolon) {
		val = p.expression()
	}
	p.consume(Semicolon, "expected ';' after return value")
	return &ReturnStmt{keyword: k, value: val}
}

func (p *parser) whileStatement() Stmt {
	p.consume(LeftParen, "expected '(' after while")
	expr := p.expression()
	p.consume(RightParen, "expected ')' after while condition")
	p.inLoop += 1
	body := p.statement()
	p.inLoop -= 1
	return &WhileStmt{condition: expr, body: body}
}

func (p *parser) block() []Stmt {
	list := make([]Stmt, 0)
	for !p.check(RightBrace) && !p.atEnd() {
		list = append(list, p.declaration())
	}
	p.consume(RightBrace, "expected '}' after block")
	return list
}

func (p *parser) exprStatement() Stmt {
	e := p.expression()
	p.consume(Semicolon, "expected ';' after expression")
	return &ExprStmt{expression: e}
}

func (p *parser) expression() Expr {
	if p.match(Fun) {
		return p.funExpr()
	}
	return p.assignment()
}

func (p *parser) funExpr() Expr {
	p.consume(LeftParen, "expected '(' after 'fun'")
	params := make([]*tokenObj, 0)
	if !p.check(RightParen) {
		for {
			if len(params) >= 255 {
				p.yerror(p.peek(), "can't have more than 255 parameters")
			}
			params = append(params, p.consume(Identifier, "expected parameter name"))
			if !p.match(Comma) {
				break
			}
		}
	}
	p.consume(RightParen, "expected ')' after parameters")
	p.consume(LeftBrace, "expected '{' after anonymous function signature")
	body := p.block()
	return &FunExpr{params: params, body: body}
}

func (p *parser) lambdaCall() Stmt {
	expr := p.funExpr()
	for {
		if p.match(LeftParen) {
			expr = p.finishCall(expr)
		} else {
			break
		}
	}
	p.consume(Semicolon, "expected ';' call to a function")
	return &ExprStmt{expression: expr}
}

func (p *parser) assignment() Expr {
	expr := p.or()
	if p.match(Equal) {
		equals := p.prev()
		value := p.assignment()
		if ev, ok := expr.(*VarExpr); ok {
			name := ev.name
			return &AssignExpr{name: name, value: value}
		}
		p.yerror(equals, "invalid assignment target")
	}
	return expr
}

func (p *parser) or() Expr {
	expr := p.and()
	for p.match(Or) {
		op := p.prev()
		right := p.and()
		expr = &LogicalExpr{operator: op, left: expr, right: right}
	}
	return expr
}

func (p *parser) and() Expr {
	expr := p.equality()
	for p.match(And) {
		op := p.prev()
		right := p.equality()
		expr = &LogicalExpr{operator: op, left: expr, right: right}
	}
	return expr
}

// equality -> comparison ( ( "!=" | "==" ) comparison )* ;
func (p *parser) equality() Expr {
	expr := p.comparison()
	for p.match(BangEqual, EqualEqual) {
		op := p.prev()
		right := p.comparison()
		expr = &BinaryExpr{operator: op, left: expr, right: right}
	}
	return expr
}

// comparison -> term ( ( ">" | ">=" | "<" | "<=" ) term )* ;
func (p *parser) comparison() Expr {
	expr := p.term()
	for p.match(Greater, GreaterEqual, Less, LessEqual) {
		op := p.prev()
		right := p.term()
		expr = &BinaryExpr{operator: op, left: expr, right: right}
	}
	return expr
}

// term ->  factor ( ( "-" | "+" ) factor )* ;
func (p *parser) term() Expr {
	expr := p.factor()
	for p.match(Plus, Minus) {
		op := p.prev()
		right := p.factor()
		expr = &BinaryExpr{operator: op, left: expr, right: right}
	}
	return expr
}

// factor -> unary ( ( "/" | "*" ) unary )* ;
func (p *parser) factor() Expr {
	expr := p.unary()
	for p.match(Slash, Star) {
		op := p.prev()
		right := p.unary()
		expr = &BinaryExpr{operator: op, left: expr, right: right}
	}
	return expr
}

// unary -> ( "!" | "-" ) unary
//        | primary ;
func (p *parser) unary() Expr {
	if p.match(Bang, Minus) {
		op := p.prev()
		right := p.unary()
		return &UnaryExpr{operator: op, right: right}
	}
	return p.call()
}

func (p *parser) call() Expr {
	expr := p.primary()
	for {
		if p.match(LeftParen) {
			expr = p.finishCall(expr)
		} else {
			break
		}
	}
	return expr
}

func (p *parser) finishCall(expr Expr) Expr {
	args := make([]Expr, 0)
	if !p.check(RightParen) {
		for {
			if len(args) >= 255 {
				p.yerror(p.peek(), "can't have more than 255 arguments")
			}
			args = append(args, p.expression())
			if !p.match(Comma) {
				break
			}
		}
	}
	paren := p.consume(RightParen, "expected ')' after arguments")
	return &CallExpr{callee: expr, paren: paren, args: args}
}

// primary -> NUMBER | STRING | "true" | "false" | "nil"
//          | "(" expression ")" ;
func (p *parser) primary() Expr {
	switch {
	case p.match(False):
		return &LiteralExpr{value: false}
	case p.match(True):
		return &LiteralExpr{value: true}
	case p.match(Nil):
		return &LiteralExpr{value: nil}
	case p.match(Number, String):
		return &LiteralExpr{value: p.prev().literal}
	case p.match(Identifier):
		return &VarExpr{name: p.prev()}
	case p.match(LeftParen):
		expr := p.expression()
		p.consume(RightParen, "expected enclosing ')' after expression")
		return &GroupingExpr{e: expr}
	}
	p.perror(p.peek(), "expected expression")
	return nil
}
