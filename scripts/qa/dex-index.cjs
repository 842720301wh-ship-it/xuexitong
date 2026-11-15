// Read-only DEX inventory for the user-supplied APK. No code from the APK is executed.
const fs = require('node:fs');
const file = '.cache/apk/dex/classes.dex';
const b = fs.readFileSync(file);
if (b.subarray(0, 4).toString() !== 'dex\n') throw new Error('Not DEX');
const strings = [];
const count = b.readUInt32LE(0x38), offset = b.readUInt32LE(0x3c);
for (let i = 0; i < count; i++) {
  let start = b.readUInt32LE(offset + i * 4);
  while (b[start++] & 0x80) {}
  const end = b.indexOf(0, start);
  strings.push(b.subarray(start, end).toString('utf8').replace(/\r?\n/g, '\\n'));
}
fs.writeFileSync('.cache/apk/strings.txt', strings.map((s, i) => `${i}\t${s}`).join('\n'));
const classes = [];
const typeOffset = b.readUInt32LE(0x44);
for (let i = 0, n = b.readUInt32LE(0x60), at = b.readUInt32LE(0x64); i < n; i++, at += 32) {
  const typeID = b.readUInt32LE(at);
  classes.push(strings[b.readUInt32LE(typeOffset + typeID * 4)]);
}
fs.writeFileSync('.cache/apk/classes.txt', classes.join('\n'));
console.log(JSON.stringify({ bytes: b.length, strings: count, classes: classes.length }));
