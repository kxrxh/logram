package telegram

import (
	"context"
	"log"
	"os"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type errorOnlyLogger struct{}

func (l *errorOnlyLogger) Debugf(format string, args ...interface{}) {
}

func (l *errorOnlyLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[TELEGRAM ERROR] "+format, args...)
}

type Client struct {
	client *telego.Bot
	ctx    context.Context
}

func NewClient(token string) (*Client, error) {
	var options []telego.BotOption

	if os.Getenv("TELEGRAM_DEBUG") == "true" {
		options = append(options, telego.WithDefaultDebugLogger())
	} else {
		options = append(options, telego.WithLogger(&errorOnlyLogger{}))
	}

	client, err := telego.NewBot(token, options...)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
		ctx:    context.Background(),
	}, nil
}

func (c *Client) GetMe() (*telego.User, error) {
	return c.client.GetMe(c.ctx)
}

func (c *Client) SendMessageHTML(chatID int64, text string) error {
	_, err := c.client.SendMessage(c.ctx, tu.Message(
		tu.ID(chatID),
		text,
	).WithParseMode("HTML"))
	return err
}

func (c *Client) Updates() (<-chan telego.Update, error) {
	return c.client.UpdatesViaLongPolling(c.ctx, nil)
}

func (c *Client) Bot() *telego.Bot {
	return c.client
}
