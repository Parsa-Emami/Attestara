// Package policy assesses declared licensing facts without inferring compliance.
package policy

import (
	"strings"
	"unicode/utf8"

	"attestara/internal/contracts"
	"attestara/internal/licensecatalog"
)

// LicenseAssessment recognizes syntax/identifiers, never permission to release.
// Any unresolved declaration or reference requires review, even inside an OR.
type LicenseAssessment struct {
	Expression string `json:"expression"`
	Status     string `json:"status"`
}

type licenseToken struct {
	text       string
	start, end int
}
type licenseParser struct {
	tokens []licenseToken
	pos    int
	review bool
	err    error
}

func licenseFailure(code string) error {
	message := "Expected a well-formed SPDX license expression."
	if code == "limit_exceeded" {
		message = "License expression exceeds 4096 characters."
	}
	if code == "dependency_unavailable" {
		message = "Pinned SPDX identifier catalog is unavailable."
	}
	return &contracts.Error{Code: code, Field: "expression", Message: message}
}

func idChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.'
}
func idString(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !idChar(s[i]) {
			return false
		}
	}
	return true
}
func customRef(s, prefix string) bool {
	if strings.HasPrefix(s, "DocumentRef-") {
		doc, rest, ok := strings.Cut(s[len("DocumentRef-"):], ":")
		if !ok || !idString(doc) {
			return false
		}
		s = rest
	}
	return strings.HasPrefix(s, prefix) && idString(s[len(prefix):])
}
func (p *licenseParser) peek() licenseToken {
	if p.pos == len(p.tokens) {
		return licenseToken{}
	}
	return p.tokens[p.pos]
}
func operator(s, upper string) bool { return s == upper || s == strings.ToLower(upper) }
func (p *licenseParser) identifier(exception bool) bool {
	token := p.peek()
	if token.text == "" || operator(token.text, "AND") || operator(token.text, "OR") || operator(token.text, "WITH") {
		return false
	}
	p.pos++
	prefix := "LicenseRef-"
	want := "license"
	if exception {
		prefix = "AdditionRef-"
		want = "exception"
	}
	if customRef(token.text, prefix) {
		p.review = true
		return true
	}
	// Reserved references must be well formed and in the correct operand role.
	if !idString(token.text) || strings.HasPrefix(token.text, "LicenseRef-") || strings.HasPrefix(token.text, "AdditionRef-") || strings.HasPrefix(token.text, "DocumentRef-") {
		return false
	}
	kind, err := licensecatalog.Kind(token.text)
	if err != nil {
		p.err = licenseFailure("dependency_unavailable")
		return false
	}
	if kind == "" {
		p.review = true
	} else if kind != want {
		return false
	}
	if p.peek().text == "+" {
		if exception || token.end != p.peek().start {
			return false
		}
		p.pos++
	}
	return true
}
func (p *licenseParser) primary() bool {
	if p.peek().text == "(" {
		p.pos++
		if !p.expression() || p.peek().text != ")" {
			return false
		}
		p.pos++
		return true
	}
	if !p.identifier(false) {
		return false
	}
	if operator(p.peek().text, "WITH") {
		with := p.peek()
		if p.tokens[p.pos-1].end == with.start {
			return false
		}
		p.pos++
		if with.end == p.peek().start || !p.identifier(true) {
			return false
		}
	}
	return true
}
func (p *licenseParser) expression() bool {
	if !p.primary() {
		return false
	}
	// Recognition visits every operand; AND/OR precedence cannot change whether
	// any term is unresolved. No policy decision or expression rewrite is made.
	for operator(p.peek().text, "AND") || operator(p.peek().text, "OR") {
		op := p.peek()
		previous := p.tokens[p.pos-1]
		if previous.end == op.start && previous.text != ")" {
			return false
		}
		p.pos++
		if next := p.peek(); next.start == op.end && next.text != "(" {
			return false
		}
		if !p.primary() {
			return false
		}
	}
	return true
}

// AssessLicenseExpression recognizes SPDX 3.0.1 expression syntax against the
// pinned SPDX List 3.29.0 catalog. OR is a valid choice, not ambiguity by itself.
// Source: https://spdx.github.io/spdx-spec/v3.0.1/annexes/spdx-license-expressions/
// The C04 text limit bounds parsing, nesting, and allocation. No I/O or execution.
func AssessLicenseExpression(expression string) (LicenseAssessment, error) {
	if len(expression) > 4*4096 || utf8.RuneCountInString(expression) > 4096 {
		return LicenseAssessment{}, licenseFailure("limit_exceeded")
	}
	if !utf8.ValidString(expression) {
		return LicenseAssessment{}, licenseFailure("invalid_input")
	}
	result := LicenseAssessment{Expression: expression, Status: "review"}
	if strings.Trim(expression, " \t") == "" {
		return result, nil
	}
	p := licenseParser{}
	for i := 0; i < len(expression); {
		if expression[i] == ' ' || expression[i] == '\t' {
			i++
			continue
		}
		start := i
		if strings.ContainsRune("()+", rune(expression[i])) {
			i++
		} else {
			for i < len(expression) && (idChar(expression[i]) || expression[i] == ':') {
				i++
			}
			if i == start {
				return LicenseAssessment{}, licenseFailure("invalid_input")
			}
		}
		p.tokens = append(p.tokens, licenseToken{expression[start:i], start, i})
	}
	if !p.expression() || p.pos != len(p.tokens) {
		if p.err != nil {
			return LicenseAssessment{}, p.err
		}
		return LicenseAssessment{}, licenseFailure("invalid_input")
	}
	if !p.review {
		result.Status = "recognized"
	}
	return result, nil
}
