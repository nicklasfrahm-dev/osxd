// Package calc evaluates the arithmetic expressions typed into the launcher.
package calc

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

var constants = map[string]float64{
	"pi": math.Pi,
	"π":  math.Pi,
	"e":  math.E,
}

var functions = map[string]func(float64) float64{
	"sqrt":  math.Sqrt,
	"abs":   math.Abs,
	"ln":    math.Log,
	"log":   math.Log10,
	"sin":   math.Sin,
	"cos":   math.Cos,
	"tan":   math.Tan,
	"floor": math.Floor,
	"ceil":  math.Ceil,
	"round": math.Round,
}

// Evaluate returns the formatted result of query if it is a calculation.
// Bare numbers and constants are not calculations, so typing "42" or "e"
// keeps searching apps without showing a result.
func Evaluate(query string) (string, bool) {
	v, ops, err := eval(query)
	if err != nil || ops == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return "", false
	}
	return Format(v), true
}

// Eval parses and evaluates expr. It supports + - * / % ^ (also × ÷ **),
// parentheses, the constants pi and e, and the functions sqrt, abs, ln, log,
// sin, cos, tan, floor, ceil and round.
func Eval(expr string) (float64, error) {
	v, _, err := eval(expr)
	return v, err
}

func eval(expr string) (float64, int, error) {
	p := &parser{s: []rune(strings.TrimPrefix(strings.TrimSpace(expr), "="))}
	v, err := p.expr()
	if err != nil {
		return 0, 0, err
	}
	p.space()
	if p.i < len(p.s) {
		return 0, 0, fmt.Errorf("unexpected %q", p.s[p.i])
	}
	return v, p.ops, nil
}

// Format renders v with at most 12 significant digits, which hides float
// noise such as 0.1+0.2 = 0.30000000000000004.
func Format(v float64) string {
	if v == 0 {
		return "0" // also normalises -0
	}
	return strconv.FormatFloat(v, 'g', 12, 64)
}

// parser is a recursive-descent parser over the grammar
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/" | "%") unary }
//	unary   = ("-" | "+") unary | power
//	power   = primary [ "^" unary ]
//	primary = number | constant | function "(" expr ")" | "(" expr ")"
type parser struct {
	s   []rune
	i   int
	ops int // operators and function calls seen
}

func (p *parser) space() {
	for p.i < len(p.s) && unicode.IsSpace(p.s[p.i]) {
		p.i++
	}
}

// accept consumes tok if it comes next.
func (p *parser) accept(tok string) bool {
	p.space()
	r := []rune(tok)
	if p.i+len(r) > len(p.s) || string(p.s[p.i:p.i+len(r)]) != tok {
		return false
	}
	p.i += len(r)
	return true
}

func (p *parser) expr() (float64, error) {
	v, err := p.term()
	for err == nil {
		switch {
		case p.accept("+"):
			var r float64
			r, err = p.term()
			v += r
		case p.accept("-"), p.accept("−"):
			var r float64
			r, err = p.term()
			v -= r
		default:
			return v, nil
		}
		p.ops++
	}
	return 0, err
}

func (p *parser) term() (float64, error) {
	v, err := p.unary()
	for err == nil {
		var op rune
		switch {
		case p.accept("*"), p.accept("×"):
			op = '*'
		case p.accept("/"), p.accept("÷"):
			op = '/'
		case p.accept("%"):
			op = '%'
		default:
			return v, nil
		}
		var r float64
		if r, err = p.unary(); err != nil {
			break
		}
		p.ops++
		switch op {
		case '*':
			v *= r
		case '/':
			if r == 0 {
				return 0, errors.New("division by zero")
			}
			v /= r
		case '%':
			if r == 0 {
				return 0, errors.New("division by zero")
			}
			v = math.Mod(v, r)
		}
	}
	return 0, err
}

func (p *parser) unary() (float64, error) {
	switch {
	case p.accept("-"), p.accept("−"):
		v, err := p.unary()
		return -v, err
	case p.accept("+"):
		return p.unary()
	}
	return p.power()
}

func (p *parser) power() (float64, error) {
	base, err := p.primary()
	if err != nil {
		return 0, err
	}
	if p.accept("^") || p.accept("**") {
		exp, err := p.unary() // right-associative: 2^3^2 = 2^9
		if err != nil {
			return 0, err
		}
		p.ops++
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) primary() (float64, error) {
	p.space()
	if p.i >= len(p.s) {
		return 0, errors.New("unexpected end of expression")
	}
	if p.accept("(") {
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if !p.accept(")") {
			return 0, errors.New("missing )")
		}
		return v, nil
	}
	c := p.s[p.i]
	if unicode.IsDigit(c) || c == '.' {
		return p.number()
	}
	if unicode.IsLetter(c) {
		name := p.ident()
		if f, ok := functions[name]; ok {
			if !p.accept("(") {
				return 0, fmt.Errorf("%s needs (", name)
			}
			arg, err := p.expr()
			if err != nil {
				return 0, err
			}
			if !p.accept(")") {
				return 0, errors.New("missing )")
			}
			p.ops++
			return f(arg), nil
		}
		if v, ok := constants[name]; ok {
			return v, nil
		}
		return 0, fmt.Errorf("unknown name %q", name)
	}
	return 0, fmt.Errorf("unexpected %q", c)
}

func (p *parser) number() (float64, error) {
	start := p.i
	for p.i < len(p.s) && (unicode.IsDigit(p.s[p.i]) || p.s[p.i] == '.') {
		p.i++
	}
	// Scientific notation, e.g. 1.5e3, but not a trailing constant as in 2e.
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i + 1
		if j < len(p.s) && (p.s[j] == '+' || p.s[j] == '-') {
			j++
		}
		if j < len(p.s) && unicode.IsDigit(p.s[j]) {
			for j < len(p.s) && unicode.IsDigit(p.s[j]) {
				j++
			}
			p.i = j
		}
	}
	return strconv.ParseFloat(string(p.s[start:p.i]), 64)
}

func (p *parser) ident() string {
	start := p.i
	for p.i < len(p.s) && unicode.IsLetter(p.s[p.i]) {
		p.i++
	}
	return strings.ToLower(string(p.s[start:p.i]))
}
