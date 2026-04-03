#!/usr/bin/env node

const https = require('node:https');
const path = require('node:path');

const packageRoot = path.resolve(__dirname, '..');
const pkg = require(path.join(packageRoot, 'package.json'));

const version = process.argv[2] || pkg.version;
const encodedName = encodeURIComponent(pkg.name);
const url = `https://registry.npmjs.org/${encodedName}/${encodeURIComponent(version)}`;

https
  .get(url, (response) => {
    response.resume();

    if (response.statusCode === 200) {
      console.log('true');
      process.exit(0);
    }

    if (response.statusCode === 404) {
      console.log('false');
      process.exit(0);
    }

    console.error(`[orch] unexpected npm registry status: ${response.statusCode || 'unknown'}`);
    process.exit(1);
  })
  .on('error', (error) => {
    console.error(`[orch] failed to query npm registry: ${error.message}`);
    process.exit(1);
  });
