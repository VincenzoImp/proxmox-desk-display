# Docker Compose

From the repository root:

```bash
cp examples/config.example.yaml config.yaml
cp examples/.env.example .env
```

Edit `config.yaml` and `.env`, then start:

```bash
docker compose -f examples/docker-compose.yaml up -d --build
```

Open:

```text
http://localhost:8765
```

Check health:

```bash
curl http://localhost:8765/healthz
```

Read display state:

```bash
curl -H "Authorization: Bearer $DISPLAY_TOKEN" \
  http://localhost:8765/api/v1/display-state
```
