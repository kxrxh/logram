package telegram

import (
	"context"
	"log"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type Bot struct {
	client     *telego.Bot
	botHandler *th.BotHandler
	updates    <-chan telego.Update
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewBot(token string) (*Bot, error) {
	client, err := telego.NewBot(token, telego.WithDefaultDebugLogger())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Bot{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (b *Bot) Start() error {
	_, err := b.client.GetMe(b.ctx)
	if err != nil {
		return err
	}

	b.updates, err = b.client.UpdatesViaLongPolling(b.ctx, nil)
	if err != nil {
		return err
	}

	b.botHandler, err = th.NewBotHandler(b.client, b.updates)
	if err != nil {
		return err
	}

	go func() {
		if err := b.botHandler.Start(); err != nil {
			log.Printf("bot handler start error: %v", err)
		}
	}()

	return nil
}

func (b *Bot) Client() *telego.Bot {
	return b.client
}

func (b *Bot) Context() context.Context {
	return b.ctx
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	_, err := b.client.SendMessage(b.ctx, tu.Message(
		tu.ID(chatID),
		text,
	))
	return err
}

func (b *Bot) SendMessageWithKeyboard(chatID int64, text string, buttons [][]string) error {
	var keyboard [][]telego.InlineKeyboardButton
	for _, row := range buttons {
		var buttonRow []telego.InlineKeyboardButton
		for _, btn := range row {
			buttonRow = append(buttonRow, telego.InlineKeyboardButton{
				Text: btn,
			})
		}
		keyboard = append(keyboard, buttonRow)
	}

	_, err := b.client.SendMessage(b.ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithReplyMarkup(tu.InlineKeyboard(keyboard...)))

	return err
}

func (b *Bot) HandleCommands(handler func(*th.Context, telego.Message) error, predicates ...th.Predicate) {
	b.botHandler.HandleMessage(handler, predicates...)
}

func (b *Bot) HandleMessages(handler func(*th.Context, telego.Message) error) {
	b.botHandler.HandleMessage(handler)
}

func (b *Bot) HandleCallbackQueries(handler func(*th.Context, telego.CallbackQuery) error) {
	b.botHandler.HandleCallbackQuery(handler)
}

func (b *Bot) Stop() {
	if b.botHandler != nil {
		if err := b.botHandler.Stop(); err != nil {
			log.Printf("bot handler stop error: %v", err)
		}
	}
	b.cancel()
	log.Println("Telegram bot stopped")
}
