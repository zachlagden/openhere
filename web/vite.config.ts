import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

const root = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(root, "../internal/ui/dist");

function keepEmbedPlaceholder(): Plugin {
  return {
    name: "keep-embed-placeholder",
    apply: "build",
    async closeBundle() {
      await mkdir(outDir, { recursive: true });
      await writeFile(path.join(outDir, ".gitkeep"), "");
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepEmbedPlaceholder()],
  resolve: {
    alias: { "@": path.resolve(root, "src") },
  },
  build: {
    outDir,
    emptyOutDir: true,
    assetsInlineLimit: 0,
    sourcemap: false,
    target: "es2023",
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    css: false,
  },
});
