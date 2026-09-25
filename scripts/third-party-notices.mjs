// Writes THIRD_PARTY_NOTICES: the license of Go, of every Go module the Server
// links, and of every package the Web App installs. The web list deliberately
// includes build-only tools rather than tracing what the bundle keeps; extra
// notices are harmless, a missing one is not. Run from the repository root.
import { execFileSync } from 'node:child_process';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const run = (cmd, ...args) => execFileSync(cmd, args, { encoding: 'utf8' }).trim();

const licenseTexts = (dir) =>
  readdirSync(dir)
    .filter((file) => /^(licen[cs]e|copying|notice)/i.test(file))
    .sort()
    .map((file) => readFileSync(join(dir, file), 'utf8').trim());

// Keyed by "name version", which also drops pnpm's duplicate peer-dependency
// installs of one version.
const notices = new Map();
// A web package without a license file falls back to the license its
// package.json declares.
function add(name, dir, declared) {
  const texts = licenseTexts(dir);
  if (texts.length === 0 && !declared) throw new Error(`no license for ${name} in ${dir}`);
  notices.set(name, texts.length > 0 ? texts : [`License: ${declared}`]);
}

add('Go', run('go', 'env', 'GOROOT'));

// The modules the image's binary links, which the Dockerfile builds for linux
// without cgo, whatever machine runs this.
process.env.GOOS = 'linux';
process.env.CGO_ENABLED = '0';
const modules = run(
  'go',
  'list',
  '-deps',
  '-f',
  '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}',
  './cmd/yogurt',
);
for (const line of modules.split('\n').filter(Boolean).sort()) {
  const [path, version, ...dir] = line.split(' ');
  add(`${path} ${version}`, dir.join(' '));
}

const web = JSON.parse(run('pnpm', '--dir', 'web', 'licenses', 'list', '--json', '--long'));
const packages = Object.values(web)
  .flat()
  .flatMap((pkg) => pkg.paths)
  .map((dir) => ({ dir, manifest: JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8')) }))
  // A package pinned to an OS or CPU is a native build tool for the machine
  // that installed it. Nothing of it ships, and skipping it keeps this file
  // the same whichever machine generates it.
  .filter(({ manifest }) => !manifest.os && !manifest.cpu)
  .map(({ dir, manifest }) => ({ name: `${manifest.name} ${manifest.version}`, dir, manifest }))
  .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
for (const { name, dir, manifest } of packages) add(name, dir, manifest.license);

const rule = '-'.repeat(80);
writeFileSync(
  'THIRD_PARTY_NOTICES',
  'Yogurt includes the third-party software below, each under its own license.\n' +
    [...notices].map(([name, texts]) => `\n${rule}\n${name}\n${rule}\n\n${texts.join('\n\n')}\n`).join(''),
);
