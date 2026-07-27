import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'

const projectRoot = path.resolve(import.meta.dirname, '..')
const srcRoot = path.join(projectRoot, 'src')
const i18nPath = path.join(srcRoot, 'utils', 'i18n.ts')
const sourceFiles = []
collectSourceFiles(srcRoot, sourceFiles)

const uiFiles = sourceFiles.filter((file) => file !== i18nPath)
const localeBranchPattern = /locale(?:\.value)?\s*[!=]={1,2}\s*['"`](?:zh|en|ja)['"`]|\bisZh\b|const\s+text\s*=/
const mojibakePattern =
  /鑺|鏈|褰|闆|鏃|瀹炰|鏀跺|淇℃|閿欒|璁块|鎼滅|缁撴|寮€|琛屾|鍏ㄩ|鍐呭|娉㈠|閲囨|鍋ュ|绯荤|鍗曟|閫夋|涓婄|涓嬬|绉婚|鍚屾|娉ㄥ|杈\?|卤/

const errors = []

for (const file of uiFiles) {
  const content = fs.readFileSync(file, 'utf8')
  const rel = path.relative(projectRoot, file)

  for (const [index, line] of content.split(/\r?\n/).entries()) {
    if (localeBranchPattern.test(line)) {
      errors.push(`${rel}:${index + 1} contains inline locale branching or inline text() pair`)
    }

    if (mojibakePattern.test(line)) {
      errors.push(`${rel}:${index + 1} contains suspected mojibake text`)
    }
  }
}

const i18nSource = fs.readFileSync(i18nPath, 'utf8')

if (!/supportedLocales\s*=\s*\[\s*'en'\s*,\s*'zh'\s*,\s*'ja'\s*\]/.test(i18nSource)) {
  errors.push('src/utils/i18n.ts must declare supportedLocales with en, zh, and ja')
}

if (!/\bjaMessages\b/.test(i18nSource) || !/\bja:\s*jaMessages\b/.test(i18nSource)) {
  errors.push('src/utils/i18n.ts must wire jaMessages into localizedMessages')
}

const messagesInitializer = findMessagesInitializer(i18nSource)
if (!messagesInitializer) {
  errors.push('src/utils/i18n.ts does not export a messages object')
} else {
  const localeTrees = new Map()
  for (const locale of ['en', 'zh']) {
    const localeNode = findProperty(messagesInitializer, locale)
    if (!localeNode) {
      errors.push(`src/utils/i18n.ts is missing locale "${locale}"`)
      continue
    }
    localeTrees.set(locale, collectLeafKeys(localeNode))
  }

  if (localeTrees.size === 2) {
    const [baseLocale, ...otherLocales] = ['en', 'zh']
    const baseKeys = localeTrees.get(baseLocale)

    for (const locale of otherLocales) {
      const localeKeys = localeTrees.get(locale)
      for (const key of baseKeys) {
        if (!localeKeys.has(key)) {
          errors.push(`src/utils/i18n.ts locale "${locale}" is missing key "${key}"`)
        }
      }
      for (const key of localeKeys) {
        if (!baseKeys.has(key)) {
          errors.push(`src/utils/i18n.ts locale "${locale}" has extra key "${key}"`)
        }
      }
    }
  }
}

if (errors.length > 0) {
  console.error(`i18n check failed with ${errors.length} issue(s):`)
  for (const error of errors) console.error(`- ${error}`)
  process.exit(1)
}

console.log('i18n check passed')

function collectSourceFiles(dir, files) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      collectSourceFiles(fullPath, files)
    } else if (/\.(ts|vue)$/.test(entry.name)) {
      files.push(fullPath)
    }
  }
}

function findMessagesInitializer(source) {
  const file = ts.createSourceFile(i18nPath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  let initializer = null

  function visit(node) {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.name.text === 'messages' &&
      node.initializer
    ) {
      initializer = unwrapExpression(node.initializer)
      return
    }
    ts.forEachChild(node, visit)
  }

  visit(file)
  return initializer
}

function unwrapExpression(node) {
  if (ts.isAsExpression(node) || ts.isParenthesizedExpression(node)) {
    return unwrapExpression(node.expression)
  }
  return node
}

function findProperty(objectNode, name) {
  if (!ts.isObjectLiteralExpression(objectNode)) return null

  for (const property of objectNode.properties) {
    if (!ts.isPropertyAssignment(property)) continue
    if (propertyName(property.name) !== name) continue
    return unwrapExpression(property.initializer)
  }

  return null
}

function collectLeafKeys(node, prefix = '') {
  const keys = new Set()
  if (!ts.isObjectLiteralExpression(node)) {
    keys.add(prefix)
    return keys
  }

  for (const property of node.properties) {
    if (!ts.isPropertyAssignment(property)) continue
    const name = propertyName(property.name)
    if (!name) continue
    const propertyPrefix = prefix ? `${prefix}.${name}` : name
    const childKeys = collectLeafKeys(unwrapExpression(property.initializer), propertyPrefix)
    for (const key of childKeys) keys.add(key)
  }

  return keys
}

function propertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text
  }
  return null
}
