package parser

import "errors"

var (
	ErrInvalidFormat = errors.New("invalid log format")
	ErrInvalidRegex  = errors.New("invalid regex pattern")
	ErrEmptyLine     = errors.New("empty log line")
	ErrNoMatchRules  = errors.New("no matching rules")
)

type ParseError struct {
	Line   string
	Reason error
}

// Error returns a string representation of the ParseError.
func (e *ParseError) Error() string {
	if e == nil || e.Reason == nil {
		return ""
	}
	return e.Reason.Error()
}

// Unwrap allows ParseError to work with errors.Is() and errors.As()
func (e *ParseError) Unwrap() error {
	return e.Reason
}

type RuleError struct {
	Rule   string
	Reason error
}

func (e *RuleError) Error() string {
	if e == nil || e.Reason == nil {
		return ""
	}
	return "rule [" + e.Rule + "] error: " + e.Reason.Error()
}

// Unwrap allows RuleError to work with errors.Is() and errors.As()
func (e *RuleError) Unwrap() error {
	return e.Reason
}
