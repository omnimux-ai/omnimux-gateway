/**
 * omnimux logout — clear stored access token (and user_id).
 *
 *   omnimux logout
 */

const { deleteStoredToken, accessTokenMeta } = require("../lib/secrets");
const { loadConfig, saveConfig } = require("../lib/config");
const { printJson } = require("../lib/output");

async function run(ctx) {
  deleteStoredToken();
  const cfg = loadConfig();
  const hadUserId = !!cfg.user_id;
  if (cfg.user_id) {
    delete cfg.user_id;
    saveConfig(cfg);
  }
  printJson(
    {
      success: true,
      access_token: accessTokenMeta(),
      user_id_cleared: hadUserId,
    },
    ctx.flags
  );
}

module.exports = { run };
