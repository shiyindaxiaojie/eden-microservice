import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const views = [
  { file: 'cluster.vue', card: 'node-card.node-card', grid: '.card-grid.card-grid', columns: 4 },
  { file: 'rbac.vue', card: 'info-card', grid: '.card-grid', columns: 4 },
  { file: 'namespace.vue', card: 'info-card', grid: '.card-grid', columns: 4 },
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
    new RegExp(`${escapeRegex(view.grid)}\\s*\\{[^}]*grid-template-columns: repeat\\(${view.columns}, minmax\\(0, 1fr\\)\\);`).test(source),
    `${view.file}: desktop cards must use ${view.columns} columns`,
  )
  assert(source.includes(`${view.card}:focus-within`), `${view.file}: cards must have a keyboard-focus selection state`)
  assert(source.includes("'is-selected'"), `${view.file}: cards must retain a selected state after click`)
  assert(source.includes('.card-actions .el-button') && source.includes('font-size: 12px !important'), `${view.file}: edit action font must not exceed card body text`)
}

const clusterSource = await readFile(resolve('src/views', 'cluster.vue'), 'utf8')
assert(clusterSource.includes('nodeToneClass(member)'), 'cluster.vue: every node card must receive a stable palette tone')
assert(clusterSource.includes('class="node-address"'), 'cluster.vue: node address must have a dedicated row')
assert(clusterSource.includes('class="node-state"'), 'cluster.vue: node status and role must share a dedicated state group')
assert(clusterSource.includes('class="endpoint-list"'), 'cluster.vue: protocol endpoints must use a dedicated summary list')
assert(!clusterSource.includes('class="card-symbol"'), 'cluster.vue: compact node cards must not reserve space for a decorative icon tile')
const activeClusterCardStyles = clusterSource.slice(clusterSource.lastIndexOf('/* Active node-card composition. */'))
assert(
  activeClusterCardStyles.includes('border: 1px solid var(--node-border, var(--border-color));') &&
    /\.node-card\.node-card\s*\{[^}]*background: var\(--bg-card\);/.test(activeClusterCardStyles),
  'cluster.vue: node cards must use a restrained outline on the standard card surface',
)
assert(
  activeClusterCardStyles.includes('grid-template-columns: repeat(4, minmax(0, 1fr));'),
  'cluster.vue: the active desktop node grid must fit four cards per row',
)
assert(
  /\.endpoint-list\s*\{[^}]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/.test(activeClusterCardStyles),
  'cluster.vue: two or four endpoints must fit in a two-column summary',
)
assert(activeClusterCardStyles.includes('inset: 12px auto 12px 0;'), 'cluster.vue: node state must use a compact vertical status rail')
assert(
  /\.card-body\s*\{[^}]*background: transparent;/.test(activeClusterCardStyles),
  'cluster.vue: endpoint area must blend into the card instead of using a separate panel',
)
assert(
  /\.card-body\s*\{[^}]*border-top: 0;/.test(activeClusterCardStyles),
  'cluster.vue: endpoint area must not be separated by another border',
)
assert(
  /\.endpoint-item\s*\{[^}]*border-radius: 6px;/.test(activeClusterCardStyles),
  'cluster.vue: every protocol endpoint must render as a compact tag block',
)
assert(!activeClusterCardStyles.includes('.endpoint-kind::before'), 'cluster.vue: endpoint tags must not keep decorative bullet markers')
assert(
  /\.endpoint-list\s*\{[^}]*min-height: 56px;/.test(activeClusterCardStyles),
  'cluster.vue: endpoint summary must reserve stable space for two or four ports',
)
assert(activeClusterCardStyles.includes('overflow-y: hidden;'), 'cluster.vue: eight cards must not create an inner vertical scrollbar')

for (const file of ['rbac.vue', 'namespace.vue']) {
  const source = await readFile(resolve('src/views', file), 'utf8')
  assert(source.includes('card-head-actions'), `${file}: edit controls must live in the card header`)
  assert(!source.includes('class="card-footer"'), `${file}: card actions must not reserve a separate footer row`)
  assert(source.includes(':icon="EditPen"'), `${file}: edit must use a compact icon action`)
  assert(source.includes(':icon="Delete"'), `${file}: delete must use a compact icon action`)
  assert(
    /\.info-card\s*\{[^}]*min-height: 132px;[^}]*background: var\(--bg-card\);/.test(source),
    `${file}: directory cards must use a compact standard card surface`,
  )
  assert(
    /\.info-card\.is-selected\s*\{[^}]*background: var\(--bg-card\);/.test(source),
    `${file}: selected cards must not be covered by a tinted background`,
  )
}

const rbacSource = await readFile(resolve('src/views', 'rbac.vue'), 'utf8')
assert(rbacSource.includes('updatedAt: formatUserUpdatedAt(user.updated_at)'), 'rbac.vue: user cards must map the persisted update time')
assert(rbacSource.includes("text('更新', 'Updated')"), 'rbac.vue: user cards must display update time')
assert(
  /\.pill-group\s*\{[^}]*background: transparent;/.test(rbacSource),
  'rbac.vue: access filters must not use a shared gray tray',
)
assert(
  /\.add-btn\s*\{[^}]*background: var\(--toolbar-action-bg\);/.test(rbacSource),
  'rbac.vue: the add-user command must use the emphasized action color',
)

const namespaceSource = await readFile(resolve('src/views', 'namespace.vue'), 'utf8')
assert(
  namespaceSource.includes(`:label="text('更新时间', 'Updated At')"`) && namespaceSource.includes('{{ displayUpdatedAt(row) }}'),
  'namespace.vue: namespace list must display update time',
)
assert(
  namespaceSource.includes(`:label="text('操作', 'Actions')" width="120" fixed="right"`),
  'namespace.vue: namespace actions must keep a compact fixed column beside both timestamps',
)

for (const file of ['services.vue', 'configs.vue', 'routes.vue', 'namespace.vue', 'cluster.vue', 'rbac.vue', 'settings.vue']) {
  const source = await readFile(resolve('src/views', file), 'utf8')
  assert(
    /\.pill-group\s*\{[^}]*background: transparent;/.test(source),
    `${file}: toolbar filters must not use a shared background tray`,
  )
  assert(
    /\.pill-group button\.active\s*\{[^}]*background: var\(--toolbar-active-bg\);[^}]*color: var\(--toolbar-active-text\);/.test(source),
    `${file}: toolbar filters must use the shared purple active state`,
  )
}

const themeSource = await readFile(resolve('src/styles', 'main.scss'), 'utf8')
assert(
  themeSource.includes('--toolbar-active-text: #5146a8;') && themeSource.includes('--toolbar-active-text: #b9aff2;'),
  'main.scss: toolbar active colors must remain legible in light and dark themes',
)

console.log('Directory card layout contract passed.')
