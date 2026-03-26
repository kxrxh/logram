package telegram

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type addRegexWizardState struct {
	step     int
	ruleName string
}

const (
	addRegexStepName    = 1
	addRegexStepPattern = 2

	callbackRemoveRegexPrefix = "dr:"
)

func (b *Bot) handleAddRegexCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	if b.subscriptionMgr.IsSubscribed(chatID) {
		return b.sendAlreadySubscribed(chatID)
	}

	if b.db == nil {
		return b.client.SendMessageHTML(
			chatID,
			"База данных не настроена, невозможно сохранить regex.",
		)
	}

	b.addRegexMu.Lock()
	b.addRegex[chatID] = &addRegexWizardState{step: addRegexStepName}
	b.addRegexMu.Unlock()

	return b.client.SendMessageHTMLWithReplyMarkup(
		chatID,
		"Шаг 1/2. Введите имя правила (например: error или warn).",
		tu.ForceReply(),
	)
}

func (b *Bot) handleResetRegexCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	if b.subscriptionMgr.IsSubscribed(chatID) {
		return b.sendAlreadySubscribed(chatID)
	}

	if b.db == nil {
		return b.client.SendMessageHTML(
			chatID,
			"База данных не настроена, невозможно сохранить regex.",
		)
	}

	if err := b.db.DeleteAllChatRegexRules(chatID); err != nil {
		b.sendErrorResponse(chatID, "reset regex", err)
		return nil
	}
	b.regexManager.ClearChatRules(chatID)

	return b.client.SendMessageHTML(
		chatID,
		"Все regex-правила для этого чата сброшены к значениям по умолчанию.",
	)
}

func (b *Bot) handleRemoveRegexCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	if b.subscriptionMgr.IsSubscribed(chatID) {
		return b.sendAlreadySubscribed(chatID)
	}

	if b.db == nil {
		return b.client.SendMessageHTML(
			chatID,
			"База данных не настроена, невозможно удалить regex.",
		)
	}

	rules, err := b.db.GetChatRegexRules(chatID)
	if err != nil {
		b.sendErrorResponse(chatID, "list chat regex rules", err)
		return nil
	}
	if len(rules) == 0 {
		return b.client.SendMessageHTML(
			chatID,
			"У этого чата нет сохраненных regex-правил. Сначала добавьте их через /addregex.",
		)
	}

	rows := make([][]telego.InlineKeyboardButton, 0, len(rules))
	for _, r := range rules {
		encName := base64.RawURLEncoding.EncodeToString([]byte(r.Name))
		cbData := callbackRemoveRegexPrefix + encName
		btn := tu.InlineKeyboardButton(r.Name).WithCallbackData(cbData)
		rows = append(rows, tu.InlineKeyboardRow(btn))
	}

	markup := tu.InlineKeyboard(rows...)
	return b.client.SendMessageHTMLWithReplyMarkup(
		chatID,
		"Выберите правило для удаления:",
		markup,
	)
}

func (b *Bot) handleAddRegexWizardMessage(_ *th.Context, message telego.Message) (bool, error) {
	if message.Text == "" {
		return false, nil
	}

	chatID := message.Chat.ID
	if strings.HasPrefix(message.Text, "/") {
		return false, nil
	}

	b.addRegexMu.Lock()
	state, exists := b.addRegex[chatID]
	if !exists || state == nil {
		b.addRegexMu.Unlock()
		return false, nil
	}
	step := state.step
	ruleName := state.ruleName
	b.addRegexMu.Unlock()

	if b.subscriptionMgr.IsSubscribed(chatID) {
		b.addRegexMu.Lock()
		delete(b.addRegex, chatID)
		b.addRegexMu.Unlock()
		return true, b.sendAlreadySubscribed(chatID)
	}

	if b.db == nil {
		b.addRegexMu.Lock()
		delete(b.addRegex, chatID)
		b.addRegexMu.Unlock()
		return true, b.client.SendMessageHTML(
			chatID,
			"База данных не настроена, невозможно сохранить regex.",
		)
	}

	input := strings.TrimSpace(message.Text)
	switch step {
	case addRegexStepName:
		if input == "" {
			return true, b.client.SendMessageHTML(
				chatID,
				"Имя правила не может быть пустым. Введите имя:",
			)
		}

		b.addRegexMu.Lock()
		b.addRegex[chatID].ruleName = input
		b.addRegex[chatID].step = addRegexStepPattern
		b.addRegexMu.Unlock()

		return true, b.client.SendMessageHTMLWithReplyMarkup(
			chatID,
			fmt.Sprintf("Шаг 2/2. Введите regex для правила %q.", input),
			tu.ForceReply(),
		)

	case addRegexStepPattern:
		if ruleName == "" {
			b.addRegexMu.Lock()
			delete(b.addRegex, chatID)
			b.addRegexMu.Unlock()
			return true, b.client.SendMessageHTML(
				chatID,
				"Состояние мастера было потеряно. Запустите /addregex заново.",
			)
		}
		if input == "" {
			return true, b.client.SendMessageHTML(
				chatID,
				"Regex не может быть пустым. Введите regex:",
			)
		}

		if _, err := regexp.Compile(input); err != nil {
			return true, b.client.SendMessageHTML(
				chatID,
				fmt.Sprintf("Неверный regex: %v. Попробуйте еще раз.", err),
			)
		}

		if err := b.db.UpsertChatRegexRule(chatID, ruleName, input); err != nil {
			b.sendErrorResponse(chatID, "add regex", err)
			return true, nil
		}

		chatRules, err := b.db.GetChatRegexRules(chatID)
		if err != nil {
			b.sendErrorResponse(chatID, "reload chat regex rules", err)
			return true, nil
		}

		if err := b.regexManager.RefreshChatRules(chatID, chatRules); err != nil {
			b.sendErrorResponse(chatID, "compile chat regex rules", err)
			return true, nil
		}

		b.addRegexMu.Lock()
		delete(b.addRegex, chatID)
		b.addRegexMu.Unlock()

		return true, b.client.SendMessageHTML(
			chatID,
			fmt.Sprintf("Готово! Правило %q сохранено.", ruleName),
		)

	default:
		b.addRegexMu.Lock()
		delete(b.addRegex, chatID)
		b.addRegexMu.Unlock()
		return true, b.client.SendMessageHTML(
			chatID,
			"Состояние мастера некорректно. Запустите /addregex заново.",
		)
	}
}

func (b *Bot) handleRemoveRegexCallbackQuery(ctx *th.Context, query telego.CallbackQuery) error {
	if b.db == nil {
		return nil
	}

	if !strings.HasPrefix(query.Data, callbackRemoveRegexPrefix) {
		return nil
	}

	if query.Message == nil {
		_ = ctx.Bot().AnswerCallbackQuery(
			ctx,
			tu.CallbackQuery(query.ID).WithText("Некорректные данные"),
		)
		return nil
	}

	chatID := query.Message.GetChat().ID
	if chatID == 0 {
		_ = ctx.Bot().AnswerCallbackQuery(
			ctx,
			tu.CallbackQuery(query.ID).WithText("Некорректные данные"),
		)
		return nil
	}

	if b.subscriptionMgr.IsSubscribed(chatID) {
		_ = ctx.Bot().
			AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Сначала выполните /stop"))
		return nil
	}

	encName := strings.TrimPrefix(query.Data, callbackRemoveRegexPrefix)
	rawName, err := base64.RawURLEncoding.DecodeString(encName)
	if err != nil {
		_ = ctx.Bot().
			AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Некорректные данные"))
		return nil
	}
	ruleName := string(rawName)

	if err := b.db.DeleteChatRegexRule(chatID, ruleName); err != nil {
		b.sendErrorResponse(chatID, "delete chat regex rule", err)
		_ = ctx.Bot().
			AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Ошибка удаления"))
		return nil
	}

	chatRules, err := b.db.GetChatRegexRules(chatID)
	if err == nil {
		_ = b.regexManager.RefreshChatRules(chatID, chatRules)
	}

	_ = ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Удалено"))
	return nil
}
