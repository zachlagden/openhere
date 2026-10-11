import { spawn } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import readline from "node:readline";

async function freePort() {
  return new Promise((resolve, reject) => {
    const probe = createServer();
    probe.once("error", reject);
    probe.listen(0, "127.0.0.1", () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

export async function startServer(binary) {
  const dataDir = await mkdtemp(path.join(tmpdir(), "openhere-e2e-"));
  const port = await freePort();
  const baseUrl = `http://127.0.0.1:${port}`;
  const child = spawn(binary, [], {
    env: { ...process.env, DATA_DIR: dataDir, LISTEN_ADDR: `127.0.0.1:${port}`, PUBLIC_URL: baseUrl, LOG_LEVEL: "info" },
    stdio: ["ignore", "pipe", "pipe"],
  });
  const lines = readline.createInterface({ input: child.stdout });
  const setupToken = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("The server did not print a setup token in time.")), 20_000);
    child.once("exit", (code) => reject(new Error(`The server exited early with code ${code}.`)));
    lines.on("line", (line) => {
      try {
        const entry = JSON.parse(line);
        if (entry.setup_token) {
          clearTimeout(timer);
          resolve(entry.setup_token);
        }
      } catch {
        return;
      }
    });
  });
  return {
    baseUrl,
    setupToken,
    async stop() {
      child.removeAllListeners("exit");
      child.kill("SIGTERM");
      await new Promise((resolve) => child.once("exit", resolve));
      await rm(dataDir, { recursive: true, force: true });
    },
  };
}
