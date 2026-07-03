import fs from 'node:fs'
import path from 'node:path'

const root = process.cwd()
const pagePath = path.join(root, 'src/pages/Docs/ApiDocs.tsx')
const routePath = path.join(root, 'src/routes/docs.tsx')

const failures = []

function readRequired(filePath, label) {
  if (!fs.existsSync(filePath)) {
    failures.push(`${label} missing: ${path.relative(root, filePath)}`)
    return ''
  }
  return fs.readFileSync(filePath, 'utf8')
}

function requireIncludes(source, needle, label) {
  if (!source.includes(needle)) {
    failures.push(`${label}: missing ${needle}`)
  }
}

const page = readRequired(pagePath, 'API docs page')
const route = readRequired(routePath, 'Docs route')

if (page) {
  const requiredPageSnippets = [
    'export function ApiDocs',
    'Hero',
    'QuickStart',
    'ApiReference',
    'CopyLLMButton',
    'llmCopyText',
    'FAQ',
    'CodeBlock',
    'CodeBlockCopyButton',
    'sk-YOUR_API_KEY',
    '#f4f1e8',
    '#c8432a',
    '#15130d',
    '#e1542f',
    'Fraunces',
    'Inter',
    'IBM Plex Mono',
    'POST /v1/chat/completions',
    'POST /v1/images/tasks',
    'POST /v1/videos/tasks',
    'GET /v1/tasks/:id',
    'GET /v1/tasks/:id/content',
    'POST /v1/tasks/:id/cancel',
    'gpt-image-2',
    'seedance-2.0',
    '测试异步任务',
    '复制 LLM 文本',
  ]

  for (const snippet of requiredPageSnippets) {
    requireIncludes(page, snippet, 'API docs page')
  }

  const prohibitedPageSnippets = [
    '支持模型与价格',
    "href: '#models'",
    '测试图片异步任务',
    '模型价格',
    '¥0.11',
    '¥0.12',
    '¥0.18',
    '¥0.28',
    '¥0.29',
    '¥0.32',
    '¥0.42',
    '¥0.49',
  ]

  for (const snippet of prohibitedPageSnippets) {
    if (page.includes(snippet)) {
      failures.push(`API docs page should not include ${snippet}`)
    }
  }

  const leakedKeys = page.match(/sk-[A-Za-z0-9]{8,}/g) ?? []
  const unexpectedKeys = leakedKeys.filter((value) => value !== 'sk-YOUR_API_KEY')
  if (unexpectedKeys.length > 0) {
    failures.push(`API docs page contains unexpected API key-like values: ${unexpectedKeys.join(', ')}`)
  }
}

if (route) {
  requireIncludes(route, "createFileRoute('/docs')", 'Docs route')
  requireIncludes(route, 'ApiDocs', 'Docs route')
}

if (failures.length > 0) {
  console.error('API docs page verification failed:')
  for (const failure of failures) {
    console.error(`- ${failure}`)
  }
  process.exit(1)
}

console.log('API docs page verification passed.')
