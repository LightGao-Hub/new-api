---
name: new-api-glm-billing-context
description: Continue the LightGao-Hub/new-api GLM token billing work. Use when Codex needs context about the glm-5.1/glm-5.2 configurable token billing tiers, related tests, GitHub branch, local Docker deployment, or production blue/green deployment discussion from June 28, 2026.
---

# New API GLM Billing Context

## Current Branch

Work on `glm-token-billing-multiplier` in `https://github.com/LightGao-Hub/new-api`.

The branch contains:
- `bdc7e943 Add GLM token billing multiplier`
- `99952fc5 Make GLM token billing tiers configurable`
- a later change that raises every default GLM tier multiplier by `0.1`

Do not assume `main` has these changes unless the user says it was merged.

## Billing Behavior

The billing adjustment is transparent, configurable, and limited to matching model names.

Default matched models:
- `glm-5.1`
- `glm-5.2`

Default tiers after the latest requested change:

```text
raw token count <= 200      => 1.0x
raw token count > 200       => 1.2x
raw token count > 1000      => 1.35x
raw token count > 10000     => 1.4x
raw token count > 20000     => 1.5x
raw token count > 50000     => 1.6x
raw token count > 70000     => 1.7x
```

Thresholds are strict `>`. Example: `1000` uses the `>200` tier, while `1001` uses the `>1000` tier.

Cache read tokens do not trigger the GLM billing tier and are not multiplied. This was changed after real testing showed short prompts with large cache hits, e.g. raw prompt `16077`, cached `16028`, completion `221`, were incorrectly entering the `>10000` tier. The multiplier tier now uses non-cache input plus output tokens, so that example is judged as `270` tokens and enters the `>200` tier after the latest tier change.

Runtime default config path is `token_billing_tiers.json` from the process working directory. In the Docker image, `WORKDIR` is `/data`, so the default runtime file is:

```text
/data/token_billing_tiers.json
```

The path can be overridden with:

```text
TOKEN_BILLING_CONFIG_PATH
```

Config supports a hot-reload switch:

```json
{
  "enabled": true,
  "reload_interval_seconds": 15,
  "rules": []
}
```

`enabled` defaults to `true`. When set to `false`, all GLM billing multipliers return `1.0`, so internal billing/logs and returned downstream `usage` are not adjusted. The config is kept in memory and checked at most once every 15 seconds by file mtime; it is not read on every request. Invalid JSON keeps the previous valid config.

## Key Files

Implementation:
- `service/glm_token_multiplier.go`
- `service/token_counter.go`
- `service/text_quota.go`
- `relay/channel/openai/usage.go`
- `relay/channel/openai/relay-openai.go`
- `relay/channel/openai/helper.go`
- `relay/channel/openai/chat_via_responses.go`
- `relay/channel/openai/responses_via_chat.go`
- `relay/channel/openai/relay_responses.go`

Tests:
- `service/glm_token_multiplier_test.go`
- `service/text_quota_test.go`
- `relay/channel/openai/usage_multiplier_test.go`

Config example:
- `config/token_billing_tiers.example.json`

Local Docker override from earlier work:
- `docker-compose.local.yml`

Deployment notes:
- `docs/installation/glm-token-billing-deployment.zh-CN.md`

## Validation Commands

Run focused tests first:

```powershell
go test ./service -run 'TestApplyTokenBillingMultiplier|TestCalculateTextQuotaSummaryAppliesGLM51And52TokenBillingTier|TestCalculateTextQuotaSummaryDoesNotAdjustGLM51And52TokensAtOrBelow200|TestCalculateTextQuotaSummaryDoesNotAdjustGLMEstimateFallbackTwice' -count=1 -v
```

Full service tests may still fail on an unrelated existing test:

```text
TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty
expected 1, actual 3
```

Treat that as pre-existing unless code in channel affinity was changed.

## Demo Expectations

For Chinese demo text, a previous temporary test used `TokenTypeTextNumber`, so raw token count equaled Chinese character count. After the latest +0.1 change, expected values are:

```text
glm-5.2 raw=200     => billed=200
glm-5.2 raw=500     => billed=600
glm-5.2 raw=600     => billed=720
glm-5.2 raw=1000    => billed=1200
glm-5.2 raw=1200    => billed=1620
glm-5.2 raw=12000   => billed=16800
glm-5.2 raw=25000   => billed=37500
glm-5.2 raw=60000   => billed=96000
glm-5.2 raw=120000  => billed=204000
glm-5.2 raw=240000  => billed=408000
glm-4.6 raw=240000  => billed=240000
```

## Downstream Usage Alignment

The branch now also adjusts the `usage` returned to clients on OpenAI-compatible response paths, so a downstream new-api instance that records usage from this API response sees the same token totals that this instance logs and bills.

Important implementation detail: response paths write an adjusted copy of `usage` to the client, but return the raw upstream usage to the settlement path. `PostTextConsumeQuota` still applies the multiplier once when writing quota/logs. This avoids double multiplication.

Covered paths include:
- non-stream `/v1/chat/completions`
- generated final stream usage chunk for `/v1/chat/completions`
- upstream stream chunks that already contain `usage`
- `/v1/responses` direct responses and stream completed events
- responses-to-chat and chat-to-responses compatibility conversions

## Docker Deployment Context

For production build from source:

```bash
git clone -b glm-token-billing-multiplier https://github.com/LightGao-Hub/new-api.git
cd new-api
mkdir -p data
cp config/token_billing_tiers.example.json data/token_billing_tiers.json
docker compose up -d --build
```

For validating a second stack, prefer isolated PostgreSQL and Redis to avoid production state risk:

```text
blue: current production app + postgres + redis, port 3000
green: new app + postgres-green + redis-green, port 3001
```

Do not connect validation green to production Redis/PostgreSQL unless the user explicitly accepts real write, cache, quota, and task-processing risk.

If the user wants near-zero downtime for production, validate on isolated green first, then rebuild only the production `new-api` service against the existing production DB/Redis:

```bash
docker compose up -d --build new-api
```

## Safety Boundary

The user previously asked about making billing changes hard to discover. Do not implement hidden or deceptive billing. Keep billing logic configurable, explainable, and auditable.
