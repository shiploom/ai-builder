#!/usr/bin/env node
// install.js — fetch the platform Go binary from GitHub releases (stdlib-only:
// global fetch, no dependencies). The binary itself runs fully offline.
"use strict";

const { createWriteStream, chmodSync, mkdirSync, existsSync } = require("fs");
const { get } = require("https");
const { join } = require("path");
const { spawnSync } = require("child_process");

const VERSION = require("./package.json").version;

const ASSETS = {
  "darwin-arm64": "shiploom-VERSION-darwin-arm64",
  "darwin-x64": "shiploom-VERSION-darwin-amd64",
  "linux-arm64": "shiploom-VERSION-linux-arm64",
  "linux-x64": "shiploom-VERSION-linux-amd64",
  "win32-arm64": "shiploom-VERSION-windows-amd64.exe",
  "win32-x64": "shiploom-VERSION-windows-amd64.exe",
};

function asset() {
  const key = `${process.platform}-${process.arch}`;
  const name = ASSETS[key];
  if (!name) throw new Error(`unsupported platform: ${key}`);
  return name.replace(/VERSION/g, VERSION);
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    get(url, { headers: { "User-Agent": "shiploom-npx" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return resolve(download(res.headers.location, dest));
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`download failed: HTTP ${res.statusCode} for ${url}`));
      }
      const out = createWriteStream(dest, { mode: 0o755 });
      res.pipe(out);
      out.on("finish", () => resolve());
      out.on("error", reject);
    }).on("error", reject);
  });
}

(async () => {
  const dir = join(__dirname, "vendor");
  mkdirSync(dir, { recursive: true });
  const dest = join(dir, process.platform === "win32" ? "shiploom.exe" : "shiploom");
  if (existsSync(dest)) {
    const check = spawnSync(dest, ["--version"], { encoding: "utf8" });
    if (check.status === 0 && String(check.stdout).includes(VERSION)) return;
  }
  const url = `https://github.com/shiploom/ai-builder/releases/download/v${VERSION}/${asset()}`;
  process.stderr.write(`shiploom: downloading ${asset()} ...\n`);
  await download(url, dest);
  chmodSync(dest, 0o755);
})().catch((err) => {
  process.stderr.write(`shiploom: install failed: ${err.message}\n`);
  process.exit(1);
});
