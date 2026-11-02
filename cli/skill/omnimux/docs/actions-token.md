# Actions: token management

Token commands manage the **current user's own** API tokens. Keys are always masked in output; the real key is only ever moved by `tokens copy` / `tokens apply` / `tokens exec`.

---

## Action: `tokens`

List API tokens.

```bash
omnimux tokens
```

Display as a table: ID, Name, Key (masked, e.g. `sk-reHR**********OspA`), Status (1=enabled, 2=disabled, 3=expired), Group, Created Time, Expired Time, Remain Quota.

---

## Action: `tokens create`

Usage: `omnimux tokens create <name> [--group=<group>] [--quota=<usd>]`

- `<name>` — required, the token name
- `--group=<group>` — optional, assign to a specific group
- `--quota=<usd>` — optional, quota limit in dollars (converted to raw quota × 500000); omit for unlimited

```bash
omnimux tokens create my-app --group=default --quota=10
```

The create response intentionally omits the real key. **Do NOT make any follow-up call to retrieve the key.** Report success (name/ID only) and tell the user they can securely use the key via:

- `omnimux tokens copy <id>` — clipboard
- `omnimux tokens apply <id> <file>` — config file
- `omnimux tokens exec <id> -- <command>` — CLI command

The output always includes a `token_id` field: taken from the server response when available, otherwise resolved by the CLI through a follow-up **key-masked** list call matching the new token's name (newest match wins). If it cannot be resolved, `token_id` is `null` — run `omnimux tokens` to find the id in the list.

---

## Action: `tokens switch-group`

Usage: `omnimux tokens switch-group <token_id> <group>`

```bash
omnimux tokens switch-group 42 vip
```

The CLI fetches the token, updates only its group, and confirms. Report the new group on success.

---

## Action: `tokens copy`

Usage: `omnimux tokens copy <token_id>`

Retrieves the real key and copies it **directly to the system clipboard** through a child-process pipe. The key never appears in stdout, stderr, logs, or any AI-readable output.

```bash
omnimux tokens copy 42
```

Output is only a JSON success marker: `{"success":true,"token_id":42,"copied_to_clipboard":true}`.

**Mandatory constraints (see Security Guidelines in SKILL.md):**

- Do **NOT** capture, parse, or inspect the copy operation beyond its JSON result.
- Do **NOT** read clipboard contents afterward.
- Do **NOT** wrap this command in any pipeline that could expose the key.
