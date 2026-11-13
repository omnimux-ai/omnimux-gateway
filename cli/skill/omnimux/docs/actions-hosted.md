# Hosted Distribution Channels (`omnimux hosted`)

This is a legacy command reference, not a current product offer. OmniMux's consumer focus is AI character generation and reference-video recreation. Hosted distribution is not for sale; command or endpoint existence does not prove purchasable SKUs, enabled service or authorization to manage accounts. Preserve the command contracts below for existing integrations, but verify readiness and permissions against the configured instance before any action. Repository scope: [brand authority](../../../../docs/brand/README.md); distributed packs must not depend on that repository-only link.

---

## Output Contract

All `omnimux hosted` commands return the standard Agent RPC JSON envelopes:

- **Success (stdout)**:
  ```json
  {
    "ok": true,
    "data": { ... },
    "meta": { "command": "hosted.post", "cli_version": "...", "contract_version": 1 }
  }
  ```
- **Error (stderr)**:
  ```json
  {
    "ok": false,
    "error": {
      "type": "api",
      "subtype": "channel_not_ready",
      "message": "channel not ready for distribution",
      "retryable": false
    },
    "meta": { "command": "hosted.post", "cli_version": "...", "contract_version": 1 }
  }
  ```

---

## Available Subcommands

### 1. Service Status & SKUs

```bash
# Check gateway readiness and SKU defaults
omnimux hosted status

# List purchasable channel SKUs, pricing ($99/mo), and stock
omnimux hosted skus
```

### 2. Channels Management

```bash
# List all owned channels
omnimux hosted channels

# List channels filtered by status (ready, warming_up, active, suspended)
omnimux hosted channels --status=ready

# Get detailed info for a single channel
omnimux hosted channel <channel_id>
```

### 3. Channel Branding & Bio Link

Configure the public persona of the channel to direct audience traffic to the user's landing page or shop:

```bash
omnimux hosted brand <channel_id> \
  --nickname="OmniBrand US" \
  --avatar="https://cdn.example.com/logo.png" \
  --bio="Automated Social AI Tools 🚀" \
  --site="https://yourbrand.com/landing" \
  --keywords="ai,saas,growth"
```

### 4. Content Distribution (Single & Batch)

> **Important**: Distribution requires the channel to be in `ready` or `active` status. Submitting to a `warming_up` channel will be rejected with HTTP 409 `CHANNEL_NOT_READY`.

```bash
# Distribute using an already-hosted media URL
omnimux hosted post \
  --channel=1 \
  --media="https://cdn.example.com/video.mp4" \
  --caption="Introducing our new product! #viral #innovation"

# Distribute using a local video/image file (CLI handles presign + upload automatically)
omnimux hosted post \
  --channel=1 \
  --file="./output/launch-teaser.mp4" \
  --caption="Big announcement tomorrow 🔥"

# Batch distribute to multiple ready channels (Matrix Distribution)
omnimux hosted batch \
  --channels=1,2,3 \
  --file="./output/campaign.mp4" \
  --caption="Global campaign drop #trending"
```

### 5. Task Tracking & Metrics Receipt

```bash
# List tasks on a channel
omnimux hosted tasks --channel=1

# Get single task detail and view counts/engagement metrics
omnimux hosted task <task_id>

# Cancel a pending/queued task
omnimux hosted cancel <task_id>
```

### 6. Media Upload

Upload media independently to obtain a durable resource URL:

```bash
omnimux hosted upload ./promo.mp4
```

### 7. Ordering & Subscriptions (High-Risk Write)

Purchasing and renewing channel seats charge your wallet balance ($99/seat/mo). These commands require explicit confirmation via `--yes` or dry-run preview via `--dry-run`:

```bash
# Preview order cost without charging
omnimux hosted buy --sku=1 --quantity=2 --dry-run

# Confirm order
omnimux hosted buy --sku=1 --quantity=2 --yes

# Addon seats to existing subscription (Proration quote)
omnimux hosted addon --sku=1 --quantity=1

# Confirm addon seats
omnimux hosted addon --sku=1 --quantity=1 --confirm --yes

# Renew subscription
omnimux hosted renew --sku=1 --yes

# Check order delivery status
omnimux hosted order <order_id>
```
