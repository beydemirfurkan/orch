const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const packageRoot = path.resolve(__dirname, '..', '..');
const paths = {
  packageJson: path.join(packageRoot, 'package.json'),
  packageLock: path.join(packageRoot, 'package-lock.json'),
  cmdVersion: path.join(packageRoot, 'cmd', 'version.go'),
  changelog: path.join(packageRoot, 'CHANGELOG.md'),
};

function fail(message) {
  console.error(`[orch] ${message}`);
  process.exit(1);
}

function isValidSemver(value) {
  return /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(value);
}

function parseSemver(value) {
  if (!isValidSemver(value)) {
    fail(`invalid version: ${value}`);
  }
  const match = value.match(/^(\d+)\.(\d+)\.(\d+)(.*)$/);
  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
    suffix: match[4] || '',
  };
}

function bumpVersion(currentVersion, releaseType) {
  const parsed = parseSemver(currentVersion);
  if (releaseType === 'patch') {
    return `${parsed.major}.${parsed.minor}.${parsed.patch + 1}`;
  }
  if (releaseType === 'minor') {
    return `${parsed.major}.${parsed.minor + 1}.0`;
  }
  if (releaseType === 'major') {
    return `${parsed.major + 1}.0.0`;
  }
  return releaseType;
}

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function writeJSON(filePath, data) {
  fs.writeFileSync(filePath, `${JSON.stringify(data, null, 2)}\n`);
}

function getPackageVersion() {
  return readJSON(paths.packageJson).version;
}

function getCmdVersion() {
  const source = fs.readFileSync(paths.cmdVersion, 'utf8');
  const versionMatch = source.match(/version\s*=\s*"([^"]+)"/);
  if (!versionMatch) {
    fail('could not parse cmd/version.go');
  }
  return versionMatch[1];
}

function updateVersions(nextVersion) {
  const pkg = readJSON(paths.packageJson);
  const lock = readJSON(paths.packageLock);
  const currentVersion = pkg.version;

  pkg.version = nextVersion;
  lock.version = nextVersion;
  if (lock.packages && lock.packages['']) {
    lock.packages[''].version = nextVersion;
  }

  writeJSON(paths.packageJson, pkg);
  writeJSON(paths.packageLock, lock);
  updateCmdVersion(nextVersion);
  return currentVersion;
}

function updateCmdVersion(nextVersion) {
  const source = fs.readFileSync(paths.cmdVersion, 'utf8');
  if (source.includes(`var version = "${nextVersion}"`)) {
    return;
  }
  const updated = source.replace(/var version = "[^"]+"/, `var version = "${nextVersion}"`);
  if (updated === source) {
    fail('could not update cmd/version.go');
  }
  fs.writeFileSync(paths.cmdVersion, updated);
}

function runGit(args, options = {}) {
  const result = spawnSync('git', args, {
    cwd: packageRoot,
    encoding: 'utf8',
    ...options,
  });

  if (!options.allowFailure && (result.error || result.status !== 0)) {
    const stderr = (result.stderr || '').trim();
    fail(stderr || `git ${args.join(' ')} failed`);
  }

  return result;
}

function getLastTag() {
  const result = runGit(['describe', '--tags', '--abbrev=0'], { allowFailure: true });
  if (result.error || result.status !== 0) {
    return '';
  }
  return (result.stdout || '').trim();
}

function getCommitSubjects(range) {
  const args = ['log', '--pretty=format:%s'];
  if (range) {
    args.push(range);
  }
  const result = runGit(args);
  return (result.stdout || '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function getWorkingTreeChanges() {
  const result = runGit(['status', '--short']);
  return (result.stdout || '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function createTag(version, allowedFiles = []) {
  const tagName = `v${version}`;
  const changes = getWorkingTreeChanges().filter((line) => {
    return !allowedFiles.some((allowedFile) => line.endsWith(allowedFile));
  });

  if (changes.length > 0) {
    fail('refusing to create tag with unrelated working tree changes present');
  }

  const tagResult = runGit(['tag', tagName], { stdio: 'inherit', allowFailure: true });
  if (tagResult.error) {
    fail(`failed to create tag ${tagName}: ${tagResult.error.message}`);
  }
  if (tagResult.status !== 0) {
    fail(`git tag exited with status ${tagResult.status}`);
  }
  return tagName;
}

function buildReleaseNotes(version) {
  const lastTag = getLastTag();
  const range = lastTag ? `${lastTag}..HEAD` : '';
  const commits = getCommitSubjects(range);

  const lines = [];
  lines.push(`## v${version} - ${new Date().toISOString().slice(0, 10)}`);
  lines.push('');
  if (commits.length === 0) {
    lines.push('- No user-facing changes recorded.');
  } else {
    for (const subject of commits) {
      lines.push(`- ${subject}`);
    }
  }
  lines.push('');
  return lines.join('\n');
}

function updateChangelog(version) {
  const releaseNotes = buildReleaseNotes(version);
  const header = '# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n';
  let existing = fs.existsSync(paths.changelog) ? fs.readFileSync(paths.changelog, 'utf8') : header;

  if (!existing.startsWith('# Changelog')) {
    existing = `${header}${existing.trim()}\n`;
  }

  const sectionHeader = `## v${version} - `;
  if (existing.includes(sectionHeader)) {
    const sectionRegex = new RegExp(`## v${escapeRegExp(version)} - [\\s\\S]*?(?=\\n## v|$)`, 'm');
    existing = existing.replace(sectionRegex, releaseNotes.trim());
  } else {
    existing = `${header}${releaseNotes}${existing.replace(header, '')}`;
  }

  fs.writeFileSync(paths.changelog, normalizeTrailingNewline(existing));
}

function normalizeTrailingNewline(content) {
  return `${content.trim()}\n`;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

module.exports = {
  buildReleaseNotes,
  bumpVersion,
  createTag,
  fail,
  getCmdVersion,
  getLastTag,
  getPackageVersion,
  isValidSemver,
  packageRoot,
  paths,
  runGit,
  updateChangelog,
  updateVersions,
};
