package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kxrxh/logram/internal/buffer"
	"github.com/kxrxh/logram/internal/config"
	"github.com/kxrxh/logram/internal/parser"
	"github.com/kxrxh/logram/internal/reader"
)

func main() {
	pathEnv := os.Getenv("CONFIG_PATH")
	if pathEnv == "" {
		pathEnv = "./config.yaml"
	}

	cfg, err := config.Load(pathEnv)
	if err != nil {
		panic(err)
	}

	p, err := parser.NewParserFromConfig(toRuleConfig(cfg.Get().Parser.Rules))
	if err != nil {
		panic(err)
	}

	cfg.Watch(func(newCfg *config.Config) {
		log.Println("Config reloaded, updating parser rules")
		newRules, err := parser.NewParserFromConfig(toRuleConfig(newCfg.Parser.Rules))
		if err != nil {
			log.Printf("Failed to update rules: %v", err)
			return
		}
		p.UpdateRules(newRules.Rules())
		log.Println("Parser rules updated:", newCfg.Parser.Rules)
	})

	ctx, cancel := context.WithCancel(context.Background())

	readChan := reader.ReadFileTail(ctx, cfg.Get().Logs.Path)

	buf := buffer.New(ctx, 100, 500*time.Millisecond, buffer.WithPolicy(buffer.DropOldest))
	buf.Start()

	go func() {
		for line := range readChan {
			buf.Input() <- line
		}
	}()

	go func() {
		for batch := range buf.Output() {
			entry, err := p.ParseLine(batch)
			if err != nil {
				if errors.Is(err, parser.ErrEmptyLine) || errors.Is(err, parser.ErrNoMatchRules) {
					continue
				}
				log.Printf("Parse error: %v", err)
				continue
			}
			log.Printf("Parsed entry: %s [%s] %s", entry.Timestamp.Format(time.RFC3339), entry.Level, string(entry.Message))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	buf.Stop()
}

func toRuleConfig(rules []config.Rule) []parser.RuleConfig {
	result := make([]parser.RuleConfig, len(rules))
	for i, r := range rules {
		result[i] = parser.RuleConfig{
			Name:    r.Name,
			Pattern: r.Pattern,
		}
	}
	return result
}
