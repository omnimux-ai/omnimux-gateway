import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PRODUCT_DESCRIPTION } from '@/features/site-metadata/content'

import { About } from '../index'

const aboutQuery = vi.hoisted(() => ({ data: { data: '' }, isLoading: false }))
vi.mock('@tanstack/react-query', () => ({ useQuery: () => aboutQuery }))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({ systemName: 'OmniMux' }),
}))
vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <main>{props.children}</main>
  ),
}))
vi.mock('@/components/rich-content', () => ({
  RichContent: (props: { content: string }) => <div>{props.content}</div>,
}))

describe('about page content', () => {
  beforeEach(() => {
    aboutQuery.data.data = ''
    aboutQuery.isLoading = false
  })

  it('shows the product introduction, tutorial and verified links when configured content is empty', () => {
    render(<About />)
    expect(
      screen.getByRole('heading', { level: 1, name: 'About OmniMux' })
    ).toBeInTheDocument()
    expect(screen.getByText(PRODUCT_DESCRIPTION)).toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(6)
    expect(
      screen.getByRole('link', { name: 'Website · omnimux.ai' })
    ).toHaveAttribute('href', 'https://omnimux.ai/')
    expect(
      screen.getByRole('link', { name: 'Documentation · docs.omnimux.ai' })
    ).toHaveAttribute('href', 'https://docs.omnimux.ai')
    expect(
      screen.getByRole('link', { name: 'X · @OmniMuxai' })
    ).toHaveAttribute('href', 'https://x.com/OmniMuxai')
    expect(
      screen.getByRole('link', { name: 'QuantumNous' })
    ).toBeInTheDocument()
    expect(screen.queryByText('No About Content Set')).not.toBeInTheDocument()
  })

  it('keeps administrator-authored content when it exists', () => {
    aboutQuery.data.data = 'Our configured introduction.'
    render(<About />)
    expect(screen.getByText('Our configured introduction.')).toBeInTheDocument()
    expect(screen.queryByText(PRODUCT_DESCRIPTION)).not.toBeInTheDocument()
  })
})
