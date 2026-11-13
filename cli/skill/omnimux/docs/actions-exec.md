# Action: `tokens exec`

Execute a shell command with a token key securely substituted via placeholder replacement. The real key never enters the AI conversation context.

Usage: `omnimux tokens exec <token_id> [--timeout=<seconds>] -- <command...>`

- `--timeout=<seconds>` — optional, child-process timeout in seconds (default: 300). Raise it for long-running calls such as media generation; must be a positive number.

## How it works

The AI constructs the full command but uses `__OMNIMUX_TOKEN_{token_id}__` wherever the real key would go. The CLI replaces the placeholder with the real key, executes the command in a child shell, and sanitizes the child output before returning it.

```bash
omnimux tokens exec 42 -- some-app config set provider.openai.apiKey __OMNIMUX_TOKEN_42__
```

The CLI will:

1. Fetch the real key for token 42 from the instance (in memory, never printed)
2. Replace `__OMNIMUX_TOKEN_42__` with the real key in the command string
3. Execute the command in a child shell
4. Sanitize stdout/stderr to redact any accidental key echoes (`sk-` tokens, Bearer tokens, sensitive field values, connection strings)
5. Forward the sanitized output and propagate the child command's exit code

Note: unlike other commands, `tokens exec` passes the (sanitized) child output through directly instead of wrapping it in JSON — so scripts chaining off the child command keep working.

## When to use `tokens exec` vs `tokens apply`

- Use **`tokens exec`** when the target app expects the key via a CLI command (e.g. `app config set key <value>`)
- Use **`tokens apply`** when the target app reads the key from a config file (e.g. `.env`, `config.json`, `config.yaml`)

## Security rules

- **NEVER** construct the command with a real `sk-` key. Only the placeholder is allowed.
- **NEVER** try to extract the real key from the sanitized command output.
- The placeholder format is strictly `__OMNIMUX_TOKEN_{token_id}__` — no variations.
- Trust the sanitized output and exit code instead of re-running the command to verify.
