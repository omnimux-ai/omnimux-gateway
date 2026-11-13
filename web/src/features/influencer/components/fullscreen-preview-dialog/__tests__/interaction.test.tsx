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

import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { FullscreenPreviewDialog } from '../../fullscreen-preview-dialog'

afterEach(cleanup)

describe('full-screen preview public contract', () => {
  it('keeps keyboard focus inside the dialog and dismisses on Escape', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(
      <FullscreenPreviewDialog
        open
        onOpenChange={onOpenChange}
        title='Preview'
        media={<img alt='Character' src='/character.png' />}
      />
    )
    const close = screen.getByRole('button', { name: 'Close' })
    close.focus()
    await user.tab()
    expect(screen.getByRole('dialog', { name: 'Preview' })).toContainElement(
      document.activeElement as HTMLElement
    )
    await user.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
  it('keyboard opening a video-first preview moves focus inside and returns it on close', async () => {
    function PreviewFromCard() {
      const [open, setOpen] = useState(false)
      return (
        <>
          <button type='button' onClick={() => setOpen(true)}>
            Open video
          </button>
          <FullscreenPreviewDialog
            open={open}
            mediaFirst
            onOpenChange={setOpen}
            title='Video preview'
            media={<video aria-label='Video' controls tabIndex={0} />}
            actions={<button type='button'>Apply</button>}
          />
        </>
      )
    }
    const user = userEvent.setup()
    render(<PreviewFromCard />)
    const trigger = screen.getByRole('button', { name: 'Open video' })
    trigger.focus()
    await user.keyboard('{Enter}')
    const dialog = screen.getByRole('dialog', { name: 'Video preview' })
    await waitFor(() =>
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    )
    await user.tab()
    await waitFor(() =>
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    )
    await user.tab({ shift: true })
    await waitFor(() =>
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    )
    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('does not expose closed preview content', () => {
    render(
      <FullscreenPreviewDialog
        open={false}
        onOpenChange={vi.fn()}
        title='Preview'
        media={<img alt='Character' src='/character.png' />}
      />
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it('names the dialog with its real title and closes through its shared control', async () => {
    const onOpenChange = vi.fn()
    render(
      <FullscreenPreviewDialog
        open
        onOpenChange={onOpenChange}
        title='Character preview'
        media={<img alt='Character' src='/character.png' />}
        actions={<button type='button'>Download</button>}
      >
        <p>Character direction</p>
      </FullscreenPreviewDialog>
    )
    expect(
      screen.getByRole('dialog', { name: 'Character preview' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Character preview' })
    ).toBeInTheDocument()
    expect(screen.getByText('Character direction')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
