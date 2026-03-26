package telegram

import (
	"fmt"
	"html"
	"strings"

	"github.com/kxrxh/logram/internal/parser"
)

type MessageFormatter struct{}

func NewMessageFormatter() *MessageFormatter {
	return &MessageFormatter{}
}

func (f *MessageFormatter) FormatLogEntry(entry parser.LogEntry) string {
	levelText := getLevelText(entry.Level)
	// Telegram `parse_mode=HTML` requires escaping any raw `<`/`&` in user/log content.
	safeMsg := html.EscapeString(string(entry.Message))
	return fmt.Sprintf("<b>%s</b> | %s\n<code>%s</code>",
		levelText,
		entry.Timestamp.Format("02.01.2006 15:04:05"),
		safeMsg)
}

func (f *MessageFormatter) FormatSubscriptionStatus(isSubscribed bool) string {
	if isSubscribed {
		return "<b>Подписан</b>\n\nВы получаете уведомления о логах."
	}
	return "<b>Не подписан</b>\n\nВы не получаете уведомления о логах."
}

func (f *MessageFormatter) FormatHelp(commands map[string]Command) string {
	var helpText strings.Builder
	helpText.WriteString("<b>Доступные команды:</b>\n\n")
	for name, cmd := range commands {
		fmt.Fprintf(&helpText, "/%s - %s\n", name, cmd.Description)
	}
	return helpText.String()
}

func getLevelText(level parser.LogLevel) string {
	switch level {
	case parser.LevelDebug:
		return "DEBUG"
	case parser.LevelInfo:
		return "INFO"
	case parser.LevelError:
		return "ERROR"
	default:
		return string(level)
	}
}
