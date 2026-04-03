#!/usr/bin/env node

const fs = require('node:fs');
const path = require('node:path');
const https = require('node:https');
const http = require('node:http');
const { spawnSync } = require('node:child_process');

const packageRoot = path.resolve(__dirname, '..');
const pkg = require(path.join(packageRoot, 'package.json'));
const binaryDir = path.join(packageRoot, '.npm-bin');
const binaryName = process.platform === 'win32' ? 'orch.exe' : 'orch';
const binaryPath = path.join(binaryDir, binaryName);

const targets = {
  darwin: { x64: 'darwin-x64', arm64: 'darwin-arm64' },
  linux: { x64: 'linux-x64', arm64: 'linux-arm64' },
  win32: { x64: 'windows-x64.exe', arm64: 'windows-arm64.exe' },
};

function main() {
  if (process.env.ORCH_SKIP_DOWNLOAD === '1') {
    console.log('[orch] skipping binary install because ORCH_SKIP_DOWNLOAD=1');
    return;
  }

  ensureDir(binaryDir);

  if (fs.existsSync(binaryPath)) {
    makeExecutable(binaryPath);
    console.log(`[orch] using existing binary at ${binaryPath}`);
    return;
  }

  install()
    .then(() => {
      makeExecutable(binaryPath);
      console.log(`[orch] binary ready at ${binaryPath}`);
    })
    .catch((error) => {
      console.error(`[orch] install failed: ${error.message}`);
      process.exit(1);
    });
}

async function install() {
  const assetName = resolveAssetName();
  const baseUrl = process.env.ORCH_BINARY_BASE_URL || `https://github.com/beydemirfurkan/orch/releases/download/v${pkg.version}`;
  const assetUrl = `${baseUrl}/${assetName}`;

  try {
    console.log(`[orch] downloading ${assetUrl}`);
    await downloadFile(assetUrl, binaryPath);
    return;
  } catch (downloadError) {
    console.warn(`[orch] release download unavailable: ${downloadError.message}`);
  }

  buildFromSource();
}

function resolveAssetName() {
  const platformTargets = targets[process.platform];
  if (!platformTargets) {
    throw new Error(`unsupported platform: ${process.platform}`);
  }

  const assetSuffix = platformTargets[process.arch];
  if (!assetSuffix) {
    throw new Error(`unsupported architecture: ${process.arch}`);
  }

  return `orch-${assetSuffix}`;
}

function downloadFile(url, destination, redirectCount = 0) {
  if (redirectCount > 5) {
    return Promise.reject(new Error('too many redirects'));
  }

  const client = url.startsWith('https://') ? https : http;

  return new Promise((resolve, reject) => {
    const request = client.get(url, (response) => {
      if (response.statusCode && response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(downloadFile(response.headers.location, destination, redirectCount + 1));
        return;
      }

      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`unexpected status code ${response.statusCode || 'unknown'}`));
        return;
      }

      const file = fs.createWriteStream(destination, { mode: 0o755 });
      response.pipe(file);

      file.on('finish', () => {
        file.close((closeError) => {
          if (closeError) {
            reject(closeError);
            return;
          }
          resolve();
        });
      });

      file.on('error', (fileError) => {
        fs.rmSync(destination, { force: true });
        reject(fileError);
      });
    });

    request.on('error', (error) => {
      fs.rmSync(destination, { force: true });
      reject(error);
    });
  });
}

function buildFromSource() {
  const versionResult = spawnSync('go', ['version'], {
    cwd: packageRoot,
    encoding: 'utf8',
  });

  if (versionResult.error || versionResult.status !== 0) {
    throw new Error('no published binary found and `go` is not available for source build fallback');
  }

  console.log('[orch] building from source with local Go toolchain');
  const buildResult = spawnSync('go', ['build', '-o', binaryPath, '.'], {
    cwd: packageRoot,
    stdio: 'inherit',
  });

  if (buildResult.error) {
    throw buildResult.error;
  }
  if (buildResult.status !== 0) {
    throw new Error(`go build failed with exit code ${buildResult.status}`);
  }
}

function ensureDir(directory) {
  fs.mkdirSync(directory, { recursive: true });
}

function makeExecutable(filePath) {
  if (process.platform !== 'win32' && fs.existsSync(filePath)) {
    fs.chmodSync(filePath, 0o755);
  }
}

main();
