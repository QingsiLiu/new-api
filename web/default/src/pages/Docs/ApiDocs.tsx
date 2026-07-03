/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import type { BundledLanguage } from 'shiki/bundle/web'
import {
  ArrowRight,
  Check,
  Clock3,
  Copy,
  Download,
  FileText,
  KeyRound,
  MessageSquareText,
  Play,
  RefreshCw,
  WalletCards,
} from 'lucide-react'
import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type CodeExample = {
  title: string
  language: BundledLanguage
  code: string
}

const apiKeySetup = `export GEILI_API_KEY="sk-YOUR_API_KEY"
export GEILI_BASE_URL="https://all.geiliapi.com"
export GEILI_TEXT_MODEL_ID="\${GEILI_TEXT_MODEL_ID:-gpt-4o}"  # 示例；实际以 /v1/models 为准`

const textCurl = `curl -sS "$GEILI_BASE_URL/v1/chat/completions" \\
  -H "Authorization: Bearer $GEILI_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "'$GEILI_TEXT_MODEL_ID'",
    "messages": [
      {"role": "user", "content": "Reply with exactly: geili-ok"}
    ],
    "stream": false
  }'`

const textPython = `import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["GEILI_API_KEY"],
    base_url="https://all.geiliapi.com/v1",
)

response = client.chat.completions.create(
    model=os.environ["GEILI_TEXT_MODEL_ID"],
    messages=[{"role": "user", "content": "Reply with exactly: geili-ok"}],
)

print(response.choices[0].message.content)`

const asyncTaskCurl = `# 1. 提交异步任务（图片示例）
curl -sS "$GEILI_BASE_URL/v1/images/tasks" \\
  -H "Authorization: Bearer $GEILI_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "action": "generate",
    "model": "gpt-image-2",
    "input": {"prompt": "一只猫在弹钢琴"},
    "parameters": {"size": "1024x1024", "n": 1}
  }'

# 2. 查询任务，直到 status=succeeded
curl -sS "$GEILI_BASE_URL/v1/tasks/task_abc123" \\
  -H "Authorization: Bearer $GEILI_API_KEY"

# 3. 下载结果
curl -L "$GEILI_BASE_URL/v1/tasks/task_abc123/content" \\
  -H "Authorization: Bearer $GEILI_API_KEY" \\
  -o image.png`

const chatRequest = `{
  "model": "<TEXT_MODEL_ID>",
  "messages": [
    {"role": "system", "content": "你是一个简洁的助手"},
    {"role": "user", "content": "你好"}
  ],
  "stream": false
}`

const chatResponse = `{
  "id": "chatcmpl-...",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "你好！有什么可以帮你的吗？"
      }
    }
  ],
  "usage": {"total_tokens": 27}
}`

const videoTaskRequest = `{
  "action": "generate",
  "model": "seedance-2.0",
  "input": {"prompt": "湖边日落，镜头缓慢推进"},
  "parameters": {
    "duration": 5,
    "ratio": "16:9",
    "watermark": false
  }
}`

const taskResponse = `{
  "id": "task_abc123",
  "kind": "video",
  "model": "seedance-2.0",
  "status": "queued",
  "progress": "0%",
  "createdAt": 1782980000
}`

const editorialFonts = {
  body: "'Inter', var(--font-sans)",
  display: "'Fraunces', var(--font-serif)",
  mono: "'IBM Plex Mono', var(--font-mono)",
}

const navItems = [
  { href: '#quickstart', label: '快速开始' },
  { href: '#interfaces', label: '接口说明' },
  { href: '#faq', label: 'FAQ' },
]

const quickStartSteps = [
  {
    icon: KeyRound,
    label: 'Step 01',
    title: '获取 API Key',
    body: '登录控制台，进入 API Keys，创建 key。图片和视频建议选择“生图、视频多模态专用分组”。',
    example: {
      title: '环境变量',
      language: 'bash',
      code: apiKeySetup,
    },
  },
  {
    icon: MessageSquareText,
    label: 'Step 02',
    title: '测试文本同步接口',
    body: 'Chat Completions 与 OpenAI SDK 兼容。文本模型以你的 /v1/models 返回为准。',
    example: {
      title: 'cURL',
      language: 'bash',
      code: textCurl,
    },
    secondaryExample: {
      title: 'Python OpenAI SDK',
      language: 'python',
      code: textPython,
    },
  },
  {
    icon: FileText,
    label: 'Step 03',
    title: '测试异步任务',
    body: '图片和视频走任务模式：提交任务、轮询状态、下载结果。长任务不会阻塞客户端请求。',
    example: {
      title: 'cURL',
      language: 'bash',
      code: asyncTaskCurl,
    },
  },
] satisfies Array<{
  icon: typeof KeyRound
  label: string
  title: string
  body: string
  example: CodeExample
  secondaryExample?: CodeExample
}>

const endpointRows = [
  ['POST', '/v1/chat/completions', '同步文本生成'],
  ['POST', '/v1/images/tasks', '提交图片任务'],
  ['POST', '/v1/videos/tasks', '提交视频任务'],
  ['GET', '/v1/tasks/:id', '查询任务状态'],
  ['GET', '/v1/tasks/:id/content', '下载任务结果'],
  ['POST', '/v1/tasks/:id/cancel', '取消未完成任务'],
]

const faqs = [
  {
    question: '如何计费？',
    answer:
      '文本按 token 计费；图片按张数和分辨率计费；视频价格待负责人配置。余额不足时会返回 402。',
  },
  {
    question: '可以用 OpenAI SDK 吗？',
    answer:
      '可以。文本和图片同步接口只需要把 base_url 改成 https://all.geiliapi.com/v1。产品异步任务建议直接用 HTTP 调用。',
  },
  {
    question: '异步任务多久返回？',
    answer:
      '图片通常几十秒内返回，视频可能需要 1-3 分钟。任务会返回 queued、processing、succeeded、failed、canceled 或 timeout。',
  },
  {
    question: '余额不足怎么办？',
    answer:
      '先到控制台查询余额和 token 限额；如果余额不足，进入充值页补充额度后重试。',
  },
  {
    question: '可以取消任务吗？',
    answer:
      '可以。调用 POST /v1/tasks/:id/cancel。已成功或已失败的任务不会重新取消。',
  },
  {
    question: '为什么示例没有写死文本模型？',
    answer:
      '文本模型会按 key 所属分组动态返回。请先调用 /v1/models，复制你账号可用的文本模型 ID。',
  },
]

const llmCopyText = [
  '# Geili API 接入文档（LLM 版）',
  '',
  '用途：帮助 AI 快速理解如何接入 Geili API。所有示例使用占位 key，不包含真实密钥。',
  '',
  '## 基本信息',
  '- API Base URL: https://all.geiliapi.com/v1',
  '- 认证方式: Authorization: Bearer sk-YOUR_API_KEY',
  '- 一个 API key 可接入文本、图片、视频三类能力。',
  '- 文本使用同步 Chat Completions；图片和视频使用异步任务流程。',
  '- 文本模型 ID 以当前账号调用 /v1/models 返回为准。',
  '',
  '## Quick Start',
  '1. 设置环境变量',
  '```bash',
  apiKeySetup,
  '```',
  '',
  '2. 测试文本同步接口',
  '```bash',
  textCurl,
  '```',
  '',
  '3. 使用 Python OpenAI SDK',
  '```python',
  textPython,
  '```',
  '',
  '4. 测试异步任务',
  '```bash',
  asyncTaskCurl,
  '```',
  '',
  '## 常用端点',
  '- POST /v1/chat/completions: 同步文本生成',
  '- POST /v1/images/tasks: 提交图片任务',
  '- POST /v1/videos/tasks: 提交视频任务',
  '- GET /v1/tasks/:id: 查询任务状态',
  '- GET /v1/tasks/:id/content: 下载任务结果',
  '- POST /v1/tasks/:id/cancel: 取消未完成任务',
  '',
  '## 异步任务状态',
  '- queued: 已排队',
  '- processing: 处理中',
  '- succeeded: 成功',
  '- failed: 失败',
  '- canceled: 已取消',
  '- timeout: 超时',
  '',
  '## 任务请求示例',
  '```json',
  videoTaskRequest,
  '```',
  '',
  '## 任务响应示例',
  '```json',
  taskResponse,
  '```',
  '',
  '## FAQ 摘要',
  '- 余额不足：到控制台查询余额和 token 限额，充值后重试。',
  '- 任务长时间 pending：继续轮询 GET /v1/tasks/:id；长任务可能需要几分钟。',
  '- OpenAI SDK 兼容：文本接口把 base_url 改成 https://all.geiliapi.com/v1 即可。',
  '- 取消任务：调用 POST /v1/tasks/:id/cancel；终态任务不会重新取消。',
].join('\n')

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  const didCopy = document.execCommand('copy')
  document.body.removeChild(textarea)

  if (!didCopy) {
    throw new Error('Copy failed')
  }
}

function SectionHeading(props: {
  eyebrow: string
  title: string
  description: string
}) {
  return (
    <div className='max-w-3xl'>
      <p className='font-mono text-[0.6875rem] font-medium tracking-[0.18em] text-[#c8432a] uppercase dark:text-[#e1542f]'>
        {props.eyebrow}
      </p>
      <h2 className='mt-4 font-serif text-4xl leading-none font-medium tracking-normal text-[#18160f] text-balance md:text-5xl dark:text-[#ece6d7]'>
        <span style={{ fontFamily: editorialFonts.display }}>
          {props.title}
        </span>
      </h2>
      <p className='mt-5 text-base leading-8 text-[#625d50] md:text-lg dark:text-[#aaa18c]'>
        {props.description}
      </p>
    </div>
  )
}

function DocsCodeBlock(props: CodeExample) {
  return (
    <div className='overflow-hidden rounded-md border border-[#312c22] bg-[#11100d] shadow-none'>
      <div className='flex items-center justify-between border-b border-[#312c22] px-4 py-3'>
        <span
          className='font-mono text-[0.6875rem] font-medium tracking-[0.16em] text-[#aaa18c] uppercase'
          style={{ fontFamily: editorialFonts.mono }}
        >
          {props.title}
        </span>
      </div>
      <CodeBlock
        className='rounded-none border-0 bg-[#11100d] text-[#f4f1e8] [&>div>div>pre]:max-h-[520px] [&>div>div>pre]:overflow-auto [&>div>div>pre]:bg-[#11100d]! [&>div>div>pre]:text-[#f4f1e8]!'
        code={props.code}
        language={props.language}
      >
        <CodeBlockCopyButton
          aria-label={`复制 ${props.title} 代码`}
          className='h-7 rounded-sm border border-[#4b4435] bg-[#1b1812] px-2.5 font-mono text-[0.6875rem] tracking-[0.12em] text-[#f4f1e8] uppercase hover:bg-[#2a2519]'
        >
          复制
        </CodeBlockCopyButton>
      </CodeBlock>
    </div>
  )
}

function CopyLLMButton() {
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>(
    'idle'
  )
  const Icon = copyState === 'copied' ? Check : Copy

  const handleCopy = async () => {
    try {
      await copyText(llmCopyText)
      setCopyState('copied')
    } catch {
      setCopyState('failed')
    }
    window.setTimeout(() => setCopyState('idle'), 1800)
  }

  return (
    <Button
      aria-label='复制当前文档的 LLM 文本'
      className='h-11 border-[#d2cab9] px-5 dark:border-[#3a3428]'
      onClick={handleCopy}
      variant='outline'
    >
      <Icon className='mr-1.5 size-4' />
      {copyState === 'copied'
        ? '已复制'
        : copyState === 'failed'
          ? '复制失败'
          : '复制 LLM 文本'}
    </Button>
  )
}

function Hero() {
  return (
    <section className='relative border-b border-[#dcd6c8] px-5 py-20 md:px-8 md:py-28 dark:border-[#312c22]'>
      <div className='mx-auto grid max-w-7xl gap-10 lg:grid-cols-[1fr_360px] lg:items-end'>
        <div>
          <p className='font-mono text-[0.6875rem] font-medium tracking-[0.2em] text-[#c8432a] uppercase dark:text-[#e1542f]'>
            API Docs
          </p>
      <h1 className='mt-5 max-w-5xl font-serif text-6xl leading-[0.95] font-medium tracking-normal text-[#18160f] text-balance md:text-8xl dark:text-[#ece6d7]'>
            <span style={{ fontFamily: editorialFonts.display }}>Geili API</span>
            <span className='block text-[#c8432a] dark:text-[#e1542f]'>
              一个接口，海量模型
            </span>
          </h1>
          <p className='mt-7 max-w-2xl text-lg leading-9 text-[#625d50] md:text-xl dark:text-[#aaa18c]'>
            文本 / 图片 / 视频统一接入。同步 Chat Completions 直接返回，
            图片与视频走异步任务流程，5 分钟跑通首次调用。
          </p>
          <div className='mt-9 flex flex-wrap gap-3'>
            <Button
              className='h-11 bg-[#c8432a] px-5 text-[#fbf3e6] hover:bg-[#b53a25] dark:bg-[#e1542f] dark:hover:bg-[#cc4728]'
              render={<Link to='/sign-up' />}
            >
              获取 API Key
              <ArrowRight className='ml-1.5 size-4' />
            </Button>
            <Button
              className='h-11 border-[#d2cab9] px-5 dark:border-[#3a3428]'
              variant='outline'
              render={<a href='#quickstart' />}
            >
              查看快速开始
            </Button>
            <CopyLLMButton />
          </div>
        </div>

        <div className='border-l border-[#18160f] pl-6 dark:border-[#ece6d7]'>
          <p className='font-mono text-[0.6875rem] tracking-[0.16em] text-[#8c8676] uppercase'>
            Core endpoints
          </p>
          <div className='mt-5 space-y-4'>
            {endpointRows.slice(0, 3).map((row) => (
              <div key={row[1]} className='grid grid-cols-[64px_1fr] gap-3'>
                <span className='font-mono text-xs text-[#c8432a] dark:text-[#e1542f]'>
                  {row[0]}
                </span>
                <span className='font-mono text-sm text-[#18160f] dark:text-[#ece6d7]'>
                  {row[1]}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function QuickStart() {
  return (
    <section
      className='border-b border-[#dcd6c8] px-5 py-16 md:px-8 md:py-24 dark:border-[#312c22]'
      id='quickstart'
    >
      <div className='mx-auto max-w-7xl'>
        <SectionHeading
          description='只保留首次接入需要的内容：拿 key、跑文本、跑异步任务。视频与图片使用同一套异步任务流程。'
          eyebrow='5 minute start'
          title='快速开始，三步跑通'
        />
        <div className='mt-12 grid gap-8'>
          {quickStartSteps.map((step) => {
            const Icon = step.icon
            return (
              <section
                className='grid gap-6 border-t border-[#18160f] pt-8 lg:grid-cols-[320px_1fr] dark:border-[#ece6d7]'
                key={step.title}
              >
                <div>
                  <div className='flex items-center gap-3'>
                    <span className='flex size-9 items-center justify-center rounded-full bg-[#c8432a] text-[#fbf3e6] dark:bg-[#e1542f]'>
                      <Icon className='size-4' />
                    </span>
                    <span className='font-mono text-[0.6875rem] tracking-[0.16em] text-[#8c8676] uppercase'>
                      {step.label}
                    </span>
                  </div>
                  <h3 className='mt-5 font-serif text-3xl leading-none font-medium text-[#18160f] dark:text-[#ece6d7]'>
                    {step.title}
                  </h3>
                  <p className='mt-4 text-sm leading-7 text-[#625d50] dark:text-[#aaa18c]'>
                    {step.body}
                  </p>
                </div>
                <div className='grid gap-4'>
                  <DocsCodeBlock {...step.example} />
                  {step.secondaryExample ? (
                    <DocsCodeBlock {...step.secondaryExample} />
                  ) : null}
                </div>
              </section>
            )
          })}
        </div>
      </div>
    </section>
  )
}

function FlowStep(props: {
  icon: typeof Play
  title: string
  description: string
  isLast?: boolean
}) {
  const Icon = props.icon
  return (
    <div className='relative flex gap-4'>
      <div className='flex flex-col items-center'>
        <span className='flex size-10 items-center justify-center rounded-full border border-[#d2cab9] bg-[#f8f5ec] text-[#c8432a] dark:border-[#3a3428] dark:bg-[#1b1812] dark:text-[#e1542f]'>
          <Icon className='size-4' />
        </span>
        {!props.isLast ? (
          <span className='my-2 h-full min-h-8 w-px bg-[#dcd6c8] dark:bg-[#312c22]' />
        ) : null}
      </div>
      <div className='pb-7'>
        <h4 className='font-serif text-xl leading-tight text-[#18160f] dark:text-[#ece6d7]'>
          {props.title}
        </h4>
        <p className='mt-2 text-sm leading-6 text-[#625d50] dark:text-[#aaa18c]'>
          {props.description}
        </p>
      </div>
    </div>
  )
}

function ApiReference() {
  return (
    <section
      className='border-b border-[#dcd6c8] px-5 py-16 md:px-8 md:py-24 dark:border-[#312c22]'
      id='interfaces'
    >
      <div className='mx-auto max-w-7xl'>
        <SectionHeading
          description='同步接口适合文本问答；异步接口适合图片和视频等长任务。客户端只需要同一个 Base URL 和同一个 Authorization 头。'
          eyebrow='API shape'
          title='接口说明'
        />

        <div className='mt-12 grid gap-10 xl:grid-cols-2'>
          <section className='border-t border-[#18160f] pt-8 dark:border-[#ece6d7]'>
            <div className='flex items-center justify-between gap-4'>
              <div>
                <p className='font-mono text-[0.6875rem] tracking-[0.16em] text-[#c8432a] uppercase dark:text-[#e1542f]'>
                  A. Sync
                </p>
                <h3 className='mt-3 font-serif text-3xl leading-none text-[#18160f] dark:text-[#ece6d7]'>
                  同步 Chat Completions
                </h3>
              </div>
              <span className='rounded-full border border-[#d2cab9] px-3 py-1 font-mono text-xs text-[#625d50] dark:border-[#3a3428] dark:text-[#aaa18c]'>
                POST /v1/chat/completions
              </span>
            </div>
            <p className='mt-5 text-sm leading-7 text-[#625d50] dark:text-[#aaa18c]'>
              适合聊天、摘要、结构化输出。使用 OpenAI SDK 时只改
              base_url，模型 ID 以你的 /v1/models 返回为准。
            </p>
            <div className='mt-6 grid gap-4'>
              <DocsCodeBlock
                code={chatRequest}
                language='json'
                title='Request JSON'
              />
              <DocsCodeBlock
                code={chatResponse}
                language='json'
                title='Response JSON'
              />
            </div>
          </section>

          <section className='border-t border-[#18160f] pt-8 dark:border-[#ece6d7]'>
            <div className='flex items-center justify-between gap-4'>
              <div>
                <p className='font-mono text-[0.6875rem] tracking-[0.16em] text-[#c8432a] uppercase dark:text-[#e1542f]'>
                  B. Async
                </p>
                <h3 className='mt-3 font-serif text-3xl leading-none text-[#18160f] dark:text-[#ece6d7]'>
                  异步图片 / 视频任务
                </h3>
              </div>
              <span className='rounded-full border border-[#d2cab9] px-3 py-1 font-mono text-xs text-[#625d50] dark:border-[#3a3428] dark:text-[#aaa18c]'>
                Tasks
              </span>
            </div>
            <p className='mt-5 text-sm leading-7 text-[#625d50] dark:text-[#aaa18c]'>
              图片和视频生成需要几秒到几分钟，统一走任务模型。状态值：
              queued / processing / succeeded / failed / canceled / timeout。
            </p>
            <div className='mt-7 grid gap-6 lg:grid-cols-[280px_1fr] xl:grid-cols-1 2xl:grid-cols-[280px_1fr]'>
              <div>
                <FlowStep
                  description='POST /v1/images/tasks 或 POST /v1/videos/tasks，返回 task id。'
                  icon={Play}
                  title='提交任务'
                />
                <FlowStep
                  description='GET /v1/tasks/:id，轮询直到 status=succeeded 或终态。'
                  icon={RefreshCw}
                  title='轮询查询'
                />
                <FlowStep
                  description='GET /v1/tasks/:id/content，下载图片或视频二进制。'
                  icon={Download}
                  isLast
                  title='下载结果'
                />
              </div>
              <div className='grid gap-4'>
                <DocsCodeBlock
                  code={videoTaskRequest}
                  language='json'
                  title='Video Task Request'
                />
                <DocsCodeBlock
                  code={taskResponse}
                  language='json'
                  title='Task Response'
                />
              </div>
            </div>
          </section>
        </div>

        <div className='mt-12 overflow-hidden border-t border-[#18160f] dark:border-[#ece6d7]'>
          <div className='grid grid-cols-1 md:grid-cols-3'>
            {endpointRows.map((row) => (
              <div
                className='border-b border-[#dcd6c8] px-4 py-5 md:border-r md:last:border-r-0 dark:border-[#312c22]'
                key={row[1]}
              >
                <p className='font-mono text-xs text-[#c8432a] dark:text-[#e1542f]'>
                  {row[0]}
                </p>
                <p className='mt-2 break-all font-mono text-sm text-[#18160f] dark:text-[#ece6d7]'>
                  {row[1]}
                </p>
                <p className='mt-2 text-sm text-[#625d50] dark:text-[#aaa18c]'>
                  {row[2]}
                </p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function FAQ() {
  return (
    <section className='px-5 py-16 md:px-8 md:py-24' id='faq'>
      <div className='mx-auto max-w-7xl'>
        <SectionHeading
          description='围绕首次接入最容易卡住的地方，保持短答案。完整错误码和高级配置请看长篇技术文档。'
          eyebrow='FAQ'
          title='常见问题'
        />
        <div className='mt-12 grid gap-0 border-t border-[#18160f] dark:border-[#ece6d7]'>
          {faqs.map((item, index) => (
            <details
              className='group border-b border-[#dcd6c8] py-5 dark:border-[#312c22]'
              key={item.question}
              open={index === 0}
            >
              <summary className='flex cursor-pointer list-none items-center justify-between gap-4'>
                <span className='font-serif text-2xl leading-tight text-[#18160f] dark:text-[#ece6d7]'>
                  {item.question}
                </span>
                <span className='font-mono text-xs text-[#c8432a] group-open:hidden dark:text-[#e1542f]'>
                  OPEN
                </span>
                <span className='hidden font-mono text-xs text-[#8c8676] group-open:inline'>
                  CLOSE
                </span>
              </summary>
              <p className='mt-4 max-w-3xl text-sm leading-7 text-[#625d50] dark:text-[#aaa18c]'>
                {item.answer}
              </p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}

function SideNav() {
  return (
    <nav
      aria-label='API 文档目录'
      className='sticky top-8 hidden h-fit border-l border-[#dcd6c8] pl-5 lg:block dark:border-[#312c22]'
    >
      <p className='font-mono text-[0.6875rem] tracking-[0.16em] text-[#8c8676] uppercase'>
        On this page
      </p>
      <div className='mt-5 flex flex-col gap-3'>
        {navItems.map((item) => (
          <a
            className='text-sm text-[#625d50] transition-colors hover:text-[#c8432a] dark:text-[#aaa18c] dark:hover:text-[#e1542f]'
            href={item.href}
            key={item.href}
          >
            {item.label}
          </a>
        ))}
      </div>
    </nav>
  )
}

function Footer() {
  return (
    <footer className='border-t border-[#18160f] px-5 py-10 md:px-8 dark:border-[#ece6d7]'>
      <div className='mx-auto flex max-w-7xl flex-col gap-5 md:flex-row md:items-center md:justify-between'>
        <div>
          <p className='font-serif text-2xl text-[#18160f] dark:text-[#ece6d7]'>
            Geili API
          </p>
          <p className='mt-2 text-sm text-[#625d50] dark:text-[#aaa18c]'>
            一个 API key 接入文本、图片、视频。
          </p>
        </div>
        <div className='flex flex-wrap gap-4 text-sm'>
          <Link
            className='text-[#625d50] hover:text-[#c8432a] dark:text-[#aaa18c] dark:hover:text-[#e1542f]'
            to='/dashboard'
          >
            控制台
          </Link>
          <a
            className='text-[#625d50] hover:text-[#c8432a] dark:text-[#aaa18c] dark:hover:text-[#e1542f]'
            href='https://all.geiliapi.com/api/status'
            rel='noopener noreferrer'
            target='_blank'
          >
            API 状态页
          </a>
        </div>
      </div>
    </footer>
  )
}

function EditorialMetric(props: {
  label: string
  value: string
  icon: typeof Check
}) {
  const Icon = props.icon
  return (
    <div className='border-t border-[#18160f] pt-4 dark:border-[#ece6d7]'>
      <div className='flex items-center gap-2 text-[#c8432a] dark:text-[#e1542f]'>
        <Icon className='size-4' />
        <span className='font-mono text-[0.6875rem] tracking-[0.16em] uppercase'>
          {props.label}
        </span>
      </div>
      <p className='mt-3 font-serif text-3xl leading-none text-[#18160f] dark:text-[#ece6d7]'>
        {props.value}
      </p>
    </div>
  )
}

export function ApiDocs() {
  return (
    <main
      className={cn(
        'min-h-screen bg-[#f4f1e8] text-[#18160f] dark:bg-[#15130d] dark:text-[#ece6d7]',
        'font-[Inter,ui-sans-serif,system-ui] selection:bg-[#c8432a]/20 dark:selection:bg-[#e1542f]/25'
      )}
      style={{ fontFamily: editorialFonts.body }}
    >
      <Hero />
      <div className='mx-auto grid max-w-[1600px] min-w-0 lg:grid-cols-[minmax(0,1fr)_220px]'>
        <div className='min-w-0'>
          <section className='border-b border-[#dcd6c8] px-5 py-8 md:px-8 dark:border-[#312c22]'>
            <div className='mx-auto grid max-w-7xl gap-5 md:grid-cols-3'>
              <EditorialMetric icon={Check} label='Base URL' value='/v1' />
              <EditorialMetric icon={Clock3} label='First call' value='5 min' />
              <EditorialMetric icon={WalletCards} label='Billing' value='quota' />
            </div>
          </section>
          <QuickStart />
          <ApiReference />
          <FAQ />
          <Footer />
        </div>
        <aside className='hidden px-8 py-16 lg:block'>
          <SideNav />
        </aside>
      </div>
    </main>
  )
}

export default ApiDocs
