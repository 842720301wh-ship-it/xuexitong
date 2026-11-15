import { spawnSync } from 'node:child_process'
for (const args of [['test', './...'], ['vet', './...']]) {
  const r = spawnSync('go', args, { cwd: 'backend', stdio: 'inherit' })
  if (r.error) console.error(r.error.message)
  if (r.status !== 0) process.exit(r.status ?? 1)
}
