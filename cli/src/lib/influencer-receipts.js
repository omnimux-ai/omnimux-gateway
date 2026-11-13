/** Private, account/server-partitioned recovery records. No credentials or artifact URLs. */
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { configDir } = require("./config");
const { ConfigError, ApiError } = require("./errors");
const { canonical, validateLocalFile, isRemote } = require("./influencer-input");
function digest(value) { return crypto.createHash("sha256").update(typeof value === "string" ? value : JSON.stringify(canonical(value))).digest("hex"); }
function storageError() { return new ConfigError("cannot safely persist the creation recovery record", { subtype: "receipt_unavailable", hint: "restore access to the private configuration directory, then look up the original request ID" }); }
function privateDirectory(dir) {
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  const stat = fs.lstatSync(dir);
  if (!stat.isDirectory() || stat.isSymbolicLink()) throw storageError();
  fs.chmodSync(dir, 0o700);
}
function receiptLocation(auth, id) {
  const url = new URL(auth.baseUrl);
  if (url.username || url.password || url.search || url.hash) throw storageError();
  url.pathname = url.pathname.replace(/\/+$/, "");
  const root = path.join(configDir(), "receipts");
  const account = path.join(root, digest({ server: url.href, account: String(auth.userId) }));
  privateDirectory(configDir()); privateDirectory(root); privateDirectory(account);
  return path.join(account, `${digest(id)}.json`);
}
function read(file) {
  try {
    const stat = fs.lstatSync(file);
    if (stat.isSymbolicLink() || !stat.isFile()) throw storageError();
    fs.chmodSync(file, 0o600);
    const result = JSON.parse(fs.readFileSync(file, "utf8"));
    if (result.version !== 1 || !result.request_id || !result.intent || !result.fingerprints || !result.uploads) throw storageError();
    return result;
  } catch (err) { if (err.code === "ENOENT") return null; throw storageError(); }
}
function save(file, receipt) {
  const temp = `${file}.${crypto.randomUUID()}.tmp`;
  let fd;
  try {
    fd = fs.openSync(temp, "wx", 0o600);
    fs.writeFileSync(fd, JSON.stringify(receipt) + "\n");
    fs.fsyncSync(fd); fs.closeSync(fd); fd = undefined;
    fs.renameSync(temp, file); fs.chmodSync(file, 0o600);
    // Directory fsync is unavailable on Windows; the atomic rename still applies.
    if (process.platform !== "win32") {
      const directory = fs.openSync(path.dirname(file), "r");
      try { fs.fsyncSync(directory); } finally { fs.closeSync(directory); }
    }
  } catch {
    if (fd !== undefined) try { fs.closeSync(fd); } catch {}
    try { fs.unlinkSync(temp); } catch {}
    throw storageError();
  }
}
function lock(file) {
  const lockPath = file + ".lock";
  const nonce = crypto.randomUUID();
  const temp = `${lockPath}.${nonce}.tmp`;
  let fd;
  try {
    // Publish a fully written lock atomically: a killed owner cannot leave an empty lock.
    fd = fs.openSync(temp, "wx", 0o600);
    fs.writeFileSync(fd, JSON.stringify({ pid: process.pid, nonce })); fs.fsyncSync(fd);
    fs.closeSync(fd); fd = undefined;
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        fs.linkSync(temp, lockPath);
        return () => {
          try { if (JSON.parse(fs.readFileSync(lockPath, "utf8")).nonce === nonce) fs.unlinkSync(lockPath); } catch {}
        };
      } catch (err) {
        if (err.code !== "EEXIST") throw storageError();
        let holder;
        try {
          const stat = fs.lstatSync(lockPath);
          if (!stat.isFile() || stat.isSymbolicLink()) throw storageError();
          holder = JSON.parse(fs.readFileSync(lockPath, "utf8"));
        } catch { throw storageError(); }
        if (!Number.isInteger(holder.pid) || holder.pid <= 0) throw storageError();
        try { process.kill(holder.pid, 0); }
        catch (probe) {
          if (probe.code === "ESRCH") {
            try {
              if (fs.readFileSync(lockPath, "utf8") === JSON.stringify(holder)) fs.unlinkSync(lockPath);
              continue;
            } catch { throw storageError(); }
          }
        }
        throw new ApiError("this request is already being processed locally", { subtype: "creation_request_in_progress", retryable: true });
      }
    }
    throw storageError();
  } finally {
    if (fd !== undefined) try { fs.closeSync(fd); } catch {}
    try { fs.unlinkSync(temp); } catch {}
  }
}
async function fingerprint(ref) {
  validateLocalFile(ref.value, ref.kind);
  const hash = crypto.createHash("sha256");
  try { for await (const part of fs.createReadStream(ref.value)) hash.update(part); }
  catch { throw storageError(); }
  return hash.digest("hex");
}
function materialKey(ref) { return ref.field + (ref.index === undefined ? "" : `:${ref.index}`); }
async function prepare(auth, id, intent, refs) {
  let file, release;
  try { file = receiptLocation(auth, id); release = lock(file); }
  catch (err) { if (err instanceof ConfigError || err instanceof ApiError) throw err; throw storageError(); }
  try {
    const previous = read(file);
    if (previous && digest(previous.intent) !== digest(intent)) throw new ApiError("request ID is already associated with different local input", { subtype: "creation_request_conflict", status: 409 });
    const fingerprints = {};
    for (const ref of refs) {
      if (isRemote(ref.value)) continue;
      const key = materialKey(ref);
      // Once submission was attempted, a removed source file is unnecessary for recovery.
      if (previous?.attempted && !fs.existsSync(ref.value)) { fingerprints[key] = previous.fingerprints[key]; continue; }
      fingerprints[key] = await fingerprint(ref);
      if (previous && fingerprints[key] !== previous.fingerprints[key]) throw new ApiError("local material changed for an existing request ID", { subtype: "creation_request_conflict", status: 409 });
    }
    const receipt = previous || { version: 1, request_id: id, intent, fingerprints, uploads: {}, stage: "prepared", attempted: false, created_at: Date.now() };
    save(file, receipt); // The durable barrier precedes every upload or submission.
    return { receipt, save: () => save(file, receipt), release };
  } catch (err) { release(); throw err; }
}
module.exports = { prepare, materialKey, digest };
