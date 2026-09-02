# mistral-sticky

[English](README.md) | [中文](README.zh.md)

[![ci](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml/badge.svg)](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml)

Pin a conversation to one Mistral API key so [prefix cache](https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching) can hit.

```
clients  →  New API (billing, tokens)
                →  mistral-sticky  →  api.mistral.ai
                       pick key by session
```

It is a reverse proxy, not a gateway. No UI, no quota, no protocol translation beyond what Mistral's Chat Completions API needs.

Public image (linux/amd64 and linux/arm64), no login required to pull:

```bash
docker pull ghcr.io/im594/mistral-sticky:latest
```

## Why

Mistral caches the shared prefix of `messages` **per API key**. New API (and similar relays) often store hundreds of keys on one channel in `random` mode. Adjacent turns land on different keys, so `cache_tokens` stays near zero even on 10k+ token agent traces.

This proxy:

1. Authenticates with a single `PROXY_TOKEN` (what New API stores as the channel key)
2. Derives a session id from `prompt_cache_key` / `conversation_id` / metadata, else `sha256(model + first system + first user)`
3. Picks `hash % N` from `keys.txt` (append-only; never reorder)
4. Writes a stable `prompt_cache_key` upstream
5. Maps tool-call ids to Mistral's 9-character alphabet deterministically
6. Drops OpenAI-only fields (`reasoning_effort: medium`, `stream_options`, …)

On 401/403 the key is cooled for 30 days and the session fails over. **429 does not rotate** — that would kill the cache.

## Docker

The image is distroless and contains **only the binary**. Keys are mounted at runtime. Never bake `keys.txt` into a layer.

```bash
cp .env.example .env                 # set PROXY_TOKEN
mkdir -p data
cp keys.example.txt data/keys.txt    # real keys, never commit
printf '{"entries":[]}\n' > data/cooldown.json
# distroless runs as uid 65532
sudo chown -R 65532:65532 data
docker compose up -d
```

`docker-compose.yml` pulls `ghcr.io/im594/mistral-sticky:latest`. Pin a release with `ghcr.io/im594/mistral-sticky:v0.1.1` if you do not want `latest`.

If New API is on another Compose network, use `docker-compose.external-network.yml` and set the Mistral channel `base_url` to `http://mistral-sticky:8080`.

Without Compose:

```bash
docker run --rm \
  -p 127.0.0.1:8080:8080 \
  --env-file .env \
  -e LISTEN=:8080 \
  -e KEYS_FILE=/data/keys.txt \
  -e COOLDOWN_FILE=/data/cooldown.json \
  -v "$PWD/data:/data" \
  ghcr.io/im594/mistral-sticky:latest
```

## Run from source

```bash
cp .env.example .env
mkdir -p data
cp keys.example.txt data/keys.txt
printf '{"entries":[]}\n' > data/cooldown.json
go test ./...
go run ./cmd/mistral-sticky
```

## New API

Channel type stays **Mistral (42)**. Then:

| Setting | Value |
|---|---|
| `base_url` | `http://mistral-sticky:8080` |
| Key | one `PROXY_TOKEN`, not the pool of Mistral keys |
| Multi-key | off |
| Auto ban | off |
| `pass_through_body_enabled` | **on** (so New API does not randomize tool-call ids each turn) |

Give this channel higher priority than other Mistral channels for the same model, or traffic will split and pinning is wasted.

## Env

See `.env.example`.

| Variable | Meaning |
|---|---|
| `PROXY_TOKEN` | Shared secret from New API `Authorization` |
| `KEYS_FILE` | One Mistral key per line |
| `COOLDOWN_FILE` | Disabled indexes + expiry only — never store raw keys there |
| `UPSTREAM` | Default `https://api.mistral.ai` |
| `LISTEN` | Default `:8080` |

Logs may include `key_index` and a short `session_fp`. They must never include the key string or `Authorization`.

## Not in scope

- Becoming New API / LiteLLM / CLIProxyAPI
- Redis session tables (restart rebinds from the same hash)
- Polling keys (worse for cache than random)

## License

MIT
