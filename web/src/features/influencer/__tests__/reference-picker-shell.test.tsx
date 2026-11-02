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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { Dialog } from '@/components/dialog'

import { ReferencePickerShell } from '../components/reference-picker-shell'

describe('reference picker shell', () => {
  it.each(['Select reference images', 'Select reference videos'])(
    'on desktop clicking outside %s closes even while preparing',
    async (title) => {
      const onClose = vi.fn()
      render(
        <>
          <div>Workspace blank area</div>
          <ReferencePickerShell open title={title} busy onClose={onClose}>
            <span>Candidate grid</span>
          </ReferencePickerShell>
        </>
      )
      await userEvent.setup().click(screen.getByText('Workspace blank area'))
      expect(onClose).toHaveBeenCalledOnce()
    }
  )

  it('on desktop clicking content or panel whitespace keeps the picker open', async () => {
    const onClose = vi.fn()
    render(
      <ReferencePickerShell
        open
        title='Select reference images'
        onClose={onClose}
      >
        <span>Candidate grid</span>
      </ReferencePickerShell>
    )
    const user = userEvent.setup()
    await user.click(screen.getByText('Candidate grid'))
    await user.click(screen.getByRole('complementary'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('while a separate preview dialog is open outside interaction keeps the picker open', async () => {
    const onClose = vi.fn()
    render(
      <>
        <ReferencePickerShell
          open
          title='Select reference images'
          onClose={onClose}
        >
          <span>Candidate grid</span>
        </ReferencePickerShell>
        <Dialog open title='Reference preview'>
          Preview content
        </Dialog>
      </>
    )
    await userEvent.setup().click(screen.getByText('Preview content'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('after closing or unmounting outside clicks no longer invoke onClose', async () => {
    const onClose = vi.fn()
    const view = render(
      <ReferencePickerShell
        open
        title='Select reference images'
        onClose={onClose}
      >
        <span>Candidate grid</span>
      </ReferencePickerShell>
    )
    view.rerender(
      <ReferencePickerShell
        open={false}
        title='Select reference images'
        onClose={onClose}
      >
        <span>Candidate grid</span>
      </ReferencePickerShell>
    )
    const user = userEvent.setup()
    await user.click(document.body)
    view.unmount()
    await user.click(document.body)
    expect(onClose).not.toHaveBeenCalled()
  })

  it('on mobile uses the shared dialog and Escape closes even while preparing', async () => {
    const originalWidth = window.innerWidth
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 390,
    })
    const onClose = vi.fn()
    try {
      render(
        <ReferencePickerShell
          open
          title='Select reference videos'
          description='Choose a video'
          busy
          onClose={onClose}
        >
          <span>History</span>
        </ReferencePickerShell>
      )
      expect(
        screen.getByRole('dialog', { name: 'Select reference videos' })
      ).toBeInTheDocument()
      expect(screen.getByText('Choose a video')).toBeInTheDocument()
      await userEvent.setup().keyboard('{Escape}')
      expect(onClose).toHaveBeenCalledOnce()
    } finally {
      Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        value: originalWidth,
      })
    }
  })

  it('when closed hides its children without invoking selection or close callbacks', () => {
    const onClose = vi.fn()
    render(
      <ReferencePickerShell
        open={false}
        title='Select reference videos'
        onClose={onClose}
      >
        <span>History</span>
      </ReferencePickerShell>
    )
    expect(screen.queryByText('History')).not.toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('while busy shows progress but keeps the desktop close action available without confirmation', async () => {
    const onClose = vi.fn()
    render(
      <ReferencePickerShell
        open
        title='Select reference videos'
        busy
        onClose={onClose}
      >
        <span>Candidate grid</span>
      </ReferencePickerShell>
    )
    expect(
      screen.getByRole('complementary', { name: 'Select reference videos' })
    ).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('Preparing reference')
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Close panel' }))
    expect(onClose).toHaveBeenCalledOnce()
    expect(
      screen.queryByRole('button', { name: /confirm/i })
    ).not.toBeInTheDocument()
  })
})
