#!/usr/bin/env node

const { bumpVersion, fail, getPackageVersion, isValidSemver, paths, updateChangelog } = require('./lib/release-utils');

const requestedVersion = (process.argv[2] || '').trim();
if (!requestedVersion) {
  fail('missing version argument. Example: npm run changelog -- 0.1.1 or npm run changelog -- patch');
}

const currentVersion = getPackageVersion();
const nextVersion = ['patch', 'minor', 'major'].includes(requestedVersion)
  ? bumpVersion(currentVersion, requestedVersion)
  : requestedVersion;

if (!isValidSemver(nextVersion)) {
  fail(`invalid version: ${nextVersion}`);
}

updateChangelog(nextVersion);
console.log(`[orch] updated ${paths.changelog} for v${nextVersion}`);
