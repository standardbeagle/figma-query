#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');

const platform = os.platform();
const arch = os.arch();

// Map Node.js platform/arch to Go naming
const platformMap = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows',
};

const archMap = {
  x64: 'amd64',
  arm64: 'arm64',
};

const goPlatform = platformMap[platform];
const goArch = archMap[arch];

if (!goPlatform || !goArch) {
  console.error(`Unsupported platform: ${platform}/${arch}`);
  process.exit(1);
}

const binaryName = platform === 'win32' ? 'figma-query.exe' : 'figma-query';
const sourceName = `figma-query-${goPlatform}-${goArch}${platform === 'win32' ? '.exe' : ''}`;

const binDir = path.join(__dirname, '..', 'bin');
const sourcePath = path.join(binDir, sourceName);
const targetPath = path.join(binDir, binaryName);

// Check if platform-specific binary exists
if (fs.existsSync(sourcePath)) {
  // Copy or rename to the generic name
  try {
    fs.copyFileSync(sourcePath, targetPath);
    fs.chmodSync(targetPath, 0o755);
    console.log(`figma-query installed for ${platform}/${arch}`);
  } catch (err) {
    console.error(`Failed to install binary: ${err.message}`);
    process.exit(1);
  }
} else if (fs.existsSync(targetPath)) {
  // Binary already exists (perhaps single-platform build)
  try {
    fs.chmodSync(targetPath, 0o755);
  } catch (err) {
    // Ignore chmod errors on Windows
  }
  console.log('figma-query ready');
} else {
  console.error(`Binary not found for ${platform}/${arch}`);
  console.error(`Expected: ${sourcePath}`);
  process.exit(1);
}
