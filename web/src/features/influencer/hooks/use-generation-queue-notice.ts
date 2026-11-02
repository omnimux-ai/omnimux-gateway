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
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { useAuthStore } from '@/stores/auth-store'

import { isTerminalStatus } from '../lib/task-status'
import type { TaskRecord } from '../types'

/** Only server-confirmed waiting submissions can trigger a capacity notice. */
export function useGenerationQueueNotice(
  submitted: TaskRecord[],
  history: TaskRecord[]
): void {
  const { t } = useTranslation()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const notified = useRef(new Set<string>())

  useEffect(() => {
    notified.current.clear()
  }, [userId])

  useEffect(() => {
    const records = new Map<string, TaskRecord>()
    for (const record of [...submitted, ...history]) {
      if (record.task_id) records.set(record.task_id, record)
    }
    const activeCount = [...records.values()].filter(
      (record) =>
        record.id > 0 &&
        !isTerminalStatus(record.status) &&
        (record.status ?? '').toUpperCase() !== 'WAITING'
    ).length
    if (activeCount < 5) return

    for (const record of submitted) {
      const taskId = record.task_id
      if (
        !taskId ||
        notified.current.has(taskId) ||
        records.get(taskId)?.status?.toUpperCase() !== 'WAITING'
      ) {
        continue
      }
      notified.current.add(taskId)
      toast.info(
        t(
          'Images and videos share 5 generation slots. Additional tasks wait in queue.'
        )
      )
    }
  }, [submitted, history, t])
}
