# mistral-sticky

[English](README.md) | [中文](README.zh.md)

[![ci](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml/badge.svg)](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml)

mistral-sticky 是面向 [Mistral API](https://docs.mistral.ai/) 的粘性 key 池。客户端使用一把 `PROXY_TOKEN` 鉴权。每段对话被固定到 `keys.txt` 中的一把 key，并转发至 `api.mistral.ai`。

Mistral 的 [前缀缓存](https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching) 按 API key 生效。命中的 prompt token 按输入价的 10% 计费。若中继在每轮请求上随机更换 key，缓存无法命中。

```
client  →  [optional relay]  →  mistral-sticky  →  api.mistral.ai
```

```bash
docker pull ghcr.io/im594/mistral-sticky:latest
```

提供 linux/amd64 与 linux/arm64。密钥在运行时挂载，不得写入镜像。

## 特性

- 按会话哈希选择 key（对 `keys.txt` 做 `hash % N`）
- 向上游写入稳定的 `prompt_cache_key`
- 将 tool-call id 确定性映射为 Mistral 要求的 9 位字母数字
- 去除 OpenAI 专有字段（`reasoning_effort: medium`、`stream_options` 等）
- 401/403：将该 key 冷却 30 天
- 429：保持同一把 key

## 示例

同一把 key 上的 5 轮 agent 对话：

| 轮次 | prompt tokens | `cache_tokens` |
| ---: | ------------: | -------------: |
| 1    |         14584 |              0 |
| 2    |         14608 |          14464 |
| 3    |         14677 |          14592 |
| 4    |         14796 |          14592 |
| 5    |         14854 |          14720 |

第 2–5 轮约 79% 的 prompt 来自缓存。

同一主机上，500 把 key 随机选取、24 小时：706 次请求覆盖 245 把 key，相邻请求复用上一把 key 为 3 / 705，`cache_tokens` 占 prompt 的 5.4%。

## 安装

```bash
cp .env.example .env
mkdir -p data
cp keys.example.txt data/keys.txt
printf '{"entries":[]}\n' > data/cooldown.json
sudo chown -R 65532:65532 data
docker compose up -d
```

在 `.env` 中设置 `PROXY_TOKEN`。将官方 Mistral key 按行写入 `data/keys.txt`。这两个文件均不要提交。镜像以 uid 65532 运行。

默认监听 `127.0.0.1:8080`。固定版本请使用 `ghcr.io/im594/mistral-sticky:v0.1.2`。`keys.txt` 只允许追加，不要重排或删除中间行。

若需加入已有 Docker 网络，修改 `docker-compose.external-network.yml` 中的网络名：

```bash
docker compose -f docker-compose.yml -f docker-compose.external-network.yml up -d
```

## 使用

```bash
curl -s http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer $PROXY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"model":"mistral-small-latest","messages":[{"role":"user","content":"ping"}]}'
```

任意 Chat Completions 客户端可将此服务作为 Mistral 的 base URL。若使用中继（New API、LiteLLM、one-api 等），将其 Mistral 上游设为 `http://mistral-sticky:8080`，渠道中只保存 `PROXY_TOKEN`。官方 key 放在 sticky 的 `keys.txt` 中。

若中继会改写请求 body（尤其是 tool-call id），需开启 body 透传，否则每轮前缀都会变化。

### New API

渠道类型为 Mistral (42)。`base_url` = `http://mistral-sticky:8080`。仅填写 `PROXY_TOKEN`。关闭多 key 与自动禁用。打开 `pass_through_body_enabled`。同一模型的其他 Mistral 渠道请降低优先级。

用量列表可能不显示折扣（`cache_ratio` 为 1）。请在日志详情中查看 `cache_tokens`。

## 给 AI agent 的说明

将以下内容复制给 AI agent：

```
将 https://github.com/IM594/mistral-sticky 部署到当前机器。

mistral-sticky 是 Mistral key 池。客户端用 PROXY_TOKEN 鉴权，官方 API key 放在 data/keys.txt。按会话 hash 选择 key，同一段对话使用同一把 key，以便命中 Mistral 前缀缓存（命中按输入价 10% 计费）。镜像：ghcr.io/im594/mistral-sticky:latest（linux/amd64、linux/arm64）。不要将 keys.txt 或 .env 写入镜像或提交到 git。

按仓库中的 docker-compose.yml 操作：
1. 复制 .env.example 为 .env，将 PROXY_TOKEN 设为随机密钥。
2. mkdir -p data；官方 key 一行一把写入 data/keys.txt；printf '{"entries":[]}\n' > data/cooldown.json。
3. chown -R 65532:65532 data（镜像以 uid 65532 运行）。
4. docker compose up -d。默认绑定 127.0.0.1:8080。
5. 将任意 Chat Completions 客户端指向该服务，Authorization 为 Bearer PROXY_TOKEN。若已有中继，在同一 Docker 网络上将其中 Mistral 上游设为 http://mistral-sticky:8080，渠道中只放 PROXY_TOKEN。若中继改写 body（tool_call id），开启透传。
6. keys.txt 只追加。不要在 429 时换 key。日志可包含 key_index 与 session_fp，禁止记录 key 原文或 Authorization。

详见 README.zh.md。完成后用 curl 请求 /healthz 以及一次 /v1/chat/completions。
```

## 配置

| 变量 | 说明 |
|---|---|
| `PROXY_TOKEN` | 进站 Bearer 密钥 |
| `KEYS_FILE` | 官方 key，每行一把（默认 `/data/keys.txt`） |
| `COOLDOWN_FILE` | 仅保存下标与过期时间 |
| `UPSTREAM` | 默认 `https://api.mistral.ai` |
| `LISTEN` | 默认 `:8080` |

参见 `.env.example`。

## 从源码构建

```bash
go test ./...
go run ./cmd/mistral-sticky
```

## License

MIT
