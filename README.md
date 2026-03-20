# k8s-pager

Autonomous Kubernetes event watcher with AI-powered diagnostics. Monitors pod crashes, OOM kills, failed deployments, Flux reconciliation failures, and more. When a threshold is reached, an AI agent investigates the root cause using Kubernetes APIs and delivers the diagnosis to your preferred channel — Slack, Telegram, or WhatsApp — so your team or downstream AI agents can act on it immediately.

## How it works

1. Watches Kubernetes events via the `events.k8s.io/v1` API
2. Counts events per resource using a sliding window (default: 5 events in 10 minutes)
3. When threshold is hit, an AI agent investigates using tools like `describe_pod`, `get_pod_logs`, `get_events`, and `get_resource`
4. Sends an alert with the event details and root cause analysis to configured channels (Slack, Telegram, WhatsApp)

## Monitored events

| Category | Events |
|---|---|
| Pod | `Failed`, `BackOff`, `OOMKilled`, `OOMKilling`, `Unhealthy`, `FailedScheduling` |
| Node/System | `EvictionThresholdMet`, `SystemOOM` |
| Volume | `FailedAttachVolume`, `FailedMount` |
| Jobs | `FailedCreate`, `DeadlineExceeded`, `BackoffLimitExceeded` |
| Flux Helm | `InstallFailed`, `UpgradeFailed`, `RollbackFailed`, `TestFailed` |
| Flux Kustomize | `BuildFailed`, `HealthCheckFailed`, `ReconciliationFailed`, `ArtifactFailed`, `PruneFailed` |
| Flux Source | `GitOperationFailed`, `AuthenticationFailed`, `IndexationFailed` |

Configurable via `EVENT_REASONS` env var.

## Quick start

### Local development

```bash
export KUBECONFIG=~/.kube/config
export LLM_API_KEY=your-api-key
go run .
```

Without any notifier configured, alerts are printed to logs.

### Deploy to Kubernetes

```bash
# Create namespace and RBAC
kubectl apply -f deploy/manifests/namespace.yaml
kubectl apply -f deploy/manifests/rbac.yaml

# Create secrets
kubectl create secret generic k8s-pager \
  --namespace k8s-pager \
  --from-literal=slack-webhook-url=https://hooks.slack.com/services/YOUR/WEBHOOK \
  --from-literal=llm-api-key=your-api-key

# Deploy
kubectl apply -f deploy/manifests/deployment.yaml
```

### Docker

```bash
docker pull ghcr.io/nevalla/k8s-pager:latest
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `LLM_API_KEY` | required | API key for the LLM provider |
| `LLM_PROVIDER` | `anthropic` | `anthropic` or `openai` |
| `LLM_MODEL` | `claude-haiku-4-5-20251001` | Model to use for diagnosis |
| `LLM_BASE_URL` | provider default | Custom endpoint (for Gemini, Azure OpenAI, Ollama, etc.) |
| `SLACK_WEBHOOK_URL` | (none) | Slack incoming webhook URL |
| `SLACK_MENTION` | (none) | Slack user/bot ID to mention in alerts (e.g., `U12345678`) |
| `TELEGRAM_BOT_TOKEN` | (none) | Telegram bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | (none) | Telegram chat/group ID to send alerts to |
| `TELEGRAM_MENTION` | (none) | Mention to prepend in group chats (e.g., `@openclaw`) |
| `WHATSAPP_API_URL` | (none) | WhatsApp Business Cloud API base URL |
| `WHATSAPP_API_TOKEN` | (none) | WhatsApp API bearer token |
| `WHATSAPP_RECIPIENT` | (none) | WhatsApp recipient phone number |
| `WHATSAPP_MENTION` | (none) | Mention to prepend in group chats (e.g., `@openclaw`) |
| `CLUSTER_NAME` | from kubeconfig | Cluster identifier shown in alerts |
| `WATCH_NAMESPACE` | all namespaces | Restrict to a single namespace |
| `EVENT_REASONS` | see above | Comma-separated list of event reasons to watch |
| `THRESHOLD` | `5` | Number of events before alerting |
| `WINDOW_SIZE` | `10m` | Sliding window for counting events |
| `COOLDOWN` | `1h` | Suppress duplicate alerts for this duration |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `KUBECONFIG` | in-cluster | Path to kubeconfig for local development |

### Notification channels

Multiple notifiers can be active simultaneously. If none are configured, alerts are printed to logs.

```bash
# Slack
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK
export SLACK_MENTION=U12345678  # optional: tag a user or bot

# Telegram
export TELEGRAM_BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
export TELEGRAM_CHAT_ID=-1001234567890
export TELEGRAM_MENTION=@openclaw  # optional: trigger bot in group chats

# WhatsApp (Business Cloud API)
export WHATSAPP_API_URL=https://graph.facebook.com/v21.0/PHONE_NUMBER_ID
export WHATSAPP_API_TOKEN=your-token
export WHATSAPP_RECIPIENT=15551234567
export WHATSAPP_MENTION=@openclaw  # optional: trigger bot in group chats
```

### Using different LLM providers

```bash
# Anthropic (default)
export LLM_PROVIDER=anthropic
export LLM_API_KEY=sk-ant-...
export LLM_MODEL=claude-haiku-4-5-20251001

# OpenAI
export LLM_PROVIDER=openai
export LLM_API_KEY=sk-...
export LLM_MODEL=gpt-4o-mini

# Gemini (via OpenAI-compatible endpoint)
export LLM_PROVIDER=openai
export LLM_API_KEY=your-gemini-key
export LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai
export LLM_MODEL=gemini-2.0-flash
```

## License

MIT
