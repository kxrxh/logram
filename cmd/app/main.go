package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kxrxh/logram/internal/buffer"
	"github.com/kxrxh/logram/internal/config"
	"github.com/kxrxh/logram/internal/database"
	"github.com/kxrxh/logram/internal/parser"
	"github.com/kxrxh/logram/internal/reader"
	"github.com/kxrxh/logram/internal/telegram"
)

func main() {
	pathEnv := os.Getenv("CONFIG_PATH")
	if pathEnv == "" {
		pathEnv = "./config.yaml"
	}

	cfg, err := config.Load(pathEnv)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	var db *database.DB
	if cfg.Get().Database.Path != "" {
		db, err = database.New(cfg.Get().Database.Path)
		if err != nil {
			log.Printf("initialize database: %v", err)
		}
	}

	p, err := parser.NewParserFromConfig(toRuleConfig(cfg.Get().Parser.Rules))
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		log.Fatalf("create parser: %v", err)
	}

	defer func() {
		if db != nil {
			if err := db.Close(); err != nil {
				log.Printf("close database: %v", err)
			}
		}
	}()

	var bot *telegram.Bot
	if cfg.Get().Bot.Token != "" {
		bot, err = telegram.NewBot(cfg.Get().Bot.Token, db)
		if err != nil {
			log.Printf("initialize telegram bot: %v", err)
		} else if err := bot.Start(); err != nil {
			log.Printf("start telegram bot: %v", err)
			bot = nil
		}
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

	batchCfg := cfg.Get().Batch
	buf := buffer.New(
		ctx,
		batchCfg.Size,
		batchCfg.Interval,
		buffer.WithPolicy(getPolicy(batchCfg.Policy)),
	)
	buf.Start()

	go func() {
		for line := range readChan {
			buf.Input() <- line
		}
	}()

	parsedChan := p.Start(ctx, buf.Output())

	var sendChan chan parser.LogEntry
	if bot != nil {
		sendChan = make(chan parser.LogEntry, batchCfg.Size)
		go func() {
			for entry := range sendChan {
				if err := bot.SendLog(entry); err != nil {
					log.Printf("send telegram message: %v", err)
				}
			}
		}()
	}

	for entry := range parsedChan {
		if bot != nil {
			sendChan <- entry
		} else {
			// Non-production path
			// #nosec G706 // false positive: dev-only log of parsed entry, not production input
			log.Printf("parsed entry: %q [%s] %q",
				entry.Timestamp.Format(time.RFC3339),
				entry.Level,
				string(entry.Message))
		}
	}

	if bot != nil {
		close(sendChan)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	buf.Stop()
	if bot != nil {
		bot.Stop()
	}
}

func getPolicy(p string) buffer.BufferPolicy {
	switch p {
	case "block_on_full":
		return buffer.BlockOnFull
	case "drop_new":
		return buffer.DropNew
	case "drop_oldest":
		return buffer.DropOldest
	default:
		return buffer.BlockOnFull
	}
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
