# mistral-sticky

[English](README.md) | [中文](README.zh.md)

[![ci](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml/badge.svg)](https://github.com/IM594/mistral-sticky/actions/workflows/ci.yml)

Mistral 的 [前缀缓存](https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching) 挂在 **API key** 上，打中的 token 按输入价 **10%** 计费。

你有一堆 key、前面再用随机/轮询去抽的话，下一轮经常换号。`cache_tokens` 就会一直是 0。一轮 1.4 万 token 的 agent 请求，每次都按全价付。

sticky 就干一件事：外面一把 `PROXY_TOKEN`，里面 `keys.txt` 里多把官方 key。同一段对话钉在同一把上，再转到 `api.mistral.ai`。谁来调都行——客户端直连、New API、LiteLLM、one-api，只要把 Mistral 的上游指过来。

```
客户端  →  （可选：你现有的中继）  →  sticky  →  api.mistral.ai
```

我们这边 Hermes 钉住之后，同一把 key 连续 5 轮：

| 轮 | prompt | cache_tokens |
|---:|---:|---:|
| 1 | 14584 | 0 |
| 2 | 14608 | 14464 |
| 3 | 14677 | 14592 |
| 4 | 14796 | 14592 |
| 5 | 14854 | 14720 |

后面几轮大概 79% 的 prompt 走缓存。钉住之前 500 把 key 随机抽，24 小时 706 次打到 245 把号，相邻两次同 key 只有 3 次。

401/403 会把那把 key 冷却 30 天再换。429 不换，换了缓存就断。

```bash
docker pull ghcr.io/im594/mistral-sticky:latest
```

amd64 / arm64 都有。密钥运行时挂载，别打进镜像。

## 自己跑

```bash
cp .env.example .env                 # 改 PROXY_TOKEN
mkdir -p data
cp keys.example.txt data/keys.txt    # 真 key，别提交
printf '{"entries":[]}\n' > data/cooldown.json
sudo chown -R 65532:65532 data       # 镜像用 uid 65532
docker compose up -d
```

默认同机 `127.0.0.1:8080`。客户端 `base_url` 指这里，`Authorization: Bearer <PROXY_TOKEN>`。

已经有别的 Docker 网络（中继也在里面）的话，改一下 `docker-compose.external-network.yml` 里的网络名，然后：

```bash
docker compose -f docker-compose.yml -f docker-compose.external-network.yml up -d
```

上游写成 `http://mistral-sticky:8080`。中继渠道里只填 `PROXY_TOKEN`，那一池 Mistral key 放 sticky 的 `keys.txt`。`keys.txt` 只往下追加，别重排或删中间行。

钉版本：`ghcr.io/im594/mistral-sticky:v0.1.2`。

## 丢给 AI agent 去部署

整段复制：

```
把 https://github.com/IM594/mistral-sticky 部署到当前机器。

这是一个 Mistral key 池。对外一把 PROXY_TOKEN，对内 data/keys.txt 里多把官方 API key。按会话 hash 选 key，同一段对话一直用同一把，好打 Mistral 前缀缓存（命中按输入价 10% 计费）。公开镜像：ghcr.io/im594/mistral-sticky:latest（linux/amd64 和 linux/arm64）。不要把 keys.txt / .env 打进镜像或提交进 git。

按仓库里的 docker-compose.yml 跑：
1. 复制 .env.example 为 .env，把 PROXY_TOKEN 设成一段随机密钥。
2. mkdir -p data；官方 key 一行一把写入 data/keys.txt；printf '{"entries":[]}\n' > data/cooldown.json。
3. 镜像以 uid 65532 跑，必须 chown -R 65532:65532 data。
4. docker compose up -d。默认只绑 127.0.0.1:8080。
5. 任何走 Chat Completions 的客户端都可以把 Mistral base URL 指到这个服务，Authorization 用 PROXY_TOKEN。如果前面还有 New API / LiteLLM / one-api 之类的中继，把它们的 Mistral 上游改成 http://mistral-sticky:8080（同一 Docker 网络用服务名），渠道里只放 PROXY_TOKEN，不要再塞那一池官方 key。中继如果会改请求 body（尤其 tool_call id），打开 body 透传，否则每轮前缀都会变，缓存仍打不中。
6. keys.txt 只追加。429 不要当失败去换 key（程序已经不换了）。日志里可以看 key_index 和 session_fp，不要打印 key 原文或 Authorization。

参考 README.zh.md。做完用 curl 打一下 /healthz 和一轮 /v1/chat/completions。
```

## New API 的话

渠道类型 Mistral (42)。`base_url` = `http://mistral-sticky:8080`。key 只填 `PROXY_TOKEN`。多 key、自动禁用关掉。`pass_through_body_enabled` 打开（不然它每轮乱改 tool id）。同一模型别的 Mistral 渠道优先级调低。

列表里可能看不到折扣，Mistral 的 `cache_ratio` 是 1。打开日志详情看 `cache_tokens`。

## 环境变量

`.env.example` 里都有。`PROXY_TOKEN` 是进站密码；`KEYS_FILE` 每行一把官方 key；`COOLDOWN_FILE` 只记下标和过期时间；`UPSTREAM` 默认 `https://api.mistral.ai`。

源码：`go test ./... && go run ./cmd/mistral-sticky`。

MIT
