#!/usr/bin/env node

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const packageRoot = path.resolve(__dirname, '..');
const binaryName = process.platform === 'win32' ? 'orch.exe' : 'orch';
const installedBinary = path.join(packageRoot, '.npm-bin', binaryName);

if (!fs.existsSync(installedBinary)) {
  console.error('orch binary is not installed. Re-run `npm install -g orch` or install from a published release.');
  process.exit(1);
}

const result = spawnSync(installedBinary, process.argv.slice(2), {
  stdio: 'inherit',
});

if (result.error) {
  console.error(`failed to launch orch: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status ?? 1);
