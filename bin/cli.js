#!/usr/bin/env node

import { spawn } from "node:child_process";
import { createWriteStream } from "node:fs";
import { mkdir } from "node:fs/promises";
import { get } from "node:https";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const BIN_DIR = resolve(__dirname, "..", ".bin");
const OWNER = "leo-cmp";
const REPO = "nudge";
const VERSION = "latest";

const PLATFORM_MAP = {
  "linux-x64": "nudge-linux-amd64",
  "linux-arm64": "nudge-linux-arm64",
  "darwin-x64": "nudge-darwin-amd64",
  "darwin-arm64": "nudge-darwin-arm64",
  "win32-x64": "nudge-windows-amd64.exe",
};

function getPlatformKey() {
  const os = process.platform;
  const arch = process.arch === "x64" ? "x64" : process.arch === "arm64" ? "arm64" : null;
  if (!arch) throw new Error(`Unsupported architecture: ${process.arch}`);
  const key = `${os}-${arch}`;
  if (!PLATFORM_MAP[key]) throw new Error(`Unsupported platform: ${key}`);
  return key;
}

async function getLatestReleaseURL() {
  const url = `https://api.github.com/repos/${OWNER}/${REPO}/releases/${VERSION}`;
  return new Promise((resolve, reject) => {
    get(url, { headers: { "User-Agent": "nudge-mcp-installer", Accept: "application/vnd.github+json" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        get(
          res.headers.location,
          { headers: { "User-Agent": "nudge-mcp-installer", Accept: "application/vnd.github+json" } },
          (redirectRes) => {
            let data = "";
            redirectRes.on("data", (c) => (data += c));
            redirectRes.on("end", () => {
              try {
                resolve(JSON.parse(data));
              } catch (e) {
                reject(new Error("Failed to parse release data"));
              }
            });
          }
        ).on("error", reject);
        return;
      }
      let data = "";
      res.on("data", (c) => (data += c));
      res.on("end", () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          reject(new Error("Failed to parse release data"));
        }
      });
    }).on("error", reject);
  });
}

async function downloadFile(url, destPath) {
  return new Promise((resolve, reject) => {
    get(url, { headers: { "User-Agent": "nudge-mcp-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        downloadFile(res.headers.location, destPath).then(resolve).catch(reject);
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`Failed to download: HTTP ${res.statusCode}`));
        return;
      }
      const file = createWriteStream(destPath, { mode: 0o755 });
      res.pipe(file);
      file.on("finish", () => resolve());
      file.on("error", reject);
    }).on("error", reject);
  });
}

async function downloadBinary(release, platformKey) {
  const assetName = PLATFORM_MAP[platformKey];
  const asset = release.assets.find((a) => a.name === assetName);
  if (!asset) throw new Error(`No binary found for ${platformKey} (${assetName})`);

  const binPath = resolve(BIN_DIR, assetName);
  await mkdir(BIN_DIR, { recursive: true });

  console.error(`[nudge-mcp] Downloading ${assetName}...`);
  await downloadFile(asset.browser_download_url, binPath);
  console.error(`[nudge-mcp] Binary installed: ${binPath}`);

  return binPath;
}

async function getOrDownloadBinary() {
  const platformKey = getPlatformKey();
  const assetName = PLATFORM_MAP[platformKey];
  const binPath = resolve(BIN_DIR, assetName);

  try {
    await import("node:fs").then((fs) => fs.accessSync(binPath));
    return binPath;
  } catch {
    // Not cached, download
  }

  const release = await getLatestReleaseURL();
  return downloadBinary(release, platformKey);
}

async function main() {
  try {
    const binPath = await getOrDownloadBinary();
    const child = spawn(binPath, ["--stdio"], {
      stdio: ["pipe", "pipe", "inherit"],
      env: process.env,
    });

    process.stdin.pipe(child.stdin);
    child.stdout.pipe(process.stdout);

    child.on("exit", (code) => process.exit(code || 0));
  } catch (err) {
    console.error(`[nudge-mcp] ${err.message}`);
    process.exit(1);
  }
}

main();
