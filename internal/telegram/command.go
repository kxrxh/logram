package telegram

import (
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type Command struct {
	Name        string
	Description string
	Handler     func(ctx *th.Context, update telego.Update) error
	Aliases     []string
}

type CommandRegistry struct {
	commands       map[string]Command
	commandAliases map[string]string
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:       make(map[string]Command),
		commandAliases: make(map[string]string),
	}
}

func (r *CommandRegistry) Register(
	name, description string,
	handler func(ctx *th.Context, update telego.Update) error,
	aliases ...string,
) {
	cmd := Command{
		Name:        name,
		Description: description,
		Handler:     handler,
		Aliases:     aliases,
	}
	r.commands[name] = cmd
	for _, alias := range aliases {
		r.commandAliases[alias] = name
	}
}

func (r *CommandRegistry) GetCommand(name string) (Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

func (r *CommandRegistry) GetCommandByAlias(alias string) (Command, bool) {
	actualName, exists := r.commandAliases[alias]
	if !exists {
		return Command{}, false
	}
	return r.GetCommand(actualName)
}

func (r *CommandRegistry) GetAllCommands() map[string]Command {
	return r.commands
}

func (r *CommandRegistry) GetAllAliases() map[string]string {
	return r.commandAliases
}
