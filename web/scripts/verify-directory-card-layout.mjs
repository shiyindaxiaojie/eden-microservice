import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const views = [
  { file: 'cluster.vue', card: 'node-card.node-card', grid: '.card-grid.card-grid' },
  { file: 'rbac.vue', card: 'info-card', grid: '.card-grid' },
  { file: 'namespace.vue', card: 'info-card', grid: '.card-grid' },
]

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

for (const view of views) {
  const source = await readFile(resolve('src/views', view.file), 'utf8')

  assert(source.includes('const pageSize = ref(8)'), `${view.file}: default page size must be 8`)
  assert(source.includes(':page-sizes="[8, 16, 32, 64]"'), `${view.file}: page-size options must be 8, 16, 32, 64`)
  assert(
    new RegExp(`${escapeRegex(view.grid)}\\s*\\{[^}]*grid-template-columns: repeat\\(4, minmax\\(0, 1fr\\)\\);`).test(source),
    `${view.file}: desktop cards must use four columns`,
  )
  assert(source.includes(`${view.card}:focus-within`), `${view.file}: cards must have a keyboard-focus selection state`)
  assert(source.includes("'is-selected'"), `${view.file}: cards must retain a selected state after click`)
  assert(source.includes('.card-actions .el-button') && source.includes('font-size: 12px !important'), `${view.file}: edit action font must not exceed card body text`)
}

console.log('Directory card layout contract passed.')
