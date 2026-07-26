# Nudge

Servidor MCP que cria lembretes via Telegram. Seu agente de IA agenda notificações e você recebe no celular.

## Instalação

```json
{
  "mcpServers": {
    "nudge": {
      "command": "npx",
      "args": ["-y", "@leo-cmp/nudge-mcp"],
      "env": {
        "TELEGRAM_BOT_TOKEN": "123:abc",
        "TELEGRAM_DEFAULT_CHAT_ID": "123456789"
      }
    }
  }
}
```

### Credenciais Telegram

1. Crie um bot com [@BotFather](https://t.me/BotFather) — você recebe o `TELEGRAM_BOT_TOKEN`
2. Mande qualquer mensagem pro bot e acesse `https://api.telegram.org/bot<TOKEN>/getUpdates` para obter o `chat.id`

## Tools MCP

| Tool | Descrição |
|------|-----------|
| `create_reminder` | Cria lembrete instantâneo, agendado ou recorrente |
| `list_reminders` | Lista lembretes por status |
| `cancel_reminder` | Cancela um lembrete pelo ID |

## Tipos de lembrete

| Tipo | Exemplo |
|------|---------|
| `instant` | "me avisa quando o build terminar" |
| `scheduled` | "me lembra amanhã às 9h de revisar o PR" |
| `recurring` | "me lembra toda segunda de checar créditos" |

## Stack

Go + SQLite + Telegram Bot API. Transporte MCP via stdio (npx) ou SSE (Docker).

## Docker

```bash
docker run -d --name nudge \
  -p 8080:8080 \
  -v nudge-data:/data \
  -e TELEGRAM_BOT_TOKEN=... \
  -e TELEGRAM_DEFAULT_CHAT_ID=... \
  ghcr.io/leo-cmp/nudge:latest
```
