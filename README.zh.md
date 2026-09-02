# mistral-sticky

[English](README.md) | [中文](README.zh.md)

[![ci](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml/badge.svg)](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml)

把同一段对话钉在一把 Mistral API key 上，让 [前缀缓存](https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching) 能命中。

```
客户端  →  New API（计费、token）
                →  mistral-sticky  →  api.mistral.ai
                       按会话选 key
```

带会话粘性的 Mistral key 池：进站一把 `PROXY_TOKEN`，出站换成 `keys.txt` 里的一把，同一段对话钉在同一把 key 上。没有 UI、没有额度。只做 Mistral Chat Completions 必需的清洗。

公开镜像（linux/amd64 与 linux/arm64），拉取不需要登录：

```bash
docker pull ghcr.io/im594/mistral-sticky:latest
```

## 为什么需要它

Mistral 按 **API key** 缓存 `messages` 的公共前缀。New API 一类中继经常在一个渠道里塞几百把 key，模式还是 `random`。相邻两轮落到不同账号上，即使用 1 万 token 的 agent 对话，`cache_tokens` 也接近 0。

它会：

1. 用一把 `PROXY_TOKEN` 鉴权（New API 渠道里只存这一把）
2. 从 `prompt_cache_key` / `conversation_id` / metadata 取会话，否则 `sha256(model + 第一条 system + 第一条 user)`
3. 从 `keys.txt` 里 `hash % N` 选下标（只追加，不要重排或删中间行）
4. 向上游写入稳定的 `prompt_cache_key`
5. 把 tool-call id 确定性映成 Mistral 要求的 9 位字母数字
6. 丢掉 Mistral 不认的 OpenAI 字段（例如 `reasoning_effort: medium`、`stream_options`）

401/403 会把该 key 冷却 30 天并换一把。**429 不换 key**，否则缓存会被自己打掉。

## Docker

镜像是 distroless，**只有二进制**。密钥运行时挂载。不要把 `keys.txt` 打进镜像层。

```bash
cp .env.example .env                 # 设置 PROXY_TOKEN
mkdir -p data
cp keys.example.txt data/keys.txt    # 真 key，永远不要提交
printf '{"entries":[]}\n' > data/cooldown.json
# distroless 以 uid 65532 运行
sudo chown -R 65532:65532 data
docker compose up -d
```

`docker-compose.yml` 拉取 `ghcr.io/im594/mistral-sticky:latest`。不想跟 `latest` 走就钉死 `ghcr.io/im594/mistral-sticky:v0.1.2`。

如果 New API 在另一个 Compose 网络里，用 `docker-compose.external-network.yml`，渠道 `base_url` 设成 `http://mistral-sticky:8080`。

不用 Compose：

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

## 从源码跑

```bash
cp .env.example .env
mkdir -p data
cp keys.example.txt data/keys.txt
printf '{"entries":[]}\n' > data/cooldown.json
go test ./...
go run ./cmd/mistral-sticky
```

## New API

渠道类型仍是 **Mistral (42)**，然后：

| 设置 | 值 |
|---|---|
| `base_url` | `http://mistral-sticky:8080` |
| Key | 一把 `PROXY_TOKEN`，不要填那一池 Mistral key |
| 多 key | 关 |
| 自动禁用 | 关 |
| `pass_through_body_enabled` | **开**（否则 New API 每轮随机改 tool-call id） |

同一模型如果还有别的 Mistral 渠道，把这个渠道优先级调高，否则流量会分流，粘性无效。

## 环境变量

见 `.env.example`。

| 变量 | 含义 |
|---|---|
| `PROXY_TOKEN` | New API `Authorization` 带来的共享密钥 |
| `KEYS_FILE` | 每行一把 Mistral key |
| `COOLDOWN_FILE` | 只存下标和过期时间，不要写 key 原文 |
| `UPSTREAM` | 默认 `https://api.mistral.ai` |
| `LISTEN` | 默认 `:8080` |

日志可以有 `key_index` 和短的 `session_fp`。禁止出现 key 字符串或 `Authorization`。

## 明确不做

- 做成 New API / LiteLLM / CLIProxyAPI
- Redis 会话表（重启后用同一套哈希重新绑定）
- 轮询 key（比 random 更伤缓存）

## License

MIT
