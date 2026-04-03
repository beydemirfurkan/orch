#!/usr/bin/env node

const { fail, getCmdVersion, getPackageVersion } = require('./lib/release-utils');

const tag = (process.argv[2] || '').trim();
if (!tag) {
  fail('missing tag argument, expected something like v0.1.0');
}

const normalizedTag = tag.startsWith('v') ? tag.slice(1) : tag;
const packageVersion = getPackageVersion();
if (normalizedTag !== packageVersion) {
  fail(`git tag ${tag} does not match package.json version ${packageVersion}`);
}

const cmdVersion = getCmdVersion();
if (cmdVersion !== packageVersion) {
  fail(`cmd/version.go version ${cmdVersion} does not match package.json version ${packageVersion}`);
}

console.log(`[orch] release version ${packageVersion} matches tag ${tag}`);
