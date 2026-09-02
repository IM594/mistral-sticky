# mistral-sticky

This GitHub repository is **public**. Treat every file, log, image layer, and CI output as world-readable.

## Secrets

- Never put real Mistral keys, `PROXY_TOKEN`, or cooldown payloads that contain keys into source, tests, commits, Dockerfiles, GHCR layers, issues, or chat.
- Real material lives only on the server: `keys.txt`, `.env`, `cooldown.json` (index + expiry only; still do not paste it into the repo).
- Git only has `keys.example.txt` and `.env.example` with obvious placeholders.
- Logs may include `key_index` and a short `session_fp`. Never log the key string, Authorization header, or full session text.
- Do not `docker build` with `keys.txt` COPY. Mount at runtime. Do not `git add -f` ignored secret files.
