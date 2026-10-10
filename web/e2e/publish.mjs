import { createHash } from "node:crypto";

async function call(baseUrl, key, method, route, body, contentType = "application/json") {
  const response = await fetch(`${baseUrl}${route}`, {
    method,
    headers: { Authorization: `Bearer ${key}`, "Content-Type": contentType },
    body: body === undefined ? undefined : typeof body === "string" ? body : JSON.stringify(body),
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${method} ${route} answered ${response.status}: ${text}`);
  }
  return text === "" ? null : JSON.parse(text);
}

function manifestFor(files) {
  return Object.entries(files).map(([filePath, content]) => ({
    path: filePath,
    size: Buffer.byteLength(content),
    sha256: createHash("sha256").update(content).digest("hex"),
    contentType: filePath.endsWith(".html") ? "text/html; charset=utf-8" : "text/plain; charset=utf-8",
  }));
}

async function uploadAndFinalize(baseUrl, key, pending, files) {
  for (const upload of pending.uploads) {
    await call(baseUrl, key, upload.method, upload.url, files[upload.path], "application/octet-stream");
  }
  return call(baseUrl, key, "POST", pending.finalizeUrl);
}

export async function publishSite(baseUrl, key, options) {
  const pending = await call(baseUrl, key, "POST", "/api/v1/publish", {
    slug: options.slug,
    displayName: options.displayName,
    workspace: options.workspace,
    folder: options.folder,
    files: manifestFor(options.files),
  });
  return uploadAndFinalize(baseUrl, key, pending, options.files);
}

export async function republishSite(baseUrl, key, slug, files) {
  const pending = await call(baseUrl, key, "PUT", `/api/v1/publish/${slug}`, { files: manifestFor(files) });
  return uploadAndFinalize(baseUrl, key, pending, files);
}

export async function apiCall(baseUrl, key, method, route, body) {
  return call(baseUrl, key, method, `/api/v1${route}`, body);
}
