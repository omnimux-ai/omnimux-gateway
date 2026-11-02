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
import { api } from '@/lib/api'

import type { ToolKind } from '../content'

const BASE = '/api/tools/tiktok'

export type ToolQuota = { used: number; limit: number }

export type ToolConfig = {
  enabled: boolean
  turnstile_site_key: string
  pass_valid: boolean
  quota: ToolQuota
  logged_in: boolean
}

export type ToolImage = {
  index: number
  width: number
  height: number
  preview_url: string
  download_url: string
}

export type ToolResult = {
  kind: ToolKind
  id: string
  author: { unique_id: string; nickname: string }
  description: string
  /** Unix seconds. */
  created_at: number
  cover_url: string
  video?: {
    duration_ms: number
    width: number
    height: number
    /** Inline, range-capable stream for the player. */
    preview_url: string
    /** Attachment download of the same file. */
    download_url: string
  }
  images?: ToolImage[]
  zip_url?: string
  quota: ToolQuota
}

type Envelope<T> = {
  success: boolean
  message?: string
  code?: string
  data?: T
}

/** A failed tool request, reduced to what the page branches on. */
export class ToolRequestError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, status: number) {
    super(code || `HTTP ${status}`)
    this.name = 'ToolRequestError'
    this.code = code
    this.status = status
  }
}

/**
 * Unwrap the `{ success, code, data }` envelope from either a resolved
 * response or an Axios rejection. Messages stay local: the caller maps
 * `code` to page copy and shows one toast.
 */
async function callTool<T>(
  request: Promise<{ data: Envelope<T> }>
): Promise<T> {
  let body: Envelope<T> | undefined
  let status = 200
  try {
    const response = await request
    body = response.data
  } catch (error) {
    const response = (
      error as {
        response?: { status?: number; data?: Envelope<T> }
      }
    ).response
    status = response?.status ?? 0
    body = response?.data
  }
  if (body?.success && body.data !== undefined) return body.data
  throw new ToolRequestError(body?.code ?? '', status)
}

export function getToolConfig(): Promise<ToolConfig> {
  return callTool(
    api.get<Envelope<ToolConfig>>(`${BASE}/config`, {
      skipErrorHandler: true,
      disableDuplicate: true,
    })
  )
}

export function verifyToolTurnstile(
  token: string
): Promise<{ pass_valid: boolean }> {
  return callTool(
    api.post<Envelope<{ pass_valid: boolean }>>(
      `${BASE}/verify`,
      { token },
      { skipErrorHandler: true }
    )
  )
}

export function resolveToolLink(
  url: string,
  kind: ToolKind
): Promise<ToolResult> {
  return callTool(
    api.post<Envelope<ToolResult>>(
      `${BASE}/resolve`,
      { url, kind },
      { skipErrorHandler: true }
    )
  )
}
