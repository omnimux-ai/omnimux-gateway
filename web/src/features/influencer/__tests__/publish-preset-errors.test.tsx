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

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { getServerErrorMessage } from '@/lib/server-error-message'

import { publishInfluencerPreset } from '../api'
import { PublishPresetDialog } from '../components/publish-preset-dialog'

const key = 'Cannot publish more than {{max}} presets'
const payload = {
  success: false,
  message: 'Cannot publish more than 3 presets',
  message_key: key,
  message_params: { max: 3 },
}
const input = {
  taskId: 'sheet_1',
  name: 'Character',
  preview: new Blob(['image']),
}

afterEach(async () => {
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
})

it('rejects failed publish responses while preserving source keys and parameters', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({ data: payload })
  const accepted = vi.fn()
  let failure: unknown
  await publishInfluencerPreset(input)
    .then(accepted)
    .catch((error: unknown) => {
      failure = error
    })

  expect(accepted).not.toHaveBeenCalled()
  expect(failure).toBeInstanceOf(Error)
  expect((failure as Error).cause).toBe(payload)
  i18next.addResourceBundle('vi', 'translation', {
    [key]: 'Không thể xuất bản quá {{max}} mẫu',
  })
  await i18next.changeLanguage('vi')
  expect(getServerErrorMessage(failure)).toBe('Không thể xuất bản quá 3 mẫu')
})

it('reports a failed publish through the top toast without inline copy', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({ data: payload })
  const errorToast = vi.spyOn(toast, 'error')
  const onSubmit = vi.fn(async () => {
    await publishInfluencerPreset(input)
  })
  const onOpenChange = vi.fn()
  render(
    <PublishPresetDialog
      open
      task={null}
      child={null}
      submitting={false}
      onSubmit={onSubmit}
      onOpenChange={onOpenChange}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Publish' }))

  await waitFor(() =>
    expect(errorToast).toHaveBeenCalledWith('Cannot publish more than 3 presets')
  )
  expect(screen.queryByRole('alert')).toBeNull()
  expect(onSubmit).toHaveBeenCalledOnce()
  expect(post).toHaveBeenCalledOnce()
  expect(onOpenChange).not.toHaveBeenCalled()
})

it('keeps unknown publish failure details verbatim in the toast', async () => {
  const detail = 'Supplier refused {{reason}} and $t(Publish)'
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: false, msg: detail },
  })
  const errorToast = vi.spyOn(toast, 'error')
  render(
    <PublishPresetDialog
      open
      task={null}
      child={null}
      submitting={false}
      onSubmit={async () => {
        await publishInfluencerPreset(input)
      }}
      onOpenChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Publish' }))

  await waitFor(() => expect(errorToast).toHaveBeenCalledWith(detail))
  expect(screen.queryByRole('alert')).toBeNull()
})
