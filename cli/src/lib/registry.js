/**
 * Command leaf registry — source of truth for schema / help / auth / risk.
 *
 * Each entry is a top-level command or a dotted leaf (e.g. tokens.create).
 */

/**
 * @typedef {"none"|"base"|"full"} AuthLevel
 * @typedef {"read"|"write"|"high-risk-write"} RiskLevel
 *
 * @typedef {object} LeafMeta
 * @property {string} name
 * @property {string} description
 * @property {AuthLevel} auth
 * @property {RiskLevel} risk
 * @property {boolean} dryRun
 * @property {{ name: string, type: string, required?: boolean, description?: string }[]} [flags]
 * @property {string[]} [examples]
 */

/** @type {LeafMeta[]} */
const LEAVES = [
  {
    name: "models",
    description: "List models available to the current user",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux models"],
  },
  {
    name: "voices",
    description: "Search and list available voice timbres and IDs",
    auth: "none",
    risk: "read",
    dryRun: false,
    flags: [
      { name: "--query", type: "string", description: "Filter by name, display name, voice ID, or tag" },
      { name: "--scene", type: "string", description: "Filter by scene/category (e.g. 电商带货, 角色扮演)" },
      { name: "--language", type: "string", description: "Filter by language (e.g. 中文, 英文)" },
      { name: "--model", type: "string", description: "Filter by gateway model (e.g. seed-audio-1.0)" },
      { name: "--limit", type: "number", description: "Limit number of results returned" },
    ],
    examples: [
      "omnimux voices",
      "omnimux voices --query 顾姐",
      "omnimux voices --scene 电商带货",
      "omnimux voices --model seed-audio-1.0",
    ],
  },
  {
    name: "pricing",
    description: "Public model price list (no access token)",
    auth: "base",
    risk: "read",
    dryRun: false,
    examples: ["omnimux pricing"],
  },
  {
    name: "groups",
    description: "List user groups and ratios",
    auth: "full",
    risk: "read",
    dryRun: false,
  },
  {
    name: "balance",
    description: "Show account balance and usage",
    auth: "full",
    risk: "read",
    dryRun: false,
  },
  {
    name: "tokens",
    description: "List API tokens (keys always masked)",
    auth: "full",
    risk: "read",
    dryRun: false,
  },
  {
    name: "tokens.create",
    description: "Create an API token (key never shown)",
    auth: "full",
    risk: "write",
    dryRun: true,
    flags: [
      { name: "group", type: "string", description: "Token group" },
      { name: "quota", type: "number", description: "Quota in USD" },
    ],
    examples: ["omnimux tokens create my-app --group=default"],
  },
  {
    name: "tokens.switch-group",
    description: "Move a token to another group",
    auth: "full",
    risk: "write",
    dryRun: true,
  },
  {
    name: "tokens.copy",
    description: "Copy the real key to the clipboard (never shown)",
    auth: "full",
    risk: "write",
    dryRun: false,
  },
  {
    name: "tokens.apply",
    description: "Inject the real key at a placeholder in a file",
    auth: "full",
    risk: "high-risk-write",
    dryRun: true,
  },
  {
    name: "tokens.exec",
    description: "Run a command with a token placeholder substituted",
    auth: "full",
    risk: "high-risk-write",
    dryRun: false,
  },
  {
    name: "login",
    description: "Device-code browser login (stores access token)",
    auth: "base",
    risk: "write",
    dryRun: false,
    flags: [
      { name: "no-open", type: "boolean" },
      { name: "no-wait", type: "boolean", description: "Return flow_id without polling" },
      { name: "resume", type: "string", description: "Resume pending flow_id" },
    ],
    examples: [
      "omnimux login --no-wait",
      "omnimux login --resume <flow_id>",
    ],
  },
  {
    name: "login.status",
    description: "Show login/config status without secrets",
    auth: "none",
    risk: "read",
    dryRun: false,
    flags: [{ name: "verify", type: "boolean", description: "Call API to verify token" }],
  },
  {
    name: "logout",
    description: "Clear stored access token and user_id",
    auth: "none",
    risk: "write",
    dryRun: false,
  },
  {
    name: "config",
    description: "View or set base_url, user_id, access_token, auto_update",
    auth: "none",
    risk: "write",
    dryRun: false,
  },
  {
    name: "scan-config",
    description: "Inspect a config file with secrets redacted (best-effort)",
    auth: "none",
    risk: "read",
    dryRun: false,
  },
  {
    name: "update",
    description: "Self-update CLI; sync registered skills",
    auth: "none",
    risk: "high-risk-write",
    dryRun: true,
    flags: [
      { name: "check", type: "boolean" },
      { name: "force", type: "boolean" },
    ],
  },
  {
    name: "skill",
    description: "Install, update, status, read for AI editor skills",
    auth: "none",
    risk: "write",
    dryRun: false,
  },
  {
    name: "skill.read",
    description: "Read package SKILL.md for a skill id (npm/source installs)",
    auth: "none",
    risk: "read",
    dryRun: false,
    examples: ["omnimux skill read omnimux"],
  },
  {
    name: "social",
    description: "Social Ops: connect accounts, upload media, publish",
    auth: "full",
    risk: "write",
    dryRun: false,
    flags: [{ name: "provider", type: "string", description: "Required for accounts/connect/disconnect/post/post-status: tiktok_direct | zernio" }],
    examples: ["omnimux social accounts --provider=tiktok_direct", "omnimux social accounts --provider=zernio"],
  },
  {
    name: "hosted",
    description: "Hosted distribution channels: status, channels, brand, publish, metrics",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted status", "omnimux hosted channels"],
  },
  {
    name: "hosted.status",
    description: "Show hosted distribution gateway readiness and billing mode",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted status"],
  },
  {
    name: "hosted.skus",
    description: "List available distribution channel SKUs, pricing, and stock",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted skus"],
  },
  {
    name: "hosted.channels",
    description: "List owned distribution channels with status, readiness, and expiry",
    auth: "full",
    risk: "read",
    dryRun: false,
    flags: [{ name: "status", type: "string", description: "Filter by channel status (e.g. ready, active, warming_up)" }],
    examples: ["omnimux hosted channels", "omnimux hosted channels --status=ready"],
  },
  {
    name: "hosted.channel",
    description: "Get detailed status and brand configuration of a distribution channel",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted channel 1"],
  },
  {
    name: "hosted.brand",
    description: "Configure channel branding (nickname, avatar, bio, landing site URL, keywords)",
    auth: "full",
    risk: "write",
    dryRun: true,
    flags: [
      { name: "nickname", type: "string", description: "Display nickname" },
      { name: "avatar", type: "string", description: "Avatar image URL" },
      { name: "bio", type: "string", description: "Bio profile text" },
      { name: "site", type: "string", description: "Bio link / destination landing page URL" },
      { name: "keywords", type: "string", description: "Comma-separated warmup/niche keywords" },
    ],
    examples: ["omnimux hosted brand 1 --bio='Official Brand Channel' --site='https://example.com'"],
  },
  {
    name: "hosted.post",
    description: "Submit content distribution task to a single channel",
    auth: "full",
    risk: "write",
    dryRun: true,
    flags: [
      { name: "channel", type: "number", required: true, description: "Channel ID" },
      { name: "media", type: "string", description: "Pre-hosted media URL" },
      { name: "file", type: "string", description: "Local video/image file path (auto-uploaded)" },
      { name: "caption", type: "string", description: "Post caption and hashtags" },
      { name: "scheduled-for", type: "string", description: "Scheduled Unix timestamp or ISO string" },
      { name: "idempotency-key", type: "string", description: "Client idempotency key" },
    ],
    examples: [
      "omnimux hosted post --channel=1 --media=https://cdn.example.com/v.mp4 --caption='New Drop #viral'",
      "omnimux hosted post --channel=1 --file=./promo.mp4 --caption='Exclusive launch'",
    ],
  },
  {
    name: "hosted.batch",
    description: "Batch distribute same content across multiple ready channels",
    auth: "full",
    risk: "write",
    dryRun: true,
    flags: [
      { name: "channels", type: "string", required: true, description: "Comma-separated channel IDs" },
      { name: "media", type: "string", description: "Pre-hosted media URL" },
      { name: "file", type: "string", description: "Local video/image file path (auto-uploaded)" },
      { name: "caption", type: "string", description: "Post caption" },
      { name: "scheduled-for", type: "string", description: "Scheduled Unix timestamp or ISO string" },
      { name: "idempotency-key", type: "string", description: "Client idempotency key" },
    ],
    examples: ["omnimux hosted batch --channels=1,2,3 --file=./promo.mp4 --caption='Matrix launch'"],
  },
  {
    name: "hosted.tasks",
    description: "List task history for a distribution channel",
    auth: "full",
    risk: "read",
    dryRun: false,
    flags: [{ name: "channel", type: "number", required: true, description: "Channel ID" }],
    examples: ["omnimux hosted tasks --channel=1"],
  },
  {
    name: "hosted.task",
    description: "Get task execution detail, error message, and distribution metrics receipt",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted task 10"],
  },
  {
    name: "hosted.cancel",
    description: "Cancel a pending or queued distribution task",
    auth: "full",
    risk: "write",
    dryRun: true,
    examples: ["omnimux hosted cancel 10"],
  },
  {
    name: "hosted.upload",
    description: "Presign and upload local media asset to storage",
    auth: "full",
    risk: "write",
    dryRun: true,
    flags: [{ name: "content-type", type: "string", description: "Explicit MIME content type" }],
    examples: ["omnimux hosted upload ./video.mp4"],
  },
  {
    name: "hosted.buy",
    description: "Order new distribution channel seats (wallet subscription)",
    auth: "full",
    risk: "high-risk-write",
    dryRun: true,
    flags: [
      { name: "sku", type: "number", description: "SKU ID (default: 1 for TikTok US)" },
      { name: "quantity", type: "number", description: "Number of channel seats (default: 1)" },
      { name: "idempotency-key", type: "string", description: "Order idempotency key" },
    ],
    examples: ["omnimux hosted buy --sku=1 --quantity=2 --yes"],
  },
  {
    name: "hosted.addon",
    description: "Add seats to existing subscription with proration (quote or confirm)",
    auth: "full",
    risk: "high-risk-write",
    dryRun: true,
    flags: [
      { name: "sku", type: "number", description: "SKU ID" },
      { name: "quantity", type: "number", description: "Number of additional channel seats" },
      { name: "confirm", type: "boolean", description: "Confirm and charge wallet (omit to quote only)" },
      { name: "idempotency-key", type: "string", description: "Order idempotency key" },
    ],
    examples: [
      "omnimux hosted addon --sku=1 --quantity=1",
      "omnimux hosted addon --sku=1 --quantity=1 --confirm --yes",
    ],
  },
  {
    name: "hosted.renew",
    description: "Renew entire subscription for another billing cycle",
    auth: "full",
    risk: "high-risk-write",
    dryRun: true,
    flags: [
      { name: "sku", type: "number", description: "SKU ID" },
      { name: "idempotency-key", type: "string", description: "Order idempotency key" },
    ],
    examples: ["omnimux hosted renew --sku=1 --yes"],
  },
  {
    name: "hosted.order",
    description: "Get fulfillment progress and assigned channels for an order",
    auth: "full",
    risk: "read",
    dryRun: false,
    examples: ["omnimux hosted order 1"],
  },
  {
    name: "schema",
    description: "Machine-readable command catalog",
    auth: "none",
    risk: "read",
    dryRun: false,
    examples: ["omnimux schema", "omnimux schema tokens.create"],
  },
  {
    name: "help",
    description: "Usage overview (JSON envelope)",
    auth: "none",
    risk: "read",
    dryRun: false,
  },
];

// Creation commands reuse the account login; all billable submissions are explicit.
const influencerFlag = (name, type, description, required = false) => ({ name, type, description, ...(required ? { required: true } : {}) });
const creationCommon = [
  influencerFlag("--request-id", "string", "Stable 8-128 character creation identity; retain it for recovery"),
  influencerFlag("--input", "file", "JSON product fields; cannot overlap explicit creation fields"),
];
const paging = [influencerFlag("--page", "integer", "Page number (default 1)"), influencerFlag("--page-size", "integer", "Items per page, 1-100 (default 20)")];
LEAVES.push(
  { name: "influencer.options", description: "Read current character choices, media limits, video pairs and creation availability", auth: "full", risk: "read", dryRun: false, examples: ["omnimux influencer options"] },
  { name: "influencer.character.create", description: "Create a character sheet using account credits; returns acceptance, not delivered media", auth: "full", risk: "high-risk-write", dryRun: true, flags: [
    ...creationCommon, influencerFlag("--type", "string", "normal (default), freak, total, cats, dogs"),
    influencerFlag("--brief", "string", "Character description, maximum 4000 characters"),
    influencerFlag("--selection", "json", "Category-to-option-ID-array object from influencer options"),
    influencerFlag("--reference-image", "file-or-https", "Optional style image, maximum 5 MiB for local files"),
  ], examples: ["omnimux influencer character create --brief 'blue coat' --request-id character-001 --dry-run", "omnimux influencer character create --brief 'blue coat' --request-id character-001 --yes"] },
  { name: "influencer.character.list", description: "List your successful original reusable character sheets", auth: "full", risk: "read", dryRun: false, flags: paging, examples: ["omnimux influencer character list --page 1 --page-size 20"] },
  { name: "influencer.character.get", description: "Read one owned original character and its multiview relations", auth: "full", risk: "read", dryRun: false, examples: ["omnimux influencer character get <task_id>"] },
  { name: "influencer.character.multiview", description: "Generate an optional multiview sheet from a successful original character using account credits", auth: "full", risk: "high-risk-write", dryRun: true, flags: creationCommon, examples: ["omnimux influencer character multiview <task_id> --request-id views-001 --yes"] },
  { name: "influencer.video.create", description: "Recreate reference-video motion with a character; consumes account credits", auth: "full", risk: "high-risk-write", dryRun: true, flags: [
    ...creationCommon, influencerFlag("--character", "string", "Owned successful original character task ID"),
    influencerFlag("--image", "file-or-https[]", "Repeat in order; first image is the main subject without --character; 1-30 total images, local maximum 5 MiB each"),
    influencerFlag("--video", "file-or-https", "Reference video, maximum 100 MiB locally; server inspects true 4-30 second duration"),
    influencerFlag("--video-task", "string", "Owned successful video task instead of --video"),
    influencerFlag("--mode", "string", "motion (default) or replace; replace keeps source aspect"),
    influencerFlag("--resolution", "string", "480p (default) or 720p"),
    influencerFlag("--aspect-ratio", "string", "480p:16:9,9:16,3:4; 720p:16:9,9:16; omitted is resolved from source by server"),
    influencerFlag("--prompt", "string", "Optional supplementary description, maximum 4000 characters"),
  ], examples: ["omnimux influencer video create --character <task_id> --video reference.mp4 --request-id video-001 --dry-run", "omnimux influencer video create --character <task_id> --video reference.mp4 --request-id video-001 --yes"] },
  { name: "influencer.task.list", description: "List your character, multiview and video generation history", auth: "full", risk: "read", dryRun: false, flags: [...paging, influencerFlag("--kind", "string", "character, multiview or video"), influencerFlag("--status", "string", "Normalized task status")], examples: ["omnimux influencer task list --kind video --status running"] },
  { name: "influencer.task.get", description: "Read an owned generation task, including failed or pending tasks", auth: "full", risk: "read", dryRun: false, examples: ["omnimux influencer task get <task_id>"] },
  { name: "influencer.task.wait", description: "Wait for actual generation success without resubmitting; timeout does not cancel the cloud task", auth: "full", risk: "read", dryRun: false, flags: [influencerFlag("--timeout", "integer", "Maximum wait in seconds, default 600; 1-86400")], examples: ["omnimux influencer task wait <task_id> --timeout 600"] },
  { name: "influencer.task.download", description: "Stream a verified owned artifact to a local file; signed links and credentials are never printed", auth: "full", risk: "read", dryRun: false, flags: [influencerFlag("--output", "file", "Destination file; parent directory must exist", true), influencerFlag("--overwrite", "boolean", "Explicitly permit replacing an existing file")], examples: ["omnimux influencer task download <task_id> --output ./result.mp4"] },
  { name: "influencer.request.get", description: "Recover the original generation by client request ID; never starts a new task", auth: "full", risk: "read", dryRun: false, examples: ["omnimux influencer request get video-001"] },
);

/**
 * Top-level command → auth level for bin.js (coarse).
 * Nested leaves may require finer resolution later.
 */
function topLevelAuth(name) {
  const leaf = LEAVES.find((l) => l.name === name);
  if (leaf) return leaf.auth;
  // Nested only: use parent prefix
  const parent = LEAVES.find((l) => l.name.startsWith(name + "."));
  return parent ? parent.auth : "none";
}

/**
 * @param {string} [name]
 * @returns {LeafMeta|LeafMeta[]|null}
 */
function getLeaf(name) {
  if (!name) return null;
  return LEAVES.find((l) => l.name === name) || null;
}

/**
 * @returns {LeafMeta[]}
 */
function listLeaves() {
  return LEAVES.slice();
}

/**
 * Schema payload for agents.
 * @param {string} [name]
 */
function schemaPayload(name) {
  if (!name) {
    return {
      contract_version: require("./errors").CONTRACT_VERSION,
      commands: LEAVES.map((l) => ({
        name: l.name,
        description: l.description,
        auth: l.auth,
        risk: l.risk,
        dry_run: Boolean(l.dryRun),
        flags: l.flags || [],
        examples: l.examples || [],
      })),
    };
  }
  const leaf = getLeaf(name);
  if (!leaf) return null;
  return {
    contract_version: require("./errors").CONTRACT_VERSION,
    command: {
      name: leaf.name,
      description: leaf.description,
      auth: leaf.auth,
      risk: leaf.risk,
      dry_run: Boolean(leaf.dryRun),
      flags: leaf.flags || [],
      examples: leaf.examples || [],
    },
  };
}

module.exports = {
  LEAVES,
  listLeaves,
  getLeaf,
  topLevelAuth,
  schemaPayload,
};
