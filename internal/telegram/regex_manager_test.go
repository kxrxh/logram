package telegram

import (
	"testing"

	"github.com/kxrxh/logram/internal/database"
	"github.com/kxrxh/logram/internal/parser"
	"github.com/stretchr/testify/require"
)

func ruleCfg(name, pattern string) parser.RuleConfig {
	return parser.RuleConfig{Name: name, Pattern: pattern}
}

func chatRuleCfg(name, pattern string) database.ChatRegexRule {
	return database.ChatRegexRule{
		ChatID:  1,
		Name:    name,
		Pattern: pattern,
	}
}

func TestRegexManager_DefaultsVsOverrides(t *testing.T) {
	rm, err := NewRegexManager([]parser.RuleConfig{
		ruleCfg("defaultFoo", "FOO"),
		ruleCfg("defaultBar", "BAR"),
	})
	require.NoError(t, err)

	// chat 1: override, but not FOO
	err = rm.RefreshChatRules(1, []database.ChatRegexRule{
		chatRuleCfg("overrideBar", "BAR"),
	})
	require.NoError(t, err)

	require.False(t, rm.ShouldSend(1, []byte("FOO")))
	require.True(t, rm.ShouldSend(1, []byte("BAR")))

	// chat 2: defaults
	require.True(t, rm.ShouldSend(2, []byte("FOO")))
}

func TestRegexManager_EmptyRuleSetSendsAll(t *testing.T) {
	rm, err := NewRegexManager(nil)
	require.NoError(t, err)

	require.True(t, rm.ShouldSend(1, []byte("ANYTHING")))

	// empty overrides => treat as none
	err = rm.RefreshChatRules(1, []database.ChatRegexRule{})
	require.NoError(t, err)
	require.True(t, rm.ShouldSend(1, []byte("ANYTHING")))
}

func TestRegexManager_RuleOrdering(t *testing.T) {
	rm, err := NewRegexManager(nil)
	require.NoError(t, err)

	// first match wins
	err = rm.RefreshChatRules(1, []database.ChatRegexRule{
		chatRuleCfg("first", "FOO.*"),
		chatRuleCfg("second", "FOO"),
	})
	require.NoError(t, err)

	name, ok := rm.MatchFirstRuleName(1, []byte("FOO"))
	require.True(t, ok)
	require.Equal(t, "first", name)

	// swap: first fails, second wins
	err = rm.RefreshChatRules(1, []database.ChatRegexRule{
		chatRuleCfg("first", "^NOPE$"),
		chatRuleCfg("second", "^FOO$"),
	})
	require.NoError(t, err)

	name, ok = rm.MatchFirstRuleName(1, []byte("FOO"))
	require.True(t, ok)
	require.Equal(t, "second", name)
}

func TestRegexManager_GetActiveRulesWithSource(t *testing.T) {
	rm, err := NewRegexManager([]parser.RuleConfig{
		ruleCfg("defaultFoo", "FOO"),
		ruleCfg("defaultBar", "BAR"),
	})
	require.NoError(t, err)

	// chat 2: defaults
	rules, fromDefaults := rm.GetActiveRulesWithSource(2)
	require.True(t, fromDefaults)
	require.Len(t, rules, 2)
	require.Equal(t, "defaultFoo", rules[0].Name)
	require.Equal(t, "FOO", rules[0].Pattern)
	require.Equal(t, "defaultBar", rules[1].Name)
	require.Equal(t, "BAR", rules[1].Pattern)

	// chat 1: overrides
	err = rm.RefreshChatRules(1, []database.ChatRegexRule{
		chatRuleCfg("overrideBar", "BAR"),
	})
	require.NoError(t, err)

	rules, fromDefaults = rm.GetActiveRulesWithSource(1)
	require.False(t, fromDefaults)
	require.Len(t, rules, 1)
	require.Equal(t, "overrideBar", rules[0].Name)
	require.Equal(t, "BAR", rules[0].Pattern)
}
