import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'
import { splitStatValueText } from './stat-value-parts'

const indexCss = readFileSync('src/styles/index.css', 'utf8')
const statValueSource = readFileSync('src/components/stat-value.tsx', 'utf8')

function readUtility(name: string) {
  const match = indexCss.match(
    new RegExp(`@utility ${name} \\{([\\s\\S]*?)\\n\\}`, 'm')
  )
  assert.ok(match, `missing @utility ${name}`)
  return match[1]
}

describe('splitStatValueText', () => {
  test('separates leading currency symbols', () => {
    assert.deepEqual(splitStatValueText('¥1.23k'), {
      currency: '¥',
      main: '1.23k',
      unit: undefined,
    })
  })

  test('separates compact metric units', () => {
    assert.deepEqual(splitStatValueText('42t/s'), {
      currency: undefined,
      main: '42',
      unit: 't/s',
    })
    assert.deepEqual(splitStatValueText('98%'), {
      currency: undefined,
      main: '98',
      unit: '%',
    })
    assert.deepEqual(splitStatValueText('1.2s'), {
      currency: undefined,
      main: '1.2',
      unit: 's',
    })
  })

  test('keeps plain values intact', () => {
    assert.deepEqual(splitStatValueText('--'), {
      currency: undefined,
      main: '--',
      unit: undefined,
    })
    assert.deepEqual(splitStatValueText('1,234'), {
      currency: undefined,
      main: '1,234',
      unit: undefined,
    })
  })

  test('renders demoted currency and unit parts via stat value utilities', () => {
    const affix = readUtility('stat-value-affix')
    const unit = readUtility('stat-value-unit')

    assert.match(statValueSource, /className='stat-value-affix'/)
    assert.match(statValueSource, /className='stat-value-unit'/)
    assert.match(affix, /font-size:\s*0\.62em;/)
    assert.match(affix, /opacity:\s*0\.66;/)
    assert.match(unit, /font-size:\s*0\.58em;/)
    assert.match(unit, /opacity:\s*0\.62;/)
  })
})
