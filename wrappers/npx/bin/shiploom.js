#!/usr/bin/env node
// bin/shiploom.js — passthrough to the vendored Go binary.
"use strict";

const { spawnSync } = require("child_process");
const { existsSync } = require("fs");
const { join } = require("path");

const bin = join(__dirname, "..", "vendor", process.platform === "win32" ? "shiploom.exe" : "shiploom");
if (!existsSync(bin)) {
  // Self-heal when postinstall never ran (npx cache installs and
  // install-scripts allow-lists both skip it): install.js is idempotent.
  const installer = spawnSync(process.execPath, [join(__dirname, "..", "install.js")], { stdio: "inherit" });
  if (installer.status !== 0 || !existsSync(bin)) {
    process.stderr.write("shiploom: binary missing (run `npm install` again)\n");
    process.exit(1);
  }
}
const child = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(child.status === null ? 1 : child.status);
