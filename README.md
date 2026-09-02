# mistral-sticky

[English](README.md) | [中文](README.zh.md)

[![ci](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml/badge.svg)](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml)

Mistral [prefix cache](https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching) lives on the **API key**. Hits bill at **10% of input**.

If you have a pile of keys and something in front picks at random, the next turn often gets a different key. `cache_tokens` stays 0. A 14k-token agent call pays for all 14k every time.

sticky does one job: one `PROXY_TOKEN` on the outside, many official keys in `keys.txt`. Same conversation stays on the same key, then it goes to `api.mistral.ai`. Call it directly, or put New API / LiteLLM / one-api in front and point their Mistral upstream here.

```
client  →  (optional: your existing relay)  →  sticky  →  api.mistral.ai
```

A Hermes session after pinning, same key, five turns:

| turn | prompt | cache_tokens |
|---:|---:|---:|
| 1 | 14584 | 0 |
| 2 | 14608 | 14464 |
| 3 | 14677 | 14592 |
| 4 | 14796 | 14592 |
| 5 | 14854 | 14720 |

Later turns reused about 79% of the prompt from cache. Before pinning, 500 keys on random, 24h: 706 requests hit 245 keys; back-to-back same key 3 times.

401/403 cool that key for 30 days. 429 keeps the same key so the cache stays.

```bash
docker pull ghcr.io/im594/mistral-sticky:latest
```

linux/amd64 and linux/arm64. Mount keys at runtime. Do not bake them into the image.

## Run it

```bash
cp .env.example .env                 # set PROXY_TOKEN
mkdir -p data
cp keys.example.txt data/keys.txt    # real keys, never commit
printf '{"entries":[]}\n' > data/cooldown.json
sudo chown -R 65532:65532 data       # image runs as uid 65532
docker compose up -d
```

Listens on `127.0.0.1:8080` by default. Point the client `base_url` here, `Authorization: Bearer <PROXY_TOKEN>`.

If a relay already sits on another Docker network, edit the name in `docker-compose.external-network.yml` and:

```bash
docker compose -f docker-compose.yml -f docker-compose.external-network.yml up -d
```

Upstream is `http://mistral-sticky:8080`. The relay stores only `PROXY_TOKEN`. Official keys stay in sticky's `keys.txt`. Append to that file. Do not reorder or delete lines in the middle.

Pin a release with `ghcr.io/im594/mistral-sticky:v0.1.2`.

## Paste this to an AI agent

Copy the whole block:

```
Deploy https://github.com/IM594/mistral-sticky on this machine.

It is a Mistral key pool. One PROXY_TOKEN on the outside, many official API keys in data/keys.txt. It picks a key by session hash so one conversation stays on one key and Mistral prefix cache can hit (cached tokens bill at 10% of input). Public image: ghcr.io/im594/mistral-sticky:latest (linux/amd64 and linux/arm64). Do not COPY keys.txt or .env into the image or commit them.

Use the repo docker-compose.yml:
1. Copy .env.example to .env and set PROXY_TOKEN to a random secret.
2. mkdir -p data; write one official key per line to data/keys.txt; printf '{"entries":[]}\n' > data/cooldown.json.
3. The image runs as uid 65532, so chown -R 65532:65532 data.
4. docker compose up -d. Default bind is 127.0.0.1:8080.
5. Any Chat Completions client can set the Mistral base URL to this service and Authorization to PROXY_TOKEN. If there is already a relay (New API, LiteLLM, one-api, …), point that relay's Mistral upstream at http://mistral-sticky:8080 (use the Compose service name on a shared Docker network). Put only PROXY_TOKEN in the relay. Keep the official keys in sticky's keys.txt. If the relay rewrites the body (especially tool_call ids), enable body pass-through, or the prefix changes every turn and cache still misses.
6. keys.txt is append-only. Do not rotate on 429 (the binary already keeps the key). Logs may show key_index and session_fp. Never log the raw key or Authorization.

See README.md. When done, curl /healthz and one /v1/chat/completions request.
```

## If you use New API

Channel type Mistral (42). `base_url` = `http://mistral-sticky:8080`. One `PROXY_TOKEN`. Multi-key off, auto-ban off. Turn on `pass_through_body_enabled` or it randomizes tool ids every turn. Lower the priority of other Mistral channels for the same model.

The usage list may not show a discount (`cache_ratio` is 1 for Mistral). Open the log detail and read `cache_tokens`.

## Env

See `.env.example`. `PROXY_TOKEN` is the inbound secret. `KEYS_FILE` is one official key per line. `COOLDOWN_FILE` stores index + expiry only. `UPSTREAM` defaults to `https://api.mistral.ai`.

From source: `go test ./... && go run ./cmd/mistral-sticky`.

MIT
