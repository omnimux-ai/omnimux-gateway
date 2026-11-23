/**
 * Social Ops commands for official direct publishing via OmniMux Social Ops API.
 *
 * Routes: /api/social/v1/*
 * Uses OmniMux user credentials only; official direct provider: tiktok_direct.
 */

const fs = require("fs");
const path = require("path");

const { printJson, UsageError } = require("../lib/output");
const { openUrl } = require("../lib/open-url");
const { putBinary, guessContentType, DEFAULT_MAX_BYTES } = require("../lib/upload");

const OFFICIAL_PROVIDER = "tiktok_direct";
const RETIRED_ZERNIO_MSG =
  'provider "zernio" is retired; migrate to the official TikTok direct provider (--provider tiktok_direct) and reconnect via: omnimux social connect --provider tiktok_direct --platform tiktok';

function parseBoolFlagValue(flagName, raw) {
  const s = String(raw).trim().toLowerCase();
  if (s === "true" || s === "1") return true;
  if (s === "false" || s === "0") return false;
  throw new UsageError(`invalid boolean value for ${flagName}: ${raw}`);
}

function consumeOptionalBool(flagName, args, idx) {
  const next = args[idx + 1];
  if (next !== undefined && ["true", "false", "1", "0"].includes(String(next).trim().toLowerCase())) {
    return { value: parseBoolFlagValue(flagName, next), nextIndex: idx + 1 };
  }
  return { value: true, nextIndex: idx };
}

function parseNonNegIntFlag(flagName, raw) {
  if (raw === undefined || raw === null || !/^\d+$/.test(String(raw).trim())) {
    throw new UsageError(`invalid non-negative integer for ${flagName}: ${raw}`);
  }
  const n = Number(String(raw).trim());
  if (!Number.isSafeInteger(n) || n < 0) {
    throw new UsageError(`invalid non-negative integer for ${flagName}: ${raw}`);
  }
  return n;
}

function parseSocialFlags(args) {
  const flags = { files: [] };
  const positional = [];
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--provider") {
      flags.provider = args[++i];
    } else if (a.startsWith("--provider=")) {
      flags.provider = a.slice("--provider=".length);
    } else if (a === "--platform") {
      flags.platform = args[++i];
    } else if (a.startsWith("--platform=")) {
      flags.platform = a.slice("--platform=".length);
    } else if (a === "--redirect-url") {
      flags.redirectUrl = args[++i];
    } else if (a.startsWith("--redirect-url=")) {
      flags.redirectUrl = a.slice("--redirect-url=".length);
    } else if (a === "--accounts") {
      flags.accounts = args[++i];
    } else if (a.startsWith("--accounts=")) {
      flags.accounts = a.slice("--accounts=".length);
    } else if (a === "--text") {
      flags.text = args[++i];
    } else if (a.startsWith("--text=")) {
      flags.text = a.slice("--text=".length);
    } else if (a === "--title") {
      flags.title = args[++i];
    } else if (a.startsWith("--title=")) {
      flags.title = a.slice("--title=".length);
    } else if (a === "--description") {
      flags.description = args[++i];
    } else if (a.startsWith("--description=")) {
      flags.description = a.slice("--description=".length);
    } else if (a === "--photo-cover-index") {
      flags.photoCoverIndex = parseNonNegIntFlag("--photo-cover-index", args[++i]);
    } else if (a.startsWith("--photo-cover-index=")) {
      flags.photoCoverIndex = parseNonNegIntFlag(
        "--photo-cover-index",
        a.slice("--photo-cover-index=".length)
      );
    } else if (a === "--is-aigc") {
      const res = consumeOptionalBool("--is-aigc", args, i);
      flags.isAigc = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--is-aigc=")) {
      flags.isAigc = parseBoolFlagValue("--is-aigc", a.slice("--is-aigc=".length));
    } else if (a === "--no-is-aigc") {
      flags.isAigc = false;
    } else if (a === "--privacy-level") {
      flags.privacyLevel = args[++i];
    } else if (a.startsWith("--privacy-level=")) {
      flags.privacyLevel = a.slice("--privacy-level=".length);
    } else if (a === "--allow-comment") {
      const res = consumeOptionalBool("--allow-comment", args, i);
      flags.allowComment = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--allow-comment=")) {
      flags.allowComment = parseBoolFlagValue("--allow-comment", a.slice("--allow-comment=".length));
    } else if (a === "--no-allow-comment") {
      flags.allowComment = false;
    } else if (a === "--allow-duet") {
      const res = consumeOptionalBool("--allow-duet", args, i);
      flags.allowDuet = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--allow-duet=")) {
      flags.allowDuet = parseBoolFlagValue("--allow-duet", a.slice("--allow-duet=".length));
    } else if (a === "--no-allow-duet") {
      flags.allowDuet = false;
    } else if (a === "--allow-stitch") {
      const res = consumeOptionalBool("--allow-stitch", args, i);
      flags.allowStitch = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--allow-stitch=")) {
      flags.allowStitch = parseBoolFlagValue("--allow-stitch", a.slice("--allow-stitch=".length));
    } else if (a === "--no-allow-stitch") {
      flags.allowStitch = false;
    } else if (a === "--commercial-content") {
      const res = consumeOptionalBool("--commercial-content", args, i);
      flags.commercialContent = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--commercial-content=")) {
      flags.commercialContent = parseBoolFlagValue(
        "--commercial-content",
        a.slice("--commercial-content=".length)
      );
    } else if (a === "--no-commercial-content") {
      flags.commercialContent = false;
    } else if (a === "--your-brand") {
      const res = consumeOptionalBool("--your-brand", args, i);
      flags.yourBrand = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--your-brand=")) {
      flags.yourBrand = parseBoolFlagValue("--your-brand", a.slice("--your-brand=".length));
    } else if (a === "--no-your-brand") {
      flags.yourBrand = false;
    } else if (a === "--branded-content") {
      const res = consumeOptionalBool("--branded-content", args, i);
      flags.brandedContent = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--branded-content=")) {
      flags.brandedContent = parseBoolFlagValue(
        "--branded-content",
        a.slice("--branded-content=".length)
      );
    } else if (a === "--no-branded-content") {
      flags.brandedContent = false;
    } else if (a === "--music-usage-confirmed") {
      const res = consumeOptionalBool("--music-usage-confirmed", args, i);
      flags.musicUsageConfirmed = res.value;
      i = res.nextIndex;
    } else if (a.startsWith("--music-usage-confirmed=")) {
      flags.musicUsageConfirmed = parseBoolFlagValue(
        "--music-usage-confirmed",
        a.slice("--music-usage-confirmed=".length)
      );
    } else if (a === "--no-music-usage-confirmed") {
      flags.musicUsageConfirmed = false;
    } else if (a === "--tiktok-options") {
      flags.tiktokOptionsJson = args[++i];
    } else if (a.startsWith("--tiktok-options=")) {
      flags.tiktokOptionsJson = a.slice("--tiktok-options=".length);
    } else if (a === "--scheduled-for") {
      flags.scheduledFor = args[++i];
    } else if (a.startsWith("--scheduled-for=")) {
      flags.scheduledFor = a.slice("--scheduled-for=".length);
    } else if (a === "--idempotency-key") {
      flags.idempotencyKey = args[++i];
    } else if (a.startsWith("--idempotency-key=")) {
      flags.idempotencyKey = a.slice("--idempotency-key=".length);
    } else if (a === "--media") {
      flags.media = args[++i];
    } else if (a.startsWith("--media=")) {
      flags.media = a.slice("--media=".length);
    } else if (a === "--file") {
      flags.files.push(args[++i]);
    } else if (a.startsWith("--file=")) {
      flags.files.push(a.slice("--file=".length));
    } else if (a === "--content-type") {
      flags.contentType = args[++i];
    } else if (a.startsWith("--content-type=")) {
      flags.contentType = a.slice("--content-type=".length);
    } else if (a === "--no-open") {
      flags.noOpen = true;
    } else if (a === "--refresh") {
      flags.refresh = true;
    } else if (a === "--no-refresh") {
      flags.noRefresh = true;
    } else if (a.startsWith("--")) {
      throw new UsageError(`unknown social flag: ${a}`);
    } else {
      positional.push(a);
    }
  }
  return { flags, positional };
}

function buildTikTokOptions(flags) {
  const opts = {};
  if (flags.tiktokOptionsJson !== undefined) {
    let parsed;
    try {
      parsed = JSON.parse(flags.tiktokOptionsJson);
    } catch {
      throw new UsageError("--tiktok-options must be a valid JSON object");
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new UsageError("--tiktok-options must be a valid JSON object");
    }
    Object.assign(opts, parsed);
  }
  if (flags.privacyLevel !== undefined) opts.privacy_level = flags.privacyLevel;
  if (flags.allowComment !== undefined) opts.allow_comment = flags.allowComment;
  if (flags.allowDuet !== undefined) opts.allow_duet = flags.allowDuet;
  if (flags.allowStitch !== undefined) opts.allow_stitch = flags.allowStitch;
  if (flags.commercialContent !== undefined) opts.commercial_content = flags.commercialContent;
  if (flags.yourBrand !== undefined) opts.your_brand = flags.yourBrand;
  if (flags.brandedContent !== undefined) opts.branded_content = flags.brandedContent;
  if (flags.musicUsageConfirmed !== undefined) {
    opts.music_usage_confirmed = flags.musicUsageConfirmed;
  }
  if (flags.title !== undefined) opts.title = flags.title;
  if (flags.description !== undefined) opts.description = flags.description;
  if (flags.photoCoverIndex !== undefined) opts.photo_cover_index = flags.photoCoverIndex;
  if (flags.isAigc !== undefined) opts.is_aigc = flags.isAigc;
  if (opts.photo_cover_index !== undefined) {
    if (
      typeof opts.photo_cover_index !== "number" ||
      !Number.isInteger(opts.photo_cover_index) ||
      opts.photo_cover_index < 0
    ) {
      throw new UsageError(
        `invalid non-negative integer for photo_cover_index: ${opts.photo_cover_index}`
      );
    }
  }
  return Object.keys(opts).length > 0 ? opts : null;
}

function resolveSocialProvider(sub, rawProvider) {
  const provider = rawProvider ? String(rawProvider).trim().toLowerCase() : "";
  if (provider === "zernio") {
    throw new UsageError(RETIRED_ZERNIO_MSG);
  }
  if (sub === "creator-info") {
    if (!provider) return OFFICIAL_PROVIDER;
    if (provider !== OFFICIAL_PROVIDER) {
      throw new UsageError(`social creator-info supports --provider ${OFFICIAL_PROVIDER} only`);
    }
    return OFFICIAL_PROVIDER;
  }
  if (["accounts", "connect", "disconnect", "post", "post-status"].includes(sub)) {
    if (provider !== OFFICIAL_PROVIDER) {
      throw new UsageError(`social command requires --provider ${OFFICIAL_PROVIDER}`);
    }
    return OFFICIAL_PROVIDER;
  }
  return provider;
}

function parseAccountIds(raw) {
  if (!raw) return [];
  return String(raw)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => {
      const n = Number(s);
      if (!Number.isInteger(n) || n <= 0) {
        throw new UsageError(`invalid account id: ${s}`);
      }
      return n;
    });
}

function parseMediaUrls(raw) {
  if (!raw) return [];
  return String(raw)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function unwrapApiData(res) {
  if (res && typeof res === "object" && res.data !== undefined) {
    return res.data;
  }
  return res;
}

function seatsSummary(seats) {
  if (!seats || typeof seats !== "object") return null;
  const credits = seats.seat_price_credits != null ? seats.seat_price_credits : 0;
  const usd = seats.seat_price_usd != null ? seats.seat_price_usd : 0;
  return {
    billing_mode: seats.billing_mode || "off",
    seats_purchased: seats.seats_purchased || 0,
    seats_occupied: seats.seats_occupied || 0,
    seats_vacant: seats.seats_vacant || 0,
    next_expiry: seats.next_expiry || null,
    seat_price_credits: credits,
    seat_price_usd: usd,
    text: `社媒绑定席位 已购买=${seats.seats_purchased || 0} 占用中=${seats.seats_occupied || 0} 可绑空闲=${seats.seats_vacant || 0} 最近到期=${seats.next_expiry || "—"} 新开一席=${credits}积分/$${usd}`,
  };
}

async function fetchSeatsQuiet(ctx) {
  try {
    const res = await ctx.api("GET", "/api/social/v1/seats");
    return unwrapApiData(res);
  } catch {
    return null;
  }
}

function mapSeatConnectError(err) {
  const msg = String((err && err.message) || "");
  const { ApiError } = require("../lib/errors");
  if (msg.includes("SOCIAL_SEAT_QUOTA") || msg.includes("席位不足") || msg.includes("余额不够")) {
    return new ApiError("社媒绑定席位不足或余额不够", {
      subtype: "quota_exceeded",
      status: err && err.status,
    });
  }
  if (msg.includes("SOCIAL_SEAT_CAP") || msg.includes("席位已达上限")) {
    return new ApiError("社媒绑定席位已达上限，请先解绑一个账号", {
      subtype: "upstream",
      status: err && err.status,
    });
  }
  return err;
}

/**
 * Presign + PUT a local file. Returns { public_url, filename, content_type, bytes }.
 */
async function uploadLocalFile(ctx, filePath, contentType) {
  const abs = path.resolve(filePath);
  if (!fs.existsSync(abs) || !fs.statSync(abs).isFile()) {
    throw new UsageError(`file not found: ${filePath}`);
  }
  const st = fs.statSync(abs);
  if (st.size > DEFAULT_MAX_BYTES) {
    throw new UsageError(`file too large (max ${DEFAULT_MAX_BYTES} bytes): ${filePath}`);
  }
  const filename = path.basename(abs);
  const ct = contentType || guessContentType(filename);
  if (!ct) {
    throw new UsageError(
      `cannot guess content-type for ${filename}; pass --content-type (e.g. video/mp4)`
    );
  }
  const presign = await ctx.api("POST", "/api/social/v1/media/presign", {
    filename,
    content_type: ct,
    size_bytes: st.size,
  });
  const data = (presign && presign.data) || presign || {};
  const uploadUrl = data.upload_url;
  const publicUrl = data.public_url;
  if (!uploadUrl || !publicUrl) {
    throw new UsageError("presign response missing upload_url or public_url");
  }
  const putHeaders = (data.headers && data.headers["Content-Type"]) || ct;
  const buf = fs.readFileSync(abs);
  await putBinary(uploadUrl, buf, putHeaders);
  return {
    public_url: publicUrl,
    filename,
    content_type: ct,
    bytes: st.size,
  };
}

async function run(ctx) {
  const sub = ctx.args[0];
  if (!sub || sub === "help") {
    printJson(
      {
        commands: {
          status: "GET /api/social/v1/status",
          accounts: "list connected accounts: --provider tiktok_direct",
          connect: "start OAuth: --provider tiktok_direct --platform tiktok [--redirect-url <url>]",
          disconnect: "disconnect account: social disconnect <accountId> --provider tiktok_direct",
          "creator-info": "query creator publishing constraints: social creator-info <accountId> [--provider tiktok_direct]",
          "media upload": "upload file: social media upload <path> [--content-type t]",
          post:
            "create post: --provider tiktok_direct --accounts id [--text '...'] [--media url1,url2] [--file path]... [--title '...'] [--description '...'] [--photo-cover-index n] [--is-aigc[=true|false]|--no-is-aigc] [--privacy-level LEVEL] [--allow-comment|--no-allow-comment] [--allow-duet|--no-allow-duet] [--allow-stitch|--no-allow-stitch] [--commercial-content|--no-commercial-content] [--your-brand|--no-your-brand] [--branded-content|--no-branded-content] [--music-usage-confirmed|--no-music-usage-confirmed] [--tiktok-options JSON] [--scheduled-for ISO] [--idempotency-key k]",
          "post-status": "get post: social post-status <postId> --provider tiktok_direct [--no-refresh]",
        },
      },
      ctx.flags
    );
    return;
  }

  // Nested: social media upload <path>
  if (sub === "media") {
    const action = ctx.args[1];
    const rest = ctx.args.slice(2);
    const { flags, positional } = parseSocialFlags(rest);
    if (action === "upload") {
      const filePath = positional[0];
      if (!filePath) {
        throw new UsageError("usage: social media upload <path> [--content-type t]");
      }
      const result = await uploadLocalFile(ctx, filePath, flags.contentType);
      printJson({ success: true, data: result }, ctx.flags);
      return;
    }
    throw new UsageError(`unknown social media subcommand: ${action || "(missing)"}`);
  }

  const rest = ctx.args.slice(1);
  const { flags, positional } = parseSocialFlags(rest);
  const provider = resolveSocialProvider(sub, flags.provider);

  switch (sub) {
    case "status": {
      const data = await ctx.api("GET", "/api/social/v1/status");
      const out = data && typeof data === "object" ? { ...data } : { data };
      if (ctx.auth && ctx.auth.accessToken) {
        const seats = await fetchSeatsQuiet(ctx);
        if (seats) {
          out.seats = seats;
          out.seats_summary = seatsSummary(seats);
        }
      }
      printJson(out, ctx.flags);
      return;
    }
    case "accounts": {
      const data = await ctx.api("GET", `/api/social/v1/accounts?provider=${provider}`);
      printJson(data, ctx.flags);
      return;
    }
    case "connect": {
      if (!flags.platform) {
        throw new UsageError("social connect requires --platform tiktok");
      }
      if (provider === OFFICIAL_PROVIDER && flags.platform !== "tiktok") {
        throw new UsageError("tiktok_direct supports TikTok only (--platform tiktok)");
      }
      const seats = await fetchSeatsQuiet(ctx);
      const vacant = seats && Number(seats.seats_vacant);
      const willCharge =
        seats &&
        seats.billing_mode === "seat" &&
        vacant === 0;
      const needCredits =
        seats && seats.seat_price_credits != null ? seats.seat_price_credits : 0;
      const body = { platform: flags.platform, provider };
      if (flags.redirectUrl) body.redirect_url = flags.redirectUrl;
      let data;
      try {
        data = await ctx.api("POST", "/api/social/v1/connect", body);
      } catch (err) {
        throw mapSeatConnectError(err);
      }
      const authUrl =
        (data && data.data && data.data.auth_url) ||
        (data && data.auth_url) ||
        null;
      const out = data && typeof data === "object" ? { ...data } : { data };
      if (willCharge) {
        out.seat_charge_hint = `社媒绑定席位无空闲，账号落库后将扣 ${needCredits} 积分`;
      }
      out.next_step = `授权成功后运行 social accounts --provider ${provider} 查看该来源账号`;
      printJson(out, ctx.flags);
      if (authUrl && !flags.noOpen && !ctx.flags.noOpen) {
        try {
          openUrl(authUrl);
        } catch {
          // non-fatal
        }
      }
      return;
    }
    case "disconnect": {
      const id = positional[0];
      if (!id) throw new UsageError("usage: social disconnect <accountId> --provider tiktok_direct");
      const data = await ctx.api("DELETE", `/api/social/v1/accounts/${encodeURIComponent(id)}?provider=${provider}`);
      printJson(data, ctx.flags);
      return;
    }
    case "creator-info": {
      const id = positional[0];
      if (!id) throw new UsageError("usage: social creator-info <accountId> [--provider tiktok_direct]");
      const data = await ctx.api(
        "GET",
        `/api/social/v1/accounts/${encodeURIComponent(id)}/creator-info?provider=${provider}`
      );
      printJson(data, ctx.flags);
      return;
    }
    case "post": {
      const accountIds = parseAccountIds(flags.accounts);
      if (accountIds.length === 0) {
        throw new UsageError("social post requires --accounts id1,id2");
      }
      const tiktokOptions = buildTikTokOptions(flags);
      const mediaItems = [];
      for (const u of parseMediaUrls(flags.media)) {
        mediaItems.push({ url: u });
      }
      for (const f of flags.files || []) {
        const up = await uploadLocalFile(ctx, f, flags.contentType);
        mediaItems.push({ url: up.public_url });
      }
      if (!flags.text && mediaItems.length === 0) {
        throw new UsageError("social post requires --text and/or --media/--file");
      }
      const body = {
        provider,
        content: flags.text || "",
        account_ids: accountIds,
      };
      if (mediaItems.length) {
        body.media_items = mediaItems;
      }
      if (tiktokOptions) {
        body.tiktok_options = tiktokOptions;
      }
      if (flags.scheduledFor) {
        body.scheduled_for = flags.scheduledFor;
        body.publish_now = false;
      } else {
        body.publish_now = true;
      }
      const headers = {};
      if (flags.idempotencyKey) {
        headers["Idempotency-Key"] = flags.idempotencyKey;
      }
      const data = await ctx.api("POST", "/api/social/v1/posts", body, headers);
      printJson(data, ctx.flags);
      return;
    }
    case "post-status": {
      const id = positional[0];
      if (!id) throw new UsageError("usage: social post-status <postId> --provider tiktok_direct");
      const q = `?provider=${provider}${flags.noRefresh ? "&refresh=0" : ""}`;
      const data = await ctx.api("GET", `/api/social/v1/posts/${encodeURIComponent(id)}${q}`);
      printJson(data, ctx.flags);
      return;
    }
    default:
      throw new UsageError(`unknown social subcommand: ${sub}`);
  }
}

module.exports = { run, parseSocialFlags, buildTikTokOptions, uploadLocalFile };
