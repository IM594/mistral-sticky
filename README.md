# mistral-sticky

Thin reverse proxy in front of `https://api.mistral.ai`. It pins a conversation to one upstream API key with consistent hashing so Mistral prefix cache can hit. The GitHub repository is **public** — never commit real keys.

## Run locally

```bash
cp .env.example .env          # set PROXY_TOKEN
mkdir -p data
cp keys.example.txt data/keys.txt   # replace with real keys on the server only
printf '{"entries":[]}\n' > data/cooldown.json
go test ./...
go run ./cmd/mistral-sticky
```

```bash
docker compose build
# on oraclearm2, also join 1panel-network:
# docker compose -f docker-compose.yml -f docker-compose.server.yml up -d
```

The image copies only the binary. Mount `data/keys.txt` at runtime.

## Env

See `.env.example`. `PROXY_TOKEN` is what New API stores as the channel key. Upstream Mistral keys stay in `data/keys.txt` (one per line, never reorder).
