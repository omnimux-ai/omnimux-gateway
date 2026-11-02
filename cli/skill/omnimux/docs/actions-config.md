# Actions: `scan-config` and `tokens apply`

## Action: `scan-config`

Inspect a config file's structure with best-effort secret redaction. Useful for understanding config layout without exposing secrets, but not guaranteed to catch every sensitive value in every file format.

Usage: `omnimux scan-config <file_path>`

```bash
omnimux scan-config ~/.config/some-app/config.json
```

Default output is JSON `{ "path", "content", "redaction": "best-effort" }`; `--pretty` prints the sanitized content verbatim. Redactions (marked `<REDACTED>`):

- `sk-xxx` token patterns
- `Bearer` tokens
- Credential-bearing connection strings such as `user:pass@host`
- Values of fields with sensitive names (password, apiKey, secret, token, credential, auth, private_key, access_key, client_secret, etc.)

Supports JSON, YAML, ENV, TOML, and similar key-value formats. Non-sensitive fields are shown as-is.

> **Disclaimer:** `scan-config` provides a best-effort structural view for safer inspection. It is not a formal parser and is not guaranteed to catch every sensitive value in every file format. Treat it as a risk-reduction measure, not an absolute security guarantee.

---

## Action: `tokens apply`

Apply a token's real key to any config file securely. The key never enters the AI conversation context.

Usage: `omnimux tokens apply <token_id> <file_path>`

### Workflow

**Step 1 — Scan the target file** (MANDATORY if the file already exists):

```bash
omnimux scan-config <file_path>
```

Use this sanitized output to understand the file structure. **NEVER read the target config file directly** — it may contain existing real keys or other secrets. If the file does not exist yet, skip the scan and create it from scratch.

**Step 2 — Write the placeholder**:

Edit the file and set the API key field to `__OMNIMUX_TOKEN_{token_id}__`. For example:

- JSON: `"apiKey": "__OMNIMUX_TOKEN_42__"`
- YAML: `apiKey: __OMNIMUX_TOKEN_42__`
- ENV:  `OPENAI_API_KEY=__OMNIMUX_TOKEN_42__`
- TOML: `api_key = "__OMNIMUX_TOKEN_42__"`

Also set the base URL field if needed — use the configured instance URL (`omnimux config get base_url`) with `/v1` appended (e.g., `https://api.example.com/v1`). The base URL is NOT a secret and can be written directly.

**Step 3 — Apply the real key**:

```bash
omnimux tokens apply 42 <file_path>
```

The CLI fetches the real key in memory, replaces every `__OMNIMUX_TOKEN_42__` occurrence, and atomically replaces the file (temp file + rename, original mode preserved). On failure the original file is left untouched.

**Step 4 — Confirm** to the user that the token has been configured.

### Security rules

- **NEVER** read the target file directly with Read/cat/etc. Always use `scan-config`.
- **NEVER** write any `sk-` value into the file yourself. Only the placeholder is allowed.
- **NEVER** read the file after applying the key to verify it — trust the CLI's JSON success result.
- The placeholder format is strictly `__OMNIMUX_TOKEN_{token_id}__` — no variations.
