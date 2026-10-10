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

export async function publishSite(baseUrl, key, options) {
  const entries = Object.entries(options.files);
  const manifest = entries.map(([filePath, content]) => ({
    path: filePath,
    size: Buffer.byteLength(content),
    sha256: createHash("sha256").update(content).digest("hex"),
    contentType: filePath.endsWith(".html") ? "text/html; charset=utf-8" : "text/plain; charset=utf-8",
  }));
  const pending = await call(baseUrl, key, "POST", "/api/v1/publish", {
    slug: options.slug,
    displayName: options.displayName,
    files: manifest,
  });
  for (const upload of pending.uploads) {
    await call(baseUrl, key, upload.method, upload.url, options.files[upload.path], "application/octet-stream");
  }
  return call(baseUrl, key, "POST", pending.finalizeUrl);
}
