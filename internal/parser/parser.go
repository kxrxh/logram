package parser

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"sync"
	"time"
)

var (
	timestampFormats = []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelError LogLevel = "ERROR"
)

type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   []byte
	RuleName  string
	Raw       []byte
}

type Rule struct {
	Name  string
	Regex *regexp.Regexp
}

type RuleConfig struct {
	Name    string
	Pattern string
}

type Parser struct {
	mu    sync.RWMutex
	rules []Rule
}

func NewParser(rules []Rule) *Parser {
	return &Parser{
		rules: rules,
	}
}

func NewParserFromConfig(rules []RuleConfig) (*Parser, error) {
	var parsedRules []Rule
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, &RuleError{
				Rule:   r.Name,
				Reason: err,
			}
		}
		parsedRules = append(parsedRules, Rule{
			Name:  r.Name,
			Regex: re,
		})
	}
	return NewParser(parsedRules), nil
}

func (p *Parser) UpdateRules(rules []Rule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = rules
}

func (p *Parser) Rules() []Rule {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rules
}

func (p *Parser) Start(ctx context.Context, input <-chan []byte) <-chan LogEntry {
	output := make(chan LogEntry, 100)

	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-input:
				if !ok {
					return
				}
				entry, err := p.ParseLine(line)
				if err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case output <- entry:
				}
			}
		}
	}()

	return output
}

func (p *Parser) ParseLogStream(ch <-chan []byte) ([]LogEntry, []ParseError) {
	entries := make([]LogEntry, 0, 100)
	var errs []ParseError

	for line := range ch {
		entry, err := p.ParseLine(line)
		if err != nil {
			if errors.Is(err.Reason, ErrNoMatchRules) {
				continue
			}
			errs = append(errs, *err)
			continue
		}
		entries = append(entries, entry)
	}

	return entries, errs
}

func (p *Parser) ParseLine(line []byte) (LogEntry, *ParseError) {
	line = cleanLine(line)

	if len(line) == 0 {
		return LogEntry{}, &ParseError{Line: "", Reason: ErrEmptyLine}
	}

	p.mu.RLock()
	rules := p.rules
	p.mu.RUnlock()

	if len(rules) == 0 {
		return parseDefault(line)
	}

	for _, rule := range rules {
		if rule.Regex.Match(line) {
			entry, err := parseDefault(line)
			if err != nil {
				return LogEntry{}, &ParseError{Line: string(line), Reason: err.Reason}
			}
			entry.RuleName = rule.Name
			return entry, nil
		}
	}

	return LogEntry{}, &ParseError{Line: string(line), Reason: ErrNoMatchRules}
}

func cleanLine(s []byte) []byte {
	if !bytes.Contains(s, []byte("\x1b")) {
		return s
	}
	return ansiRegex.ReplaceAll(s, []byte(""))
}

func parseLevelBytes(s []byte) LogLevel {
	if bytes.Equal(s, []byte("DEBUG")) {
		return LevelDebug
	}
	if bytes.Equal(s, []byte("ERROR")) {
		return LevelError
	}
	return LevelInfo
}

func parseDefault(line []byte) (LogEntry, *ParseError) {
	tsStr, rest, found := bytes.Cut(line, []byte(" "))
	if !found {
		return LogEntry{}, &ParseError{Line: string(line), Reason: ErrInvalidFormat}
	}

	if len(rest) < 3 || rest[0] != '[' {
		return LogEntry{}, &ParseError{Line: string(line), Reason: ErrInvalidFormat}
	}

	closeBracket := bytes.IndexByte(rest, ']')
	if closeBracket == -1 {
		return LogEntry{}, &ParseError{Line: string(line), Reason: ErrInvalidFormat}
	}
	levelStr := rest[1:closeBracket]

	var msg []byte
	if len(rest) > closeBracket+2 {
		msg = rest[closeBracket+2:]
	}

	timestamp, err := parseTimestamp(string(tsStr))
	if err != nil {
		return LogEntry{}, &ParseError{Line: string(line), Reason: err}
	}

	return LogEntry{
		Timestamp: timestamp,
		Level:     parseLevelBytes(levelStr),
		Message:   msg,
		Raw:       line,
	}, nil
}

func parseTimestamp(value string) (time.Time, error) {
	switch len(value) {
	case 10:
		return time.Parse("2006-01-02", value)
	case 19:
		return time.Parse("2006-01-02 15:04:05", value)
	case 20, 24, 25:
		if ts, err := time.Parse(time.RFC3339, value); err == nil {
			return ts, nil
		}
	}

	for _, format := range timestampFormats {
		if ts, err := time.Parse(format, value); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, ErrInvalidFormat
}

func parseLevel(s string) LogLevel {
	switch s {
	case "DEBUG":
		return LevelDebug
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}
