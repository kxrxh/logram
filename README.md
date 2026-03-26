## logram

Tails a log file, filters lines with regex rules, and sends them to Telegram chats.
It formats messages as Telegram HTML (log text is escaped), and can batch logs per chat to avoid spam.

## Run

1. Set `config.yaml` (see `example.config.yaml`)
2. Start: `go run ./cmd/app`

`CONFIG_PATH` can override the config file path.

## Telegram commands (per chat)

`/start`, `/stop`, `/help`, `/status`  
Regex: `/regexes`, `/addregex`, `/resetregex`, `/removeregex`  
Batch toggle: `/batch`

