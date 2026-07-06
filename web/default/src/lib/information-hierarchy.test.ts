import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const channelsColumns = readFileSync(
  new URL(
    '../features/channels/components/channels-columns.tsx',
    import.meta.url
  ),
  'utf8'
)
const apiKeysTable = readFileSync(
  new URL('../features/keys/components/api-keys-table.tsx', import.meta.url),
  'utf8'
)
const usersColumns = readFileSync(
  new URL('../features/users/components/users-columns.tsx', import.meta.url),
  'utf8'
)

describe('table information hierarchy', () => {
  test('keeps primary names medium weight with foreground color', () => {
    assert.match(channelsColumns, /className='text-foreground font-medium'>Tag/)
    assert.match(channelsColumns, /className='text-foreground font-medium'/)
    assert.match(
      apiKeysTable,
      /className='[^']*\btext-sm\b[^']*\bfont-medium\b[^']*\btext-foreground\b|className='[^']*\btext-foreground\b[^']*\btext-sm\b[^']*\bfont-medium\b/
    )
    assert.match(
      usersColumns,
      /className='text-foreground max-w-\[140px\] font-medium'/
    )
  })

  test('renders user remarks as muted secondary text instead of status color', () => {
    assert.doesNotMatch(
      usersColumns,
      /render={<StatusBadge variant='success' copyable={false} \/>}/
    )
    assert.match(
      usersColumns,
      /className='text-muted-foreground max-w-\[80px\] truncate text-xs'/
    )
  })
})
