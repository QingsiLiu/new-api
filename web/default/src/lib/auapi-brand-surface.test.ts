import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { describe, test } from 'node:test'

const localeFiles = ['en', 'zh', 'zh-TW', 'fr', 'ru', 'ja', 'vi'] as const

describe('AUAPI admin brand surfaces', () => {
  test('theme preset labels keep compatibility IDs but expose the AUAPI brand', async () => {
    const source = await readFile(
      new URL('./theme-customization.ts', import.meta.url),
      'utf8'
    )
    assert.match(source, /value: 'geili-minimal'/)
    assert.match(source, /name: '极简 \/ AUAPI Minimal'/)
    assert.match(source, /name: 'AUAPI Editorial'/)
    assert.doesNotMatch(source, /name: .*Geili/)
  })

  test('all shipped locales translate the three compatibility preset IDs as AUAPI', async () => {
    for (const locale of localeFiles) {
      const text = await readFile(
        new URL(`../i18n/locales/${locale}.json`, import.meta.url),
        'utf8'
      )
      const messages = (JSON.parse(text) as {
        translation: Record<string, string>
      }).translation
      for (const key of [
        'preset.geili-editorial',
        'preset.geili-minimal',
        'preset.geili-modern',
      ]) {
        assert.match(messages[key], /AUAPI/, `${locale}:${key}`)
        assert.doesNotMatch(messages[key], /Geili/, `${locale}:${key}`)
      }
    }
  })
})
