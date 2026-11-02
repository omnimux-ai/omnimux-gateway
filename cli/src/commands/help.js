/**
 * omnimux help — usage overview (JSON envelope for Agents).
 */

const { listLeaves } = require("../lib/registry");

const HELP_TEXT = `omnimux — user-level CLI for OmniMux (new-api)

Usage: omnimux [--base-url <url>] [--pretty] <command> [args]

Commands:
  models                            List models available to you
  voices [--query=q] [--scene=s]    Search and list available voice timbres and IDs
  pricing                           Public model price list (no token needed)
  groups                            Your groups and ratios
  balance                           Balance and usage
  tokens                            List API tokens (keys always masked)
  tokens create <name> [--group=g] [--quota=usd]
                                    Create a token (key is never shown)
  tokens switch-group <id> <group>  Move a token to another group
  tokens copy <id>                  Copy the real key to the clipboard (never shown)
  tokens apply <id> <file>          Inject the real key at __OMNIMUX_TOKEN_<id>__
  tokens exec <id> [--timeout=sec] -- <cmd...>
                                    Run a command with __OMNIMUX_TOKEN_<id>__ substituted
                                    (child timeout: 300s default)
  login [--no-open]                 Browser device-code login (stores access token)
  login --no-wait                   Start login; return flow_id (Agent split-flow)
  login --resume <flow_id>          Finish pending login
  login status [--verify]           Login/config status (no secrets)
  logout                            Clear stored access token and user_id
  scan-config <file>                Show a config file with secrets redacted (best-effort)
  config [status|get|set|unset]     View/set base_url, user_id, access_token, auto_update
  update [--check] [--force]        Self-update CLI (npm or binary); syncs registered skills
  upgrade                           Alias for update
  skill status|list                 Flat skill catalog + user/project installs + drift
  skill read <id>                   Read package SKILL.md (npm/source; not embedded in binary)
  skill install --user|--project [--mode=link|copy] [--only=id,...]
                                    Install each package skill as .agents/skills/<id> (default link)
  skill update [--user|--project]   Re-sync / migrate legacy whole-tree installs to flat layout
  skill uninstall --user|--project  Remove skill installs for a scope
  social status|accounts|connect|disconnect|media|post|post-status
                                    Social Ops (bind / upload media / publish via OmniMux)
  hosted status|skus|channels|channel|brand|post|batch|tasks|task|cancel|upload|buy|addon|renew|order
                                    Hosted distribution channels (GeeLark cloud accounts)
  influencer options                Character choices, video specifications and availability
  influencer character create|list|get|multiview
                                    Create/reuse characters; optional multiview generation
  influencer video create           Recreate a reference video using your character/images
  influencer task list|get|wait|download
                                    Recover progress and download a completed artifact
  influencer request get <id>       Restore a creation by its stable request ID
  schema [leaf]                     Machine-readable command catalog
  help                              This help

Global flags:
  --base-url <url>   Instance URL (overrides OMNIMUX_BASE_URL and config file)
  --pretty           Human-readable indented JSON
  --no-open          Do not open a browser (login only)
  --no-wait          Login: return immediately with flow_id
  --resume <id>      Login: resume pending flow
  --verify           login status: verify token against API
  --dry-run          Preview side effects when supported (fail-closed otherwise)
  --yes              Confirm high-risk writes
  --check            Report only for update (no install)
  --force            Reinstall latest even if already up to date (update)
  --user / --project Skill install scope
  --mode=link|copy   Skill install mode (link keeps pace with CLI package upgrades)
  --no-auto-update   Skip background self-update for this invocation
  --version          Print version (JSON envelope)

Configuration (priority: flag > env > config file):
  base_url       --base-url | OMNIMUX_BASE_URL | omnimux config set base_url <url>
  user_id        OMNIMUX_USER_ID | omnimux config set user_id <id>
  access_token   omnimux login | OMNIMUX_ACCESS_TOKEN | omnimux config set access_token <token>
                 (stored in ~/.config/omnimux/secrets.json mode 0600 on every platform)
  auto_update    --no-auto-update | OMNIMUX_AUTO_UPDATE | omnimux config set auto_update false
                 (default: true — background self-update at most once per 24h)

Output contract (Agent RPC):
  Success: stdout { "ok": true, "data": ..., "meta": { command, cli_version, contract_version } }
  Error:   stderr { "ok": false, "error": { type, subtype, message, ... }, "meta": ... }
  Branch on ok / error.type / exit code — never on message text.
  See cli/docs/ERROR_CONTRACT.md and omnimux schema.
  sk- keys are never printed to stdout/stderr by any command.
`;

async function run(ctx) {
  const commands = listLeaves().map((l) => ({
    name: l.name,
    description: l.description,
    auth: l.auth,
    risk: l.risk,
  }));
  const out = (ctx && ctx.out) || null;
  const payload = {
    usage: HELP_TEXT,
    commands,
  };
  if (out) out.ok(payload);
  else {
    const { printJson } = require("../lib/output");
    printJson(payload, (ctx && ctx.flags) || {});
  }
}

module.exports = { run, HELP_TEXT };
