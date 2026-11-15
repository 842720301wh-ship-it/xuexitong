// Extract DEX members and inventory relevant URL literals without executing APK code.
const fs = require('node:fs');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');
fs.mkdirSync('.cache/apk/dex', { recursive: true });
const extraction = spawnSync('powershell.exe', ['-NoProfile', '-Command', `Add-Type -AssemblyName System.IO.Compression.FileSystem; $z=[IO.Compression.ZipFile]::OpenRead((Resolve-Path -LiteralPath 'tool/base.apk')); try { foreach($e in $z.Entries) { if($e.FullName -match '^classes\\d*\\.dex$') { [IO.Compression.ZipFileExtensions]::ExtractToFile($e,(Join-Path (Resolve-Path -LiteralPath '.cache/apk/dex') $e.Name),$true) } } } finally { $z.Dispose() }`], { stdio: 'inherit' });
if (extraction.status !== 0) process.exit(extraction.status ?? 1);
const sample = fs.readFileSync('tool/base.apk');
const dex = fs.readFileSync('.cache/apk/dex/classes.dex');
const text = dex.toString('latin1');
const found = [];
for (const match of text.matchAll(/https?:\/\/[^\x00-\x20"'<>\\]{1,200}/g)) {
  const url = match[0];
  if (/chaoxing\.com/.test(url) && /userLogin|accountManage|getUserInfo|fanyalogin/.test(url)) found.push({ dex_offset: match.index, hex_offset: '0x' + match.index.toString(16), url });
}
const result = { apk: 'tool/base.apk', apk_sha256: crypto.createHash('sha256').update(sample).digest('hex'), dex_sha256: crypto.createHash('sha256').update(dex).digest('hex'), evidence: found };
fs.mkdirSync('docs/research', { recursive: true });
fs.writeFileSync('docs/research/apk-auth-endpoints.json', JSON.stringify(result, null, 2) + '\n');
console.log(JSON.stringify(result, null, 2));
