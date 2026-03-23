package telegram

import (
	"html"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func (b *Bot) handleRegexesCommand(_ *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	rules, fromDefaults := b.regexManager.GetActiveRulesWithSource(chatID)
	isSubscribed := b.subscriptionMgr.IsSubscribed(chatID)

	var msg strings.Builder
	msg.WriteString("<b>Regex-правила для этого чата</b>\n\n")

	if len(rules) == 0 {
		msg.WriteString("Активных regex-правил нет. Отправляем все сообщения.\n")
		if fromDefaults {
			msg.WriteString("(это из дефолтных правил)\n")
		} else {
			msg.WriteString("(это из правил чата: overrides)\n")
		}
	} else {
		for i, r := range rules {
			if i > 0 {
				msg.WriteString("\n")
			}
			msg.WriteString("<code>")
			msg.WriteString(html.EscapeString(r.Name))
			msg.WriteString("</code>: <code>")
			msg.WriteString(html.EscapeString(r.Pattern))
			msg.WriteString("</code>")
			if fromDefaults {
				msg.WriteString(" <b>[default]</b>")
			}
		}
	}

	if isSubscribed {
		msg.WriteString("\n\nЧтобы изменить правила, сначала выполните /stop.")
	} else {
		msg.WriteString("\n\nЧтобы изменить правила, выполните /addregex.")
	}

	return b.client.SendMessageHTML(chatID, msg.String())
}
