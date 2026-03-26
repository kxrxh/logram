package telegram

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kxrxh/logram/internal/database"
	"github.com/kxrxh/logram/internal/parser"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type Bot struct {
	client          *Client
	subscriptionMgr *SubscriptionManager
	regexManager    *RegexManager
	db              *database.DB
	batchManager    *BatchManager

	addRegexMu sync.Mutex
	addRegex   map[int64]*addRegexWizardState

	ctx             context.Context
	cancel          context.CancelFunc
	commandRegistry *CommandRegistry
	formatter       *MessageFormatter
}

func NewBot(token string, db *database.DB, regexManager *RegexManager) (*Bot, error) {
	client, err := NewClient(token)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	var initialBatchEnabled map[int64]bool
	if db != nil {
		initialBatchEnabled = make(map[int64]bool)
		chats, err := db.GetAllChats()
		if err != nil {
			log.Printf("failed to load chat batching flags: %v", err)
		} else {
			for _, chat := range chats {
				if chat.BatchEnabled {
					initialBatchEnabled[chat.ChatID] = true
				}
			}
		}
	} else {
		initialBatchEnabled = make(map[int64]bool)
	}

	bm := NewBatchManager(
		ctx,
		5*time.Second,
		func(chatID int64, text string) error {
			return client.SendMessageHTML(chatID, text)
		},
		initialBatchEnabled,
	)

	return &Bot{
		client:          client,
		subscriptionMgr: NewSubscriptionManager(db),
		regexManager:    regexManager,
		db:              db,
		batchManager:    bm,
		addRegex:        make(map[int64]*addRegexWizardState),
		commandRegistry: NewCommandRegistry(),
		formatter:       NewMessageFormatter(),
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

func (b *Bot) RegisterCommand(
	name, description string,
	handler func(ctx *th.Context, update telego.Update) error,
	aliases ...string,
) {
	b.commandRegistry.Register(name, description, handler, aliases...)
}

func (b *Bot) setupCommands() {
	b.RegisterCommand("start", "Начать получать уведомления о логах", b.handleStartCommand)
	b.RegisterCommand("stop", "Отписаться от уведомлений", b.handleStopCommand)
	b.RegisterCommand("help", "Показать доступные команды", b.handleHelpCommand)
	b.RegisterCommand(
		"batch",
		"Вкл/выкл группировку логов (сообщения батчами)",
		b.handleBatchCommand,
	)
	b.RegisterCommand(
		"regexes",
		"Показать текущие regex-правила для этого чата",
		b.handleRegexesCommand,
	)
	b.RegisterCommand(
		"addregex",
		"Добавить regex-фильтр для этого чата (бот должен быть отключен)",
		b.handleAddRegexCommand,
	)
	b.RegisterCommand(
		"resetregex",
		"Сбросить все regex-фильтры для этого чата к значениям по умолчанию",
		b.handleResetRegexCommand,
	)
	b.RegisterCommand(
		"removeregex",
		"Удалить одно regex-правило для этого чата",
		b.handleRemoveRegexCommand,
	)
	b.RegisterCommand(
		"status",
		"Показать текущий статус подписки",
		b.handleStatusCommand,
		"subscribe",
		"subscription",
	)
}

func (b *Bot) Start() error {
	_, err := b.client.GetMe()
	if err != nil {
		return err
	}

	updates, err := b.client.Updates()
	if err != nil {
		return err
	}

	botHandler, err := th.NewBotHandler(b.client.Bot(), updates)
	if err != nil {
		return err
	}

	b.setupCommands()

	// Register command handlers
	for name := range b.commandRegistry.GetAllCommands() {
		botHandler.Handle(func(ctx *th.Context, update telego.Update) error {
			return b.handleCommand(ctx, update, name)
		}, th.CommandEqual(name))
	}

	// Register aliases
	for alias := range b.commandRegistry.GetAllAliases() {
		botHandler.Handle(func(ctx *th.Context, update telego.Update) error {
			if cmd, exists := b.commandRegistry.GetCommandByAlias(alias); exists {
				return cmd.Handler(ctx, update)
			}
			return nil
		}, th.CommandEqual(alias))
	}

	botHandler.HandleMessage(b.handleAnyMessage)

	botHandler.HandleCallbackQuery(
		func(ctx *th.Context, query telego.CallbackQuery) error {
			return b.handleRemoveRegexCallbackQuery(ctx, query)
		},
		th.AnyCallbackQueryWithMessage(),
		th.CallbackDataPrefix(callbackRemoveRegexPrefix),
	)

	go func() {
		if err := botHandler.Start(); err != nil {
			log.Printf("bot handler start error: %v", err)
		}
	}()

	return nil
}

func (b *Bot) handleCommand(ctx *th.Context, update telego.Update, commandName string) error {
	if update.Message == nil {
		return nil
	}

	cmd, exists := b.commandRegistry.GetCommand(commandName)
	if !exists {
		return nil
	}

	return cmd.Handler(ctx, update)
}

func (b *Bot) handleStartCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chat := update.Message.Chat
	chatID := chat.ID

	if b.subscriptionMgr.IsSubscribed(chatID) {
		return b.sendAlreadySubscribed(chatID)
	}

	title := chat.Title
	if title == "" {
		title = "Chat"
	}

	if err := b.subscriptionMgr.AddChat(chatID, title); err != nil {
		log.Printf("Failed to save chat %d: %v", chatID, err)
		b.sendErrorResponse(chatID, "subscription", err)
		return nil
	}

	log.Printf("New chat registered: %s (%d)", title, chatID)
	return b.sendActivationMessage(chatID)
}

func (b *Bot) handleAnyMessage(ctx *th.Context, message telego.Message) error {
	if message.From == nil || message.From.IsBot {
		return nil
	}

	if consumed, err := b.handleAddRegexWizardMessage(ctx, message); consumed || err != nil {
		return err
	}

	if len(message.Text) > 0 && message.Text[0] == '/' {
		commandName := strings.TrimPrefix(message.Text, "/")
		commandParts := strings.Fields(commandName)
		if len(commandParts) > 0 {
			cmdName := commandParts[0]
			if _, exists := b.commandRegistry.GetCommand(cmdName); exists {
				return nil
			}
			return b.handleHelpCommand(ctx, telego.Update{Message: &message})
		}
		return b.handleHelpCommand(ctx, telego.Update{Message: &message})
	}

	return b.handleHelpCommand(ctx, telego.Update{Message: &message})
}

func (b *Bot) handleStatusCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	isSubscribed := b.subscriptionMgr.IsSubscribed(chatID)

	statusMessage := b.formatter.FormatSubscriptionStatus(isSubscribed)
	return b.client.SendMessageHTML(chatID, statusMessage)
}

func (b *Bot) sendAlreadySubscribed(chatID int64) error {
	msg := "Уведомления уже включены. Чтобы изменить regex, сначала остановите бота: /stop."
	if err := b.client.SendMessageHTML(chatID, msg); err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
	}
	return nil
}

func (b *Bot) sendActivationMessage(chatID int64) error {
	msg := "<b>Бот активирован!</b>\n\nВы будете получать уведомления о логах. Используйте /stop для отписки."
	if err := b.client.SendMessageHTML(chatID, msg); err != nil {
		log.Printf("Failed to send activation message to chat %d: %v", chatID, err)
	}
	return nil
}

func (b *Bot) sendNotSubscribed(chatID int64) error {
	msg := "Вы не получаете уведомления о логах."
	if err := b.client.SendMessageHTML(chatID, msg); err != nil {
		log.Printf("Failed to send message to chat %d: %v", chatID, err)
	}
	return nil
}

func (b *Bot) sendUnsubscribeMessage(chatID int64) error {
	msg := "<b>Вы отписались!</b>\n\nВы больше не будете получать уведомления о логах. Используйте /start для повторной активации."
	if err := b.client.SendMessageHTML(chatID, msg); err != nil {
		log.Printf("Failed to send unsubscribe message to chat %d: %v", chatID, err)
	}
	return nil
}

func (b *Bot) sendErrorResponse(chatID int64, operation string, err error) {
	log.Printf("Error in %s for chat %d: %v", operation, chatID, err)
	errorMessage := "Произошла ошибка при обработке запроса. Пожалуйста, попробуйте позже."
	if sendErr := b.client.SendMessageHTML(chatID, errorMessage); sendErr != nil {
		log.Printf("Failed to send error message to chat %d: %v", chatID, sendErr)
	}
}

func (b *Bot) handleHelpCommand(ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	helpText := b.formatter.FormatHelp(b.commandRegistry.GetAllCommands())

	if err := b.client.SendMessageHTML(chatID, helpText); err != nil {
		log.Printf("Failed to send help message to chat %d: %v", chatID, err)
	}
	return nil
}

func (b *Bot) handleStopCommand(ctx *th.Context, update telego.Update) error {
	chatID := update.Message.Chat.ID

	if !b.subscriptionMgr.IsSubscribed(chatID) {
		return b.sendNotSubscribed(chatID)
	}

	if err := b.subscriptionMgr.RemoveChat(chatID); err != nil {
		log.Printf("Failed to remove chat %d: %v", chatID, err)
		b.sendErrorResponse(chatID, "unsubscription", err)
		return nil
	}

	log.Printf("Chat %d removed via /stop command", chatID)
	return b.sendUnsubscribeMessage(chatID)
}

func (b *Bot) handleBatchCommand(_ *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID

	current := b.batchManager.IsEnabled(chatID)
	next := !current

	if b.db != nil {
		if err := b.db.SetChatBatchEnabled(chatID, next); err != nil {
			b.sendErrorResponse(chatID, "batch toggle", err)
			return nil
		}
	}

	b.batchManager.SetEnabled(chatID, next)

	if next {
		msg := "<b>Батчинг включен</b>\n\nТеперь бот будет объединять несколько логов в одно сообщение."
		if !b.subscriptionMgr.IsSubscribed(chatID) {
			msg += "\n\n<i>Вы сейчас не подписаны на логи. Используйте /start чтобы включить уведомления.</i>"
		}
		if err := b.client.SendMessageHTML(chatID, msg); err != nil {
			log.Printf("Failed to send batch status to chat %d: %v", chatID, err)
		}
	} else {
		msg := "<b>Батчинг выключен</b>\n\nТеперь бот будет присылать каждый лог отдельным сообщением."
		if !b.subscriptionMgr.IsSubscribed(chatID) {
			msg += "\n\n<i>Вы сейчас не подписаны на логи. Используйте /start чтобы включить уведомления.</i>"
		}
		if err := b.client.SendMessageHTML(chatID, msg); err != nil {
			log.Printf("Failed to send batch status to chat %d: %v", chatID, err)
		}
	}

	return nil
}

func (b *Bot) SendMessageHTML(chatID int64, text string) error {
	return b.client.SendMessageHTML(chatID, text)
}

func (b *Bot) Stop() {
	b.cancel()
	log.Println("Telegram bot stopped")
}

func (b *Bot) SubscriberCount() int {
	return b.subscriptionMgr.SubscriberCount()
}

func (b *Bot) IsChatSubscribed(chatID int64) bool {
	return b.subscriptionMgr.IsSubscribed(chatID)
}

func (b *Bot) SendLog(entry parser.LogEntry) error {
	msg := b.formatter.FormatLogEntry(entry)

	subscribers := b.subscriptionMgr.GetAllSubscribers()
	var lastErr error
	for _, chatID := range subscribers {
		if b.regexManager != nil && !b.regexManager.ShouldSend(chatID, entry.Raw) {
			continue
		}
		if b.batchManager != nil {
			if err := b.batchManager.Enqueue(chatID, msg); err != nil {
				lastErr = err
				log.Printf("Failed to enqueue/send batch for chat %d: %v", chatID, err)
			}
			continue
		}

		if err := b.client.SendMessageHTML(chatID, msg); err != nil {
			lastErr = err
			log.Printf("Failed to send message to chat %d: %v", chatID, err)
		}
	}
	return lastErr
}
