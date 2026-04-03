#!/usr/bin/env node

const {
  bumpVersion,
  createTag,
  fail,
  getPackageVersion,
  isValidSemver,
  paths,
  updateChangelog,
  updateVersions,
} = require('./lib/release-utils');

const args = process.argv.slice(2);
const requestedVersion = (args[0] || '').trim();
const shouldTag = args.includes('--tag');
const shouldUpdateChangelog = !args.includes('--no-changelog');

if (!requestedVersion) {
  fail('missing version argument. Example: npm run release:prepare -- 0.1.1 or npm run release:prepare -- patch');
}

const currentVersion = getPackageVersion();
const nextVersion = ['patch', 'minor', 'major'].includes(requestedVersion)
  ? bumpVersion(currentVersion, requestedVersion)
  : requestedVersion;

if (!isValidSemver(nextVersion)) {
  fail(`invalid version: ${nextVersion}`);
}

updateVersions(nextVersion);
if (shouldUpdateChangelog) {
  updateChangelog(nextVersion);
}

console.log(`[orch] updated version ${currentVersion} -> ${nextVersion}`);
if (shouldUpdateChangelog) {
  console.log(`[orch] updated ${paths.changelog}`);
}

if (shouldTag) {
  const tagName = createTag(nextVersion, [
    'package.json',
    'package-lock.json',
    'cmd/version.go',
    'CHANGELOG.md',
  ]);
  console.log(`[orch] created git tag ${tagName}`);
}

console.log('[orch] next steps: review changes, commit, and push the tag when ready');
