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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { Skeleton } from '@/components/ui/skeleton'
import { DOCS_URL, X_URL } from '@/features/omnimux-landing/lib/landing-urls'
import {
  ABOUT_PRICING,
  ABOUT_RIGHTS,
  ABOUT_STEPS,
  ABOUT_TITLE,
  ABOUT_TUTORIAL_TITLE,
  PRODUCT_DESCRIPTION,
} from '@/features/site-metadata/content'
import { useSystemConfig } from '@/hooks/use-system-config'
import { isHttpUrl, isLikelyHtml } from '@/lib/content-format'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getAboutContent } from './api'

function DefaultAboutContent() {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const currentYear = new Date().getFullYear()

  return (
    <article className='mx-auto max-w-3xl space-y-10 px-4 py-12'>
      <header className='space-y-4'>
        <p className='text-muted-foreground text-sm'>OmniMux · omnimux.ai</p>
        <h1 className='text-2xl font-semibold tracking-tight sm:text-[32px]'>
          {t(ABOUT_TITLE)}
        </h1>
        <p className='text-base leading-relaxed'>{t(PRODUCT_DESCRIPTION)}</p>
      </header>
      <section className='space-y-4'>
        <h2 className='text-xl font-semibold'>{t(ABOUT_TUTORIAL_TITLE)}</h2>
        <ol className='list-decimal space-y-3 pl-6 text-base leading-relaxed'>
          {ABOUT_STEPS.map((step) => (
            <li key={step}>{t(step)}</li>
          ))}
        </ol>
        <p className='text-muted-foreground text-base leading-relaxed'>
          {t(ABOUT_RIGHTS)}
        </p>
        <p className='text-muted-foreground text-base leading-relaxed'>
          {t(ABOUT_PRICING)}
        </p>
      </section>
      <section className='space-y-4'>
        <h2 className='text-xl font-semibold'>{t('Official links')}</h2>
        <ul className='space-y-3 text-base'>
          <li>
            <a
              className='text-primary hover:underline'
              href='https://omnimux.ai/'
            >
              {t('Website')} · omnimux.ai
            </a>
          </li>
          <li>
            <a className='text-primary hover:underline' href={DOCS_URL}>
              {t('Documentation')} · docs.omnimux.ai
            </a>
          </li>
          <li>
            <a className='text-primary hover:underline' href={X_URL}>
              X · @OmniMuxai
            </a>
          </li>
        </ul>
      </section>
      <div className='space-y-4 text-sm'>
        <p>
          {t('{{name}} Project Repository:', { name: systemName })}{' '}
          <a
            href='https://github.com/laozhong86/omnimux-cli'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('https://github.com/laozhong86/omnimux-cli')}
          </a>
        </p>
        <p className='text-muted-foreground'>
          <a
            href='https://github.com/laozhong86/omnimux-cli'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('OmniMux')}
          </a>{' '}
          © {currentYear} · {t('| Based on')}{' '}
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('New API')}
          </a>{' '}
          (
          <a
            href='https://github.com/QuantumNous'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('QuantumNous')}
          </a>
          ) {t('| Based on')}{' '}
          <a
            href='https://github.com/songquanpeng/one-api'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('One API')}
          </a>{' '}
          © 2023{' '}
          <a
            href='https://github.com/songquanpeng'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('JustSong')}
          </a>
        </p>
        <p className='text-muted-foreground'>
          {t('This project must be used in compliance with the')}{' '}
          <a
            href='https://github.com/QuantumNous/new-api/blob/main/LICENSE'
            target='_blank'
            rel='noopener noreferrer'
            className='text-primary hover:underline'
          >
            {t('AGPL v3.0 License')}
          </a>
          .
        </p>
      </div>
    </article>
  )
}

export function About() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['about-content'],
    queryFn: async () => requireServerSuccess(await getAboutContent()),
  })

  const rawContent = data?.data?.trim() ?? ''
  const hasContent = rawContent.length > 0
  const isUrl = hasContent && isHttpUrl(rawContent)
  const contentIsHtml = hasContent && isLikelyHtml(rawContent)

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto flex max-w-4xl flex-col gap-4 py-12'>
          <Skeleton className='h-8 w-[45%]' />
          <Skeleton className='h-4 w-full' />
          <Skeleton className='h-4 w-[90%]' />
          <Skeleton className='h-4 w-[80%]' />
        </div>
      </PublicLayout>
    )
  }

  if (!hasContent) {
    return (
      <PublicLayout>
        <DefaultAboutContent />
      </PublicLayout>
    )
  }

  if (isUrl) {
    return (
      <PublicLayout showMainContainer={false}>
        <iframe
          src={rawContent}
          className='h-[calc(100vh-3.5rem)] w-full border-0'
          title={t('About')}
          sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts'
        />
      </PublicLayout>
    )
  }

  if (contentIsHtml) {
    return (
      <PublicLayout showMainContainer={false}>
        <RichContent
          mode='html'
          htmlVariant='isolated'
          content={rawContent}
          className='prose-neutral dark:prose-invert max-w-none'
        />
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <div className='mx-auto max-w-6xl px-4 py-8'>
        <RichContent
          mode='markdown'
          content={rawContent}
          className='prose-neutral dark:prose-invert max-w-none'
        />
      </div>
    </PublicLayout>
  )
}
