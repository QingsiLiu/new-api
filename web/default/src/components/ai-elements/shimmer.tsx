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
'use client'

import { type ElementType, memo } from 'react'
import { motion } from 'motion/react'
import { cn } from '@/lib/utils'

export type TextShimmerProps = {
  children: string
  as?: ElementType
  className?: string
  duration?: number
  spread?: number
}

const MotionP = motion.p

const ShimmerComponent = ({
  children,
  className,
  duration = 2,
  spread: _spread = 2,
}: TextShimmerProps) => {
  return (
    <MotionP
      animate={{ opacity: [0.58, 1, 0.58] }}
      className={cn('text-muted-foreground relative inline-block', className)}
      initial={{ opacity: 0.58 }}
      transition={{
        repeat: Number.POSITIVE_INFINITY,
        duration,
        ease: 'easeInOut',
      }}
    >
      {children}
    </MotionP>
  )
}

export const Shimmer = memo(ShimmerComponent)
