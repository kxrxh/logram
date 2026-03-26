package parser

import (
	"bytes"
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestParseLine_Default(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    LogEntry
		wantErr bool
		errType error
	}{
		{
			name: "valid RFC3339 format",
			line: "2024-01-15T10:30:00Z [INFO] Application started",
			want: LogEntry{
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Level:     LevelInfo,
				Message:   []byte("Application started"),
				Raw:       []byte("2024-01-15T10:30:00Z [INFO] Application started"),
			},
			wantErr: false,
		},
		{
			name: "valid date only",
			line: "2024-01-15 [INFO] Date only log",
			want: LogEntry{
				Timestamp: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Level:     LevelInfo,
				Message:   []byte("Date only log"),
				Raw:       []byte("2024-01-15 [INFO] Date only log"),
			},
			wantErr: false,
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
			errType: ErrEmptyLine,
		},
		{
			name:    "invalid format missing level",
			line:    "2024-01-15 some message",
			wantErr: true,
			errType: ErrInvalidFormat,
		},
		{
			name:    "invalid format no space",
			line:    "2024-01-15[INFO] message",
			wantErr: true,
			errType: ErrInvalidFormat,
		},
		{
			name:    "invalid format no closing bracket",
			line:    "2024-01-15 [INFO message",
			wantErr: true,
			errType: ErrInvalidFormat,
		},
		{
			name: "unknown level defaults to INFO",
			line: "2024-01-15 [UNKNOWN] some message",
			want: LogEntry{
				Timestamp: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Level:     LevelInfo,
				Message:   []byte("some message"),
				Raw:       []byte("2024-01-15 [UNKNOWN] some message"),
			},
			wantErr: false,
		},
		{
			name: "error level",
			line: "2024-01-15T10:30:00Z [ERROR] Error message",
			want: LogEntry{
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Level:     LevelError,
				Message:   []byte("Error message"),
				Raw:       []byte("2024-01-15T10:30:00Z [ERROR] Error message"),
			},
			wantErr: false,
		},
		{
			name: "debug level",
			line: "2024-01-15T10:30:00Z [DEBUG] Debug message",
			want: LogEntry{
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Level:     LevelDebug,
				Message:   []byte("Debug message"),
				Raw:       []byte("2024-01-15T10:30:00Z [DEBUG] Debug message"),
			},
			wantErr: false,
		},
	}

	p := NewParser(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ParseLine([]byte(tt.line))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil {
				if !errorsIs(err.Reason, tt.errType) {
					t.Errorf("ParseLine() error = %v, want error type %v", err.Reason, tt.errType)
				}
				return
			}
			if !tt.wantErr {
				if !tt.want.Timestamp.Equal(got.Timestamp) {
					t.Errorf(
						"ParseLine() timestamp = %v, want %v",
						got.Timestamp,
						tt.want.Timestamp,
					)
				}
				if got.Level != tt.want.Level {
					t.Errorf("ParseLine() level = %v, want %v", got.Level, tt.want.Level)
				}
				if !bytes.Equal(got.Message, tt.want.Message) {
					t.Errorf("ParseLine() message = %v, want %v", got.Message, tt.want.Message)
				}
				if !bytes.Equal(got.Raw, tt.want.Raw) {
					t.Errorf("ParseLine() raw = %v, want %v", got.Raw, tt.want.Raw)
				}
			}
		})
	}
}

func TestParseLine_WithFilters(t *testing.T) {
	tests := []struct {
		name    string
		rules   []RuleConfig
		line    string
		want    LogEntry
		wantErr bool
		errType error
	}{
		{
			name: "invalid regex pattern",
			rules: []RuleConfig{
				{
					Name:    "invalid",
					Pattern: `(?P<invalid`,
				},
			},
			line:    "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, cfgErr := NewParserFromConfig(tt.rules)
			if cfgErr != nil && tt.wantErr && tt.errType == nil {
				return
			}
			if cfgErr != nil && tt.wantErr {
				if tt.errType != nil && !errorsIs(cfgErr, tt.errType) {
					t.Errorf(
						"NewParserFromConfig() error = %v, want error type %v",
						cfgErr,
						tt.errType,
					)
				}
				return
			}
			if cfgErr != nil {
				t.Fatalf("NewParserFromConfig() unexpected error: %v", cfgErr)
			}

			got, parseErr := p.ParseLine([]byte(tt.line))
			if (parseErr != nil) != tt.wantErr {
				t.Errorf("ParseLine() error = %v, wantErr %v", parseErr, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && parseErr != nil {
				if !errorsIs(parseErr.Reason, tt.errType) {
					t.Errorf(
						"ParseLine() error = %v, want error type %v",
						parseErr.Reason,
						tt.errType,
					)
				}
				return
			}
			if !tt.wantErr {
				if got.Timestamp != tt.want.Timestamp || got.Level != tt.want.Level ||
					!bytes.Equal(
						got.Message,
						tt.want.Message,
					) || !bytes.Equal(got.Raw, tt.want.Raw) || got.RuleName != tt.want.RuleName {
					t.Errorf("ParseLine() = %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

func TestParseLine_WithANSI(t *testing.T) {
	p := NewParser(nil)
	line := []byte("\x1b[32m2024-01-15T10:30:00Z [INFO]\x1b[0m Green text message")

	got, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine() unexpected error: %v", err)
	}

	if got.Level != LevelInfo || !bytes.Equal(got.Message, []byte("Green text message")) {
		t.Errorf("ParseLine() = %+v", got)
	}
}

func TestParseLine_DefaultParser(t *testing.T) {
	line := "2024-01-15T10:30:00Z [INFO] Using default parser"

	p := NewParser(nil)
	got, err := p.ParseLine([]byte(line))
	if err != nil {
		t.Fatalf("ParseLine() unexpected error: %v", err)
	}

	if got.Level != LevelInfo || !bytes.Equal(got.Message, []byte("Using default parser")) {
		t.Errorf("ParseLine() = %+v", got)
	}
}

func TestParseLogStream(t *testing.T) {
	p := NewParser(nil)
	ch := make(chan []byte, 3)
	go func() {
		ch <- []byte("2024-01-15T10:30:00Z [INFO] Message 1")
		ch <- []byte("2024-01-15T10:31:00Z [ERROR] Message 2")
		ch <- []byte("2024-01-15T10:32:00Z [DEBUG] Message 3")
		close(ch)
	}()

	entries, errs := p.ParseLogStream(ch)

	if len(errs) != 0 {
		t.Errorf("ParseLogStream() unexpected errors: %v", errs)
	}
	if len(entries) != 3 {
		t.Errorf("ParseLogStream() got %d entries, want 3", len(entries))
	}
	if len(entries) > 0 {
		if !bytes.Equal(entries[0].Message, []byte("Message 1")) ||
			!bytes.Equal(entries[1].Message, []byte("Message 2")) ||
			!bytes.Equal(entries[2].Message, []byte("Message 3")) {
			t.Errorf("ParseLogStream() entries = %v", entries)
		}
	}
}

func TestParseLogStream_WithInvalidLines(t *testing.T) {
	p := NewParser(nil)
	ch := make(chan []byte, 4)
	go func() {
		ch <- []byte("2024-01-15T10:30:00Z [INFO] Valid message")
		ch <- []byte("invalid line")
		ch <- []byte("")
		ch <- []byte("2024-01-15T10:31:00Z [ERROR] Another valid")
		close(ch)
	}()

	entries, errs := p.ParseLogStream(ch)

	if len(entries) != 2 {
		t.Errorf("ParseLogStream() got %d entries, want 2", len(entries))
	}
	if len(errs) != 2 {
		t.Errorf("ParseLogStream() got %d errors, want 2", len(errs))
	}
}

func TestNewParserFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		rules   []RuleConfig
		wantLen int
		wantErr bool
	}{
		{
			name: "valid single rule",
			rules: []RuleConfig{
				{
					Name:    "test",
					Pattern: `^\d+`,
				},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "valid multiple rules",
			rules: []RuleConfig{
				{Name: "a", Pattern: `^a`},
				{Name: "b", Pattern: `^b`},
				{Name: "c", Pattern: `^c`},
			},
			wantLen: 3,
			wantErr: false,
		},
		{
			name: "invalid regex",
			rules: []RuleConfig{
				{
					Name:    "invalid",
					Pattern: `(?P<bad`,
				},
			},
			wantErr: true,
		},
		{
			name: "skip empty pattern",
			rules: []RuleConfig{
				{
					Name:    "error",
					Pattern: "",
				},
			},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewParserFromConfig(tt.rules)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewParserFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(p.Rules()) != tt.wantLen {
				t.Errorf("NewParserFromConfig() got %d rules, want %d", len(p.Rules()), tt.wantLen)
			}
		})
	}
}

func TestParser_UpdateRules(t *testing.T) {
	p := NewParser(nil)

	newRules := []Rule{
		{
			Name:  "new",
			Regex: regexp.MustCompile("^NEW: .*$"),
		},
	}
	p.UpdateRules(newRules)

	if len(p.Rules()) != 1 || p.Rules()[0].Name != "new" {
		t.Errorf("UpdateRules() failed")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", LevelDebug},
		{"INFO", LevelInfo},
		{"ERROR", LevelError},
		{"debug", LevelInfo},
		{"info", LevelInfo},
		{"error", LevelInfo},
		{"", LevelInfo},
		{"UNKNOWN", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "RFC3339",
			input: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 with timezone",
			input: "2024-01-15T10:30:00+03:00",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", 3*60*60)),
		},
		{
			name:  "date only",
			input: "2024-01-15",
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "datetime with space",
			input: "2024-01-15 10:30:00",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:    "invalid",
			input:   "not-a-timestamp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"no ansi", "no ansi"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"\x1b[0;0;0m", ""},
		{"", ""},
		{"prefix\x1b[32msuffix", "prefixsuffix"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanLine([]byte(tt.input))
			if !bytes.Equal(got, []byte(tt.expected)) {
				t.Errorf("cleanLine() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLogEntry_Fields(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level:     LevelInfo,
		Message:   []byte("test"),
		RuleName:  "test-rule",
		Raw:       []byte("raw"),
	}

	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if entry.Level != LevelInfo {
		t.Errorf("Level = %v, want %v", entry.Level, LevelInfo)
	}
	if !bytes.Equal(entry.Message, []byte("test")) {
		t.Errorf("Message = %v, want %v", entry.Message, "test")
	}
}

func errorsIs(err, target error) bool {
	if errors.Is(err, target) {
		return true
	}
	if err == nil || target == nil {
		return false
	}
	return err.Error() == target.Error()
}
