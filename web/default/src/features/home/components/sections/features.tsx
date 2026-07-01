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
import {
  Zap,
  Shield,
  Globe,
  Code,
  Gauge,
  DollarSign,
  Users,
  HeartHandshake,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

interface FeaturesProps {
  className?: string
}

export function Features(_props: FeaturesProps) {
  const { t } = useTranslation()

  const features = [
    {
      id: 'fast',
      num: '01',
      title: t('Lightning Fast'),
      desc: t(
        'Optimized network architecture ensures millisecond response times'
      ),
      span: 'md:col-span-2',
      icon: <Zap className='size-4' />,
      visual: (
        <div className='mt-5 grid grid-cols-3 gap-2'>
          {['OpenAI', 'Claude', 'Gemini', 'DeepSeek', 'Qwen', 'Llama'].map(
            (name) => (
              <div
                key={name}
                className='border-border bg-muted/20 text-muted-foreground flex items-center justify-center rounded-md border px-3 py-2 font-mono text-[11px]'
              >
                {name}
              </div>
            )
          )}
        </div>
      ),
    },
    {
      id: 'secure',
      num: '02',
      title: t('Secure & Reliable'),
      desc: t(
        'Enterprise-grade security with comprehensive permission management'
      ),
      span: 'md:col-span-1',
      icon: <Shield className='size-4' />,
      visual: (
        <div className='mt-5 flex items-center gap-3'>
          <span className='border-border bg-muted/20 flex size-14 items-center justify-center rounded-lg border'>
            <Shield className='text-success size-6' strokeWidth={1.5} />
          </span>
          <span className='editorial-label text-success'>{t('Verified')}</span>
        </div>
      ),
    },
    {
      id: 'global',
      num: '03',
      title: t('Global Coverage'),
      desc: t('Multi-region deployment for stable global access'),
      span: 'md:col-span-1',
      icon: <Globe className='size-4' />,
      visual: (
        <div className='mt-5 space-y-2'>
          {[t('Load Balancing'), t('Rate Limiting'), t('Cost Tracking')].map(
            (step, i) => (
              <div key={step} className='flex items-center gap-2'>
                <div className='border-border bg-muted text-muted-foreground flex size-6 items-center justify-center rounded-md border font-mono text-[10px] font-medium'>
                  {i + 1}
                </div>
                <div className='bg-border h-px flex-1' />
                <span className='text-muted-foreground text-xs'>{step}</span>
              </div>
            )
          )}
        </div>
      ),
    },
    {
      id: 'developer',
      num: '04',
      title: t('Developer Friendly'),
      desc: t('Compatible API routes for common AI application workflows'),
      span: 'md:col-span-2',
      icon: <Code className='size-4' />,
      visual: (
        <div className='mt-5 flex items-center gap-3'>
          <div className='flex -space-x-2'>
            {['API', 'SDK', 'CLI', 'Docs'].map((n) => (
              <div
                key={n}
                className='border-background bg-card text-muted-foreground flex size-8 items-center justify-center rounded-full border-2 font-mono text-[9px] font-medium'
              >
                {n}
              </div>
            ))}
          </div>
          <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
            <Code className='text-primary size-3.5' />
            {t('Multi-protocol Compatible')}
          </div>
        </div>
      ),
    },
  ]

  const additionalFeatures = [
    {
      icon: <Gauge className='size-5' strokeWidth={1.5} />,
      title: t('High Performance'),
      desc: t('Support for high concurrency with automatic load balancing'),
    },
    {
      icon: <DollarSign className='size-5' strokeWidth={1.5} />,
      title: t('Transparent Billing'),
      desc: t('Pay-as-you-go with real-time usage monitoring'),
    },
    {
      icon: <Users className='size-5' strokeWidth={1.5} />,
      title: t('Team Collaboration'),
      desc: t('Multi-user management with flexible permission allocation'),
    },
    {
      icon: <HeartHandshake className='size-5' strokeWidth={1.5} />,
      title: t('Open Source'),
      desc: t('Community driven, self-hosted, and extensible'),
    },
  ]

  return (
    <section className='relative z-10 px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-16 max-w-lg'>
          <p className='editorial-label mb-3'>{t('Core Features')}</p>
          <h2 className='editorial-section-title text-3xl md:text-4xl'>
            {t('Built for developers,')}
            <br />
            {t('designed for scale')}
          </h2>
        </AnimateInView>

        <div className='border-border bg-border grid gap-px overflow-hidden rounded-xl border md:grid-cols-3'>
          {features.map((f, i) => (
            <AnimateInView
              key={f.id}
              delay={i * 80}
              animation='fade-up'
              className={`bg-background hover:bg-accent/30 p-7 transition-colors duration-200 md:p-8 ${f.span}`}
            >
              <div className='mb-4 flex items-center gap-3'>
                <span className='border-border bg-muted text-muted-foreground flex size-7 items-center justify-center rounded-md border font-mono text-[10px] font-medium tabular-nums'>
                  {f.num}
                </span>
                <span className='text-muted-foreground'>{f.icon}</span>
              </div>
              <h3 className='editorial-section-title text-xl'>{f.title}</h3>
              <p className='text-muted-foreground mt-3 text-sm leading-relaxed'>
                {f.desc}
              </p>
              {f.visual}
            </AnimateInView>
          ))}
        </div>

        <div className='mt-12 grid grid-cols-2 gap-x-8 gap-y-10 md:grid-cols-4 md:gap-x-12'>
          {additionalFeatures.map((f, i) => (
            <AnimateInView
              key={f.title}
              delay={i * 80}
              animation='fade-up'
              className='border-border flex flex-col border-t pt-5'
            >
              <div className='text-muted-foreground mb-4'>{f.icon}</div>
              <h3 className='text-sm font-semibold'>{f.title}</h3>
              <p className='text-muted-foreground mt-2 max-w-[200px] text-xs leading-relaxed'>
                {f.desc}
              </p>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
