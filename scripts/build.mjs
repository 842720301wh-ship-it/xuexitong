import { spawnSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const args = new Set(process.argv.slice(2))
const allowed = new Set(['--desktop', '--android', '--offline'])
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', windowsHide: true, ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} 构建失败（退出码 ${result.status ?? 'unknown'}）`)
}

try {
  for (const arg of args) if (!allowed.has(arg)) throw new Error(`未知构建参数：${arg}`)
  if (args.has('--desktop') && args.has('--android')) throw new Error('--desktop 和 --android 不能同时使用')
  const android = !args.has('--desktop')
  const desktop = !args.has('--android')
  if (android && process.platform !== 'win32') throw new Error('Android 打包脚本需要 Windows；电脑版请执行 npm run build:desktop。')
  if (android) {
    // Fail before building if the Android toolchain is missing.
    run('powershell.exe', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', 'scripts/build-android.ps1', '-CheckTools'])
  }
  run(process.execPath, ['--run', 'build'], { cwd: path.join(root, 'frontend') })
  if (desktop) {
    mkdirSync(path.join(root, 'dist/backend'), { recursive: true })
    const output = path.join(root, 'dist/backend', process.platform === 'win32' ? 'tiantianxiangshang.exe' : 'tiantianxiangshang')
    run('go', ['build', '-buildvcs=false', '-trimpath', '-o', output, './cmd/server'], { cwd: path.join(root, 'backend') })
  }
  if (android) {
    run('powershell.exe', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', 'scripts/build-android.ps1', ...(args.has('--offline') ? ['-Offline'] : [])])
  }
} catch (error) {
  console.error(error.message)
  process.exitCode = 1
}
