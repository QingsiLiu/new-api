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
export type SemanticColor =
  | 'blue'
  | 'green'
  | 'cyan'
  | 'purple'
  | 'pink'
  | 'red'
  | 'orange'
  | 'amber'
  | 'yellow'
  | 'lime'
  | 'light-green'
  | 'teal'
  | 'light-blue'
  | 'indigo'
  | 'violet'
  | 'grey'
  | 'slate'

export const colorToBgClass: Record<SemanticColor, string> = {
  blue: 'bg-calm-blue-bg',
  green: 'bg-calm-green-bg',
  cyan: 'bg-calm-teal-bg',
  purple: 'bg-calm-violet-bg',
  pink: 'bg-calm-rose-bg',
  red: 'bg-calm-rose-bg',
  orange: 'bg-calm-amber-bg',
  amber: 'bg-calm-amber-bg',
  yellow: 'bg-calm-amber-bg',
  lime: 'bg-calm-green-bg',
  'light-green': 'bg-calm-green-bg',
  teal: 'bg-calm-teal-bg',
  'light-blue': 'bg-calm-blue-bg',
  indigo: 'bg-calm-violet-bg',
  violet: 'bg-calm-violet-bg',
  grey: 'bg-calm-gray-bg',
  slate: 'bg-calm-gray-bg',
}

export const avatarColorMap: Record<SemanticColor, string> = {
  blue: 'bg-calm-blue-bg text-calm-blue-fg',
  green: 'bg-calm-green-bg text-calm-green-fg',
  cyan: 'bg-calm-teal-bg text-calm-teal-fg',
  purple: 'bg-calm-violet-bg text-calm-violet-fg',
  pink: 'bg-calm-rose-bg text-calm-rose-fg',
  red: 'bg-calm-rose-bg text-calm-rose-fg',
  orange: 'bg-calm-amber-bg text-calm-amber-fg',
  amber: 'bg-calm-amber-bg text-calm-amber-fg',
  yellow: 'bg-calm-amber-bg text-calm-amber-fg',
  lime: 'bg-calm-green-bg text-calm-green-fg',
  'light-green': 'bg-calm-green-bg text-calm-green-fg',
  teal: 'bg-calm-teal-bg text-calm-teal-fg',
  'light-blue': 'bg-calm-blue-bg text-calm-blue-fg',
  indigo: 'bg-calm-violet-bg text-calm-violet-fg',
  violet: 'bg-calm-violet-bg text-calm-violet-fg',
  grey: 'bg-calm-gray-bg text-calm-gray-fg',
  slate: 'bg-calm-gray-bg text-calm-gray-fg',
}

export function getAvatarColorClass(name: string): string {
  return avatarColorMap[stringToColor(name)]
}

export function getBgColorClass(color?: string): string {
  if (!color) return colorToBgClass.blue
  return (
    (colorToBgClass as Record<string, string>)[color] || colorToBgClass.blue
  )
}

/**
 * Chart color palette - reads from semantic CSS tokens so presets stay in sync.
 */
export const CHART_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const

/**
 * Get a chart color by index (cycles through the palette)
 */
export function getChartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

/**
 * Announcement status types
 */
export type AnnouncementType =
  | 'default'
  | 'ongoing'
  | 'success'
  | 'warning'
  | 'error'

/**
 * Announcement status color mapping
 */
export const ANNOUNCEMENT_TYPE_COLORS: Record<AnnouncementType, string> = {
  default: 'bg-neutral',
  ongoing: 'bg-info',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-destructive',
}

/**
 * Get announcement status color class
 */
export function getAnnouncementColorClass(type?: string): string {
  const validType = (type || 'default') as AnnouncementType
  return ANNOUNCEMENT_TYPE_COLORS[validType] || ANNOUNCEMENT_TYPE_COLORS.default
}

/**
 * Semantic colors for tags and badges
 */
const TAG_COLORS = [
  'pink',
  'indigo',
  'teal',
  'amber',
  'blue',
  'cyan',
  'green',
  'light-blue',
  'lime',
  'orange',
  'purple',
  'red',
  'violet',
  'yellow',
] as const

/**
 * Convert string to a stable semantic color
 * Used for model tags, group badges, user avatars, etc.
 * Same string always returns the same color
 *
 * @param str - Input string (model name, group name, username, etc.)
 * @returns Semantic color name from TAG_COLORS
 *
 * @example
 * stringToColor('gpt-4') // 'blue'
 * stringToColor('claude-3') // 'purple'
 * stringToColor('default') // 'green'
 */
export function stringToColor(str: string): SemanticColor {
  let sum = 0
  for (let i = 0; i < str.length; i++) {
    sum += str.charCodeAt(i)
  }
  const index = sum % TAG_COLORS.length
  return TAG_COLORS[index]
}
