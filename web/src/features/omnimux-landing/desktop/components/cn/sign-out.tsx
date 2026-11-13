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
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

import { logout } from '@/features/auth/api'
import { clearAuthenticatedClientState } from '@/lib/auth-session'
import { handleServerError } from '@/lib/handle-server-error'

import { useCopy } from '../../i18n'
import { Icon } from './icons'

/**
 * 落地页深色版退出确认。行为与全局 SignOutDialog 一致（logout →
 * 清理客户端会话 → 提示），但保留在当前落地页：登出后导航回到未登录态。
 */
export function CnSignOutDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const queryClient = useQueryClient()
  const [busy, setBusy] = useState(false)
  const copy = useCopy().signOut

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (props.open && !dialog.open) dialog.showModal()
    if (!props.open && dialog.open) dialog.close()
  }, [props.open])

  const close = () => props.onOpenChange(false)

  const handleSignOut = async () => {
    setBusy(true)
    try {
      const response = await logout()
      if (!response.success) {
        handleServerError(response, copy.failed)
        return
      }
      clearAuthenticatedClientState(queryClient)
      toast.success(copy.done)
      close()
    } catch (error: unknown) {
      handleServerError(error, copy.failed)
    } finally {
      setBusy(false)
    }
  }

  return (
    <dialog
      ref={dialogRef}
      className='dl cn-signout'
      aria-labelledby='cn-signout-title'
      onClose={close}
      onClick={(event) => {
        if (event.target === dialogRef.current) close()
      }}
    >
      <div className='dl-top'>
        <div>
          <h2 id='cn-signout-title'>{copy.title}</h2>
          <p>{copy.body}</p>
        </div>
        <button
          className='dl-x'
          type='button'
          aria-label={copy.close}
          onClick={close}
        >
          <Icon name='x' />
        </button>
      </div>
      <div className='cn-signout-actions'>
        <button className='btn btn-ghost' type='button' onClick={close}>
          {copy.cancel}
        </button>
        <button
          className='btn btn-primary'
          type='button'
          disabled={busy}
          onClick={() => void handleSignOut()}
        >
          {busy ? copy.busy : copy.confirm}
        </button>
      </div>
    </dialog>
  )
}
