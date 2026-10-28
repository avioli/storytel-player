// Copies the version in package.json to every file that repeats it.
//
//   node scripts/sync-version.mjs          rewrite the files
//   node scripts/sync-version.mjs --check  only report files that disagree
//
// `npm version <x>` runs the rewrite through the "version" script in
// package.json, so a release bump is a single command.

import { readFileSync, writeFileSync } from 'fs';
import { resolve } from 'path';

const pkg = JSON.parse(readFileSync(resolve('package.json'), 'utf-8'));
const version = pkg.version;
if (!/^\d+\.\d+\.\d+$/.test(version)) {
  console.error(`package.json version "${version}" is not MAJOR.MINOR.PATCH`);
  process.exit(1);
}
// Windows MSIX packages need a four-part version.
const version4 = `${version}.0`;

// Each rule: [regex whose group 1 is kept, replacement value]. Every regex
// must match at least once, so a template change cannot slip by silently.
const files = {
  'server/main.go': [[/(const appVersion = ")[^"]*/g, version]],
  'build/config.yml': [[/(^\s*version: ")[^"]*/gm, version]],
  'build/darwin/Info.plist': [
    [/(<key>CFBundle(?:Short)?Version(?:String)?<\/key>\s*<string>)[^<]*/g, version],
  ],
  'build/darwin/Info.dev.plist': [
    [/(<key>CFBundle(?:Short)?Version(?:String)?<\/key>\s*<string>)[^<]*/g, version],
  ],
  'build/linux/nfpm/nfpm.yaml': [[/(^version: ")[^"]*/gm, version]],
  'build/windows/info.json': [[/("(?:file_version|ProductVersion)": ")[^"]*/g, version]],
  'build/windows/msix/app_manifest.xml': [[/(\sVersion=")[^"]*/g, version4]],
  'build/windows/msix/template.xml': [[/(\sVersion=")[^"]*/g, version4]],
  'build/windows/nsis/wails_tools.nsh': [[/(!define INFO_PRODUCTVERSION ")[^"]*/g, version]],
  'build/windows/wails.exe.manifest': [[/(<assemblyIdentity\b[^>]*\sname="com\.storytel\.[^"]*"[^>]*\sversion=")[^"]*/, version]],
  // Download links; the website in docs/ reads the version from the GitHub API.
  'README.md': [
    [/(releases\/download\/v)[^/]*/g, version],
    [/(Storytel-Player-Setup-)\d+\.\d+\.\d+/g, version],
    [/(Storytel-Player-)\d+\.\d+\.\d+(?=(?:-[a-z0-9]+)*\.(?:dmg|AppImage))/g, version],
  ],
};

const check = process.argv.includes('--check');
let failed = false;

for (const [file, rules] of Object.entries(files)) {
  const path = resolve(file);
  const before = readFileSync(path, 'utf-8');
  let after = before;
  for (const [regex, value] of rules) {
    if (!regex.test(after)) {
      console.error(`${file}: no match for ${regex}`);
      failed = true;
    }
    after = after.replace(regex, (_, keep) => keep + value);
  }
  if (after === before) continue;
  if (check) {
    console.error(`${file}: not at ${version}`);
    failed = true;
  } else {
    writeFileSync(path, after);
    console.log(`updated ${file}`);
  }
}

if (failed) process.exit(1);
console.log(check ? `all files at ${version}` : `synced to ${version}`);
