const fs = require("fs");
const path = require("path");

const { UsageError } = require("../../lib/output");
const { putBinary, guessContentType, DEFAULT_MAX_BYTES } = require("../../lib/upload");

function unwrapApiData(res) {
  if (res && typeof res === "object" && res.data !== undefined) {
    return res.data;
  }
  return res;
}

async function uploadLocalFile(ctx, filePath, explicitType) {
  const resolved = path.resolve(process.cwd(), filePath);
  if (!fs.existsSync(resolved)) {
    throw new UsageError(`file not found: ${filePath}`);
  }
  const stat = fs.statSync(resolved);
  if (stat.size > DEFAULT_MAX_BYTES) {
    throw new UsageError(`file too large (${stat.size} bytes > ${DEFAULT_MAX_BYTES} bytes cap)`);
  }
  const contentType = explicitType || guessContentType(resolved) || "application/octet-stream";
  const filename = path.basename(resolved);

  const presignRes = await ctx.api("POST", "/api/hosted/v1/media/presign", {
    filename,
    content_type: contentType,
  });
  const presignData = unwrapApiData(presignRes);
  const uploadUrl = presignData.upload_url || (presignData.data && presignData.data.upload_url);
  const resourceUrl =
    presignData.resource_url ||
    presignData.url ||
    (presignData.data && (presignData.data.resource_url || presignData.data.url));

  if (!uploadUrl || !resourceUrl) {
    throw new UsageError("presign response did not contain upload_url and resource_url");
  }

  const fileBytes = fs.readFileSync(resolved);
  await putBinary(uploadUrl, fileBytes, contentType);
  return { resource_url: resourceUrl, filename, size_bytes: stat.size, content_type: contentType };
}

module.exports = {
  unwrapApiData,
  uploadLocalFile,
};
