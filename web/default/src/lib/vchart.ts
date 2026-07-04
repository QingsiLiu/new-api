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
const VCHART_DATA_SCHEME = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const

export const VCHART_OPTION = {
  // 与老前端保持一致（浏览器环境渲染优化）
  mode: 'desktop-browser',
  theme: {
    background: 'transparent',
    fontFamily: 'var(--font-body)',
    colorScheme: {
      default: {
        dataScheme: VCHART_DATA_SCHEME,
        palette: {
          borderColor: 'var(--border)',
          axisLabelFontColor: 'var(--muted-foreground)',
          axisGridColor: 'var(--border)',
          axisDomainColor: 'var(--border)',
          popupBackgroundColor: 'var(--popover)',
          primaryFontColor: 'var(--popover-foreground)',
          secondaryFontColor: 'var(--muted-foreground)',
          shadowColor: 'rgba(0, 0, 0, 0.16)',
        },
      },
    },
    component: {
      axisX: {
        grid: { style: { stroke: 'var(--border)', lineWidth: 1 } },
        domainLine: { style: { stroke: 'var(--border)', lineWidth: 1 } },
        label: { style: { fill: 'var(--muted-foreground)', fontSize: 12 } },
      },
      axisY: {
        grid: { style: { stroke: 'var(--border)', lineWidth: 1 } },
        domainLine: { style: { stroke: 'var(--border)', lineWidth: 1 } },
        label: { style: { fill: 'var(--muted-foreground)', fontSize: 12 } },
      },
      tooltip: {
        panel: {
          backgroundColor: 'var(--popover)',
          border: { color: 'var(--border)', width: 1, radius: 8 },
          shadow: { x: 0, y: 8, blur: 18, spread: 0, color: 'rgba(0, 0, 0, 0.16)' },
        },
        keyLabel: { fontColor: 'var(--muted-foreground)', fontSize: 12 },
        valueLabel: { fontColor: 'var(--popover-foreground)', fontSize: 12 },
        titleLabel: { fontColor: 'var(--popover-foreground)', fontSize: 12 },
      },
    },
  },
} as const
