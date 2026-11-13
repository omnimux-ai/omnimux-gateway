# Gate 0 — Output / side-effect inventory (scaffold baseline)

Recorded on branch `cli/agent-native-scaffold` against `origin/main` tip before Agent RPC land.

## Success / error paths (after scaffold)

| Path | Stream | Shape |
|------|--------|--------|
| Most commands | stdout | `{ok,data,meta}` |
| Failures | stderr | `{ok,false,error,meta}` |
| `--version` | stdout | envelope `data.version` |
| `help` | stdout | envelope `data.usage` + `data.commands` |
| `tokens exec` post-spawn | child passthrough | child exit (exception) |

## Side effects (risk order TBD Sprint D)

| Action | Side effect | Notes |
|--------|-------------|-------|
| `maybeAutoUpdate` | may upgrade CLI | skipped when `--dry-run` |
| `login` | writes keychain/secrets + user_id | pending flow files under config dir |
| `logout` | deletes token | |
| `tokens create/apply/exec/copy` | API / file / clipboard / child | |
| `skill install/update/uninstall` | filesystem under `.agents/skills` | |
| `config set/unset` | config.json / secrets | |
| `social *` | API + media upload | |
| `update` | npm/binary install | |

## Distribution

| Channel | skills/ | docs/ |
|---------|---------|-------|
| npm | yes | yes (added to `files`) |
| compiled binary | **not embedded** | not embedded |

## Field migration for Agents

Top-level business fields (`token_id`, `derived`, …) → under **`data`**.
