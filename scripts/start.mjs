import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
const root = fileURLToPath(new URL('../', import.meta.url))
const result = spawnSync(process.platform === 'win32' ? 'dist/backend/tiantianxiangshang.exe' : './dist/backend/tiantianxiangshang', ['-addr', `127.0.0.1:${process.env.TIANTIAN_PORT || 18081}`, '-data', 'data', '-web', 'dist/web'], { cwd: root, stdio: 'inherit' })
if (result.error) console.error('请先执行 npm run build:desktop 或 npm run build。', result.error.message)
process.exitCode = result.status ?? 1
