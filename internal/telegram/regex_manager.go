package telegram

import (
	"regexp"
	"sync"

	"github.com/kxrxh/logram/internal/database"
	"github.com/kxrxh/logram/internal/parser"
)

type compiledRule struct {
	name  string
	regex *regexp.Regexp
}

type RegexManager struct {
	mu sync.RWMutex

	defaultRules  []compiledRule
	chatOverrides map[int64][]compiledRule
}

func NewRegexManager(defaultRules []parser.RuleConfig) (*RegexManager, error) {
	rm := &RegexManager{
		chatOverrides: make(map[int64][]compiledRule),
	}

	if err := rm.SetDefaultRules(defaultRules); err != nil {
		return nil, err
	}

	return rm, nil
}

func (rm *RegexManager) SetDefaultRules(rules []parser.RuleConfig) error {
	compiled, err := compileRuleConfigs(rules)
	if err != nil {
		return err
	}

	rm.mu.Lock()
	rm.defaultRules = compiled
	rm.mu.Unlock()
	return nil
}

func (rm *RegexManager) RefreshChatRules(chatID int64, rules []database.ChatRegexRule) error {
	compiled, err := compileChatRules(rules)
	if err != nil {
		return err
	}

	rm.mu.Lock()
	if len(compiled) == 0 {
		delete(rm.chatOverrides, chatID)
	} else {
		rm.chatOverrides[chatID] = compiled
	}
	rm.mu.Unlock()
	return nil
}

func (rm *RegexManager) ClearChatRules(chatID int64) {
	rm.mu.Lock()
	delete(rm.chatOverrides, chatID)
	rm.mu.Unlock()
}

func (rm *RegexManager) ShouldSend(chatID int64, raw []byte) bool {
	rm.mu.RLock()
	override, hasOverride := rm.chatOverrides[chatID]
	if hasOverride {
		compiled := override
		rm.mu.RUnlock()
		return matchesAny(compiled, raw)
	}
	compiled := rm.defaultRules
	rm.mu.RUnlock()
	return matchesAny(compiled, raw)
}

func (rm *RegexManager) MatchFirstRuleName(chatID int64, raw []byte) (string, bool) {
	rm.mu.RLock()
	override, hasOverride := rm.chatOverrides[chatID]
	var compiled []compiledRule
	if hasOverride {
		compiled = override
	} else {
		compiled = rm.defaultRules
	}
	rm.mu.RUnlock()

	if len(compiled) == 0 {
		return "", true
	}

	for _, r := range compiled {
		if r.regex.Match(raw) {
			return r.name, true
		}
	}
	return "", false
}

func (rm *RegexManager) GetActiveRules(chatID int64) []parser.RuleConfig {
	rules, _ := rm.GetActiveRulesWithSource(chatID)
	return rules
}

func (rm *RegexManager) GetActiveRulesWithSource(chatID int64) ([]parser.RuleConfig, bool) {
	rm.mu.RLock()
	override, hasOverride := rm.chatOverrides[chatID]
	if hasOverride {
		rm.mu.RUnlock()
		return compiledToRuleConfigs(override), false
	}
	compiled := rm.defaultRules
	rm.mu.RUnlock()

	return compiledToRuleConfigs(compiled), true
}

func compileRuleConfigs(rules []parser.RuleConfig) ([]compiledRule, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, &parser.RuleError{Rule: r.Name, Reason: err}
		}
		compiled = append(compiled, compiledRule{
			name:  r.Name,
			regex: re,
		})
	}
	return compiled, nil
}

func compileChatRules(rules []database.ChatRegexRule) ([]compiledRule, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, &parser.RuleError{Rule: r.Name, Reason: err}
		}
		compiled = append(compiled, compiledRule{
			name:  r.Name,
			regex: re,
		})
	}
	return compiled, nil
}

func matchesAny(rules []compiledRule, raw []byte) bool {
	if len(rules) == 0 {
		return true
	}

	for _, r := range rules {
		if r.regex.Match(raw) {
			return true
		}
	}
	return false
}

func compiledToRuleConfigs(rules []compiledRule) []parser.RuleConfig {
	out := make([]parser.RuleConfig, 0, len(rules))
	for _, r := range rules {
		out = append(out, parser.RuleConfig{
			Name:    r.name,
			Pattern: r.regex.String(),
		})
	}
	return out
}
