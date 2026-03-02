package parser

import (
	"fmt"
	"testing"
)

func BenchmarkParseTimestamp(b *testing.B) {
	cases := []struct {
		name string
		val  string
	}{
		{"RFC3339", "2024-01-15T10:30:00Z"},
		{"RFC3339_TZ", "2024-01-15T10:30:00+03:00"},
		{"DateTime", "2024-01-15 10:30:00"},
		{"DateOnly", "2024-01-15"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = parseTimestamp(tc.val)
			}
		})
	}
}

func BenchmarkParseLine_Parallel(b *testing.B) {
	p := NewParser(nil)
	line := []byte("2024-01-15T10:30:00Z [INFO] Application started successfully")

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = p.ParseLine(line)
		}
	})
}

func BenchmarkParseLogStream(b *testing.B) {
	p := NewParser(nil)
	numLines := 100
	lines := make([][]byte, numLines)
	for i := range numLines {
		bl := make([]byte, 0, 64)
		bl = fmt.Appendf(bl, "2024-01-15 10:30:00 [INFO] Message %d", i)
		lines[i] = bl
	}

	b.ResetTimer()
	for b.Loop() {
		ch := make(chan []byte, numLines)
		for _, l := range lines {
			ch <- l
		}
		close(ch)

		_, _ = p.ParseLogStream(ch)
	}
}

func BenchmarkRules(b *testing.B) {
	singleRule, _ := NewParserFromConfig([]RuleConfig{
		{
			Name:    "access_log",
			Pattern: `^(\d{4}-\d{2}-\d{2}) \[(?P<level>\w+)\] (?P<message>.*)$`,
		},
	})

	multiRule, _ := NewParserFromConfig([]RuleConfig{
		{Name: "rule1", Pattern: `^TypeA: .*$`},
		{Name: "rule2", Pattern: `^TypeB: .*$`},
		{Name: "rule3", Pattern: `^TypeC: .*$`},
		{Name: "rule4", Pattern: `^TypeD: .*$`},
		{
			Name:    "rule5_match",
			Pattern: `^(\d{4}-\d{2}-\d{2}) \[(?P<level>\w+)\] (?P<message>.*)$`,
		},
	})

	line := []byte("2024-01-15 [INFO] This is a custom rule match")

	b.Run("SingleRegexMatch", func(b *testing.B) {
		for b.Loop() {
			_, _ = singleRule.ParseLine(line)
		}
	})

	b.Run("LinearSearchMatch", func(b *testing.B) {
		for b.Loop() {
			_, _ = multiRule.ParseLine(line)
		}
	})

	b.Run("NoMatchFound", func(b *testing.B) {
		invalidLine := []byte("Something that matches nothing")
		for b.Loop() {
			_, _ = multiRule.ParseLine(invalidLine)
		}
	})
}

func BenchmarkErrorHandling(b *testing.B) {
	p := NewParser(nil)

	b.Run("InvalidTimestamp", func(b *testing.B) {
		line := []byte("bad-date [INFO] message")
		for b.Loop() {
			_, _ = p.ParseLine(line)
		}
	})

	b.Run("InvalidFormat", func(b *testing.B) {
		line := []byte("JustAMessageWithoutBrackets")
		for b.Loop() {
			_, _ = p.ParseLine(line)
		}
	})
}

func BenchmarkParserCreation(b *testing.B) {
	config := []RuleConfig{
		{
			Name:    "complex",
			Pattern: `^(?P<ts>\d+) \[(?P<lvl>\w+)\] (?P<msg>.*)$`,
		},
	}

	b.Run("NewParserFromConfig", func(b *testing.B) {
		// Tests the overhead of regex compilation
		for b.Loop() {
			_, _ = NewParserFromConfig(config)
		}
	})
}

func BenchmarkParseLogStream_WithRules(b *testing.B) {
	p, _ := NewParserFromConfig([]RuleConfig{
		{
			Name:    "custom",
			Pattern: `^(\d{4}-\d{2}-\d{2}) \[(?P<level>\w+)\] (?P<message>.*)$`,
		},
	})

	numLines := 100
	lines := make([][]byte, numLines)
	for i := range numLines {
		bl := make([]byte, 0, 64)
		bl = fmt.Appendf(bl, "2024-01-15 [INFO] Log number %d", i)
		lines[i] = bl
	}

	b.ResetTimer()
	for b.Loop() {
		ch := make(chan []byte, numLines)
		for _, l := range lines {
			ch <- l
		}
		close(ch)
		_, _ = p.ParseLogStream(ch)
	}
}
