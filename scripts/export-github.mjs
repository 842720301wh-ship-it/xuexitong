import { copyFileSync, existsSync, lstatSync, mkdirSync, mkdtempSync, readdirSync, realpathSync, renameSync, rmSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = realpathSync(fileURLToPath(new URL('../', import.meta.url)))
const release = path.join(root, 'release')
const output = path.join(release, 'github-upload')

// Keep the export small and reproducible. New top-level files/directories must
// be reviewed here before they become part of the public source package.
const entries = [
  'README.md', '.gitignore', 'package.json', 'package-lock.json',
  'frontend', 'backend', 'android', 'scripts',
  'docs/usage.md', 'docs/github-upload.md', 'docs/screenshots',
]
for (const name of ['LICENSE', 'LICENSE.md', 'LICENSE.txt', 'NOTICE', 'NOTICE.md']) {
  if (existsSync(path.join(root, name))) entries.push(name)
}
const excludedDirectories = new Set([
  '.git', 'node_modules', 'dist', 'build', 'data', '.cache', '.tools',
  '.workbench-local', '.gradle', 'jniLibs', '__pycache__', '.venv', 'venv',
  '.idea', '.vscode', 'release', 'tool', 'work',
])
const excludedNames = new Set(['local.properties', 'cookies.txt', 'config.ini', '.DS_Store', 'Thumbs.db'])
const excludedExtensions = /\.(?:exe|dll|so|apk|aab|apks|tsbuildinfo|enc|key|pem|jks|keystore|p12|pfx|log|har|pcap|pcapng|pyc|pyo|pyd)$/i

function isExcluded(name) {
  return excludedNames.has(name) || excludedExtensions.test(name) ||
    ((name === '.env' || name.startsWith('.env.')) && !['.env.example', '.env.sample'].includes(name))
}

function assertInside(candidate, parent) {
  const relative = path.relative(parent, candidate)
  if (!relative || relative === '..' || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
    throw new Error(`路径不在预期目录内：${candidate}`)
  }
}

function assertDirectory(directory) {
  if (!existsSync(directory)) return
  const stat = lstatSync(directory)
  if (stat.isSymbolicLink() || !stat.isDirectory() || realpathSync(directory) !== directory) {
    throw new Error(`输出目录必须是项目内的普通目录：${directory}`)
  }
}

let count = 0
function copySource(relative, staging) {
  const source = path.join(root, relative)
  assertInside(source, root)
  const name = path.basename(relative)
  if (isExcluded(name) || excludedDirectories.has(name)) return
  const stat = lstatSync(source)
  // Never follow junctions or links into local data or outside the workspace.
  if (stat.isSymbolicLink()) throw new Error(`请先检查源码中的符号链接：${relative}`)
  if (stat.isDirectory()) {
    for (const child of readdirSync(source).sort()) copySource(path.join(relative, child), staging)
  } else if (stat.isFile()) {
    const target = path.join(staging, relative)
    assertInside(target, staging)
    mkdirSync(path.dirname(target), { recursive: true })
    copyFileSync(source, target)
    count++
  }
}

let staging
try {
  assertInside(release, root)
  assertInside(output, release)
  assertDirectory(release)
  mkdirSync(release, { recursive: true })
  assertDirectory(output)
  if (existsSync(path.join(output, '.git'))) {
    throw new Error('上传目录已含 Git 仓库，拒绝覆盖。请先将仓库移到独立工作目录并保留历史。')
  }
  staging = mkdtempSync(path.join(release, '.github-upload-'))
  for (const entry of entries) copySource(entry, staging)
  // Only replace this generated directory after the complete copy succeeds.
  assertDirectory(release)
  assertDirectory(output)
  if (existsSync(output)) rmSync(output, { recursive: true })
  renameSync(staging, output)
  staging = undefined
  console.log(`GitHub source ready: ${output}`)
  console.log(`已导出 ${count} 个文件。请上传此目录内的内容，README.md 应位于仓库根目录。`)
} catch (error) {
  console.error(error.message)
  process.exitCode = 1
} finally {
  if (staging) {
    assertInside(staging, release)
    assertDirectory(release)
    assertDirectory(staging)
    rmSync(staging, { recursive: true })
  }
}
