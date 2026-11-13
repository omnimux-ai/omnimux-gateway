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

/**
 * fork: tool-to-workbench — the TikTok tool page exchanges a parsed work for a
 * short-lived opaque token; the signed-in workbench redeems it for an upload
 * the user owns. Only the work id goes up and only the token comes back, so
 * neither the visitor's link nor a media address enters the address bar or an
 * analytics call.
 */
export const TOOL_HANDOFF_ISSUE_PATH = '/api/tools/tiktok/handoff'
export const TOOL_HANDOFF_REDEEM_PATH = '/api/tools/tiktok/handoff/redeem'

export type ToolHandoffIssueRequest = { id: string }
export type ToolHandoffIssueResponse = { token: string; expires_in: number }
export type ToolHandoffRedeemRequest = { token: string }
export type ToolHandoffRedeemResponse = {
  url: string
  preview_url: string
  name: string
}

type Envelope<T> = { success: boolean; code?: string; data?: T }

/** A failed handoff call, reduced to its server code. */
export class ToolHandoffError extends Error {
  readonly code: string

  constructor(code: string) {
    super(code || 'handoff_unavailable')
    this.name = 'ToolHandoffError'
    this.code = code
  }
}

async function callHandoff<T>(path: string, body: object): Promise<T> {
  let envelope: Envelope<T> | undefined
  try {
    const response = await api.post<Envelope<T>>(path, body, {
      skipErrorHandler: true,
    })
    envelope = response.data
  } catch (error) {
    envelope = (error as { response?: { data?: Envelope<T> } }).response?.data
  }
  if (envelope?.success && envelope.data !== undefined) return envelope.data
  throw new ToolHandoffError(envelope?.code ?? '')
}

export function issueToolHandoff(
  request: ToolHandoffIssueRequest
): Promise<ToolHandoffIssueResponse> {
  return callHandoff(TOOL_HANDOFF_ISSUE_PATH, { id: request.id })
}

export function redeemToolHandoff(
  request: ToolHandoffRedeemRequest
): Promise<ToolHandoffRedeemResponse> {
  return callHandoff(TOOL_HANDOFF_REDEEM_PATH, { token: request.token })
}
