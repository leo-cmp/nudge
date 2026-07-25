# Nudge

Sistema leve de lembretes via **Telegram**, acionado por agentes de IA através do protocolo **MCP** (Model Context Protocol) com transporte **SSE** (Server-Sent Events).

Quando você está codando e pede pra IA _"me avisa daqui 30 min"_ ou _"me lembra amanhã às 9h de revisar o PR"_, o agente chama a API MCP do Nudge, que agenda a notificação e dispara no seu Telegram no momento certo.

---

## Funcionalidades

| Tipo | Descrição |
|------|-----------|
| `instant` | Disparo imediato (ex: "me avisa quando o build terminar") |
| `scheduled` | Data/hora fixa no futuro (ISO-8601) |
| `recurring` | Repetitivo via cron (`@daily`, `0 9 * * 1`, etc.) |

### Tools MCP expostas

- **`create_reminder`** — Cria lembrete (message, type, scheduled_at?, cron_pattern?)
- **`list_reminders`** — Lista lembretes por status (pending/sent/cancelled/all)
- **`cancel_reminder`** — Cancela um lembrete pelo ID

---

## Stack

| Camada | Tecnologia |
|--------|-----------|
| Linguagem | Go 1.22 |
| Banco | SQLite (embedded, pure-Go via `modernc.org/sqlite`) |
| Transporte | HTTP + SSE (MCP 2024-11-05) |
| Notificação | Telegram Bot API |
| Container | Docker multi-stage (Alpine) |
| CI/CD | GitHub Actions → GHCR |
| Proxy | Traefik (SSL/TLS automático) |

---

## Configuração

Variáveis de ambiente (`.env`):

```env
PORT=8080
MCP_AUTH_TOKEN=seu-token-secreto
TELEGRAM_BOT_TOKEN=123:abc
TELEGRAM_DEFAULT_CHAT_ID=123456789
DATABASE_URL=/data/nudge.db
```

### Obtendo credenciais Telegram

1. Crie um bot com [@BotFather](https://t.me/BotFather) — ele te dá o `TELEGRAM_BOT_TOKEN`
2. Para obter seu `CHAT_ID`, mande qualquer mensagem pro bot e acesse:
   ```
   https://api.telegram.org/bot<TOKEN>/getUpdates
   ```
   O `chat.id` no JSON de resposta é o valor que você precisa.

---

## Rodando com Docker

```bash
# Build local
docker build -t nudge .

# Rodar
docker run -d \
  --name nudge \
  -p 8080:8080 \
  -v nudge-data:/data \
  --env-file .env \
  nudge
```

### docker-compose (com Traefik)

```yaml
services:
  nudge:
    image: ghcr.io/leo-cmp/nudge:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - MCP_AUTH_TOKEN=${MCP_AUTH_TOKEN}
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - TELEGRAM_DEFAULT_CHAT_ID=${TELEGRAM_DEFAULT_CHAT_ID}
      - DATABASE_URL=/data/nudge.db
    volumes:
      - nudge-data:/data
    networks:
      - traefik-public
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.nudge.rule=Host(`nudge.seu-dominio.com`)"
      - "traefik.http.routers.nudge.entrypoints=websecure"
      - "traefik.http.routers.nudge.tls=true"
      - "traefik.http.routers.nudge.tls.certresolver=myresolver"
      - "traefik.http.services.nudge.loadbalancer.server.port=8080"

volumes:
  nudge-data:

networks:
  traefik-public:
    external: true
```

---

## CI/CD

No push pra `main`, o GitHub Actions:

1. Builda a imagem Docker (multi-stage, Alpine)
2. Pusha pra `ghcr.io/leo-cmp/nudge:latest`
3. (A VPS puxa a imagem via Portainer/stack)

---

## Estrutura do Projeto

```
.
├── cmd/nudge/main.go          # Entrypoint
├── internal/
│   ├── config/config.go       # Variáveis de ambiente
│   ├── db/
│   │   ├── db.go              # Conexão SQLite + migração
│   │   ├── reminders.go       # CRUD de lembretes
│   │   └── reminders_test.go  # Testes unitários
│   ├── mcp/
│   │   ├── server.go          # HTTP + SSE + JSON-RPC
│   │   └── tools.go           # Handlers das tools MCP
│   ├── scheduler/scheduler.go # Loop de verificação (15s)
│   └── telegram/notifier.go   # Cliente Telegram Bot API
├── Dockerfile
├── docker-compose.yml
├── .github/workflows/deploy.yml
└── docs/specs/                # Documento de design
```

---

## Health Check

```bash
curl http://localhost:8080/health
# → OK
```

## Testando com um cliente MCP

Configure seu agente IA (Claude Desktop, Cursor, etc.) com o MCP server:

```json
{
  "mcpServers": {
    "nudge": {
      "url": "https://nudge.seu-dominio.com/sse",
      "headers": {
        "Authorization": "Bearer seu-token-secreto"
      }
    }
  }
}
```

---

## Licença

MIT
