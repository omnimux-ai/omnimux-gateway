# Actions: `models` / `pricing` / `groups` / `balance`

All four are read-only queries. Output is JSON on stdout; add `--pretty` when presenting to a human.

---

## Action: `models`

List models available to the current user (authenticated).

```bash
omnimux models
```

Present as a table: Model ID, Owner (if present in the payload).

---

## Action: `pricing`

List public model pricing. Requires only a configured `base_url` — no access token.

```bash
omnimux pricing
```

Present columns: Model, Quota type (`0`=ratio, `1`=fixed price), Price / Ratio, Groups, Endpoints (fields as present in the payload).

---

## Action: `groups`

List the groups available to the current user.

```bash
omnimux groups
```

Present the groups (and their ratios, when present) as a list.

---

## Action: `balance`

Show account balance and usage.

```bash
omnimux balance
```

The response includes a `derived` object with convenience fields:

- `derived.quota_usd` — remaining balance in USD (raw `quota` ÷ 500000)
- `derived.used_quota_usd` — used quota in USD

Also present from `data`: username, display name, email, group, request count (when present).

---

## Failure handling

- Exit code `2` → configuration missing. Run `omnimux config`, fix per `docs/setup.md`, then retry. Do not improvise credential sources.
- Exit code `1` with an HTTP message → surface the sanitized message to the user as-is.
