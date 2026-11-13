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

For commercial licensing, please contact support@quantumnous.com.
*/
import type { WorkbenchSession } from './builder-draft'

/**
 * Where an in-progress look lives: this module, for as long as the page runs.
 *
 * Remounting the route (leaving the workbench and coming back) keeps the panel,
 * and a reload starts over with one random look and no locks. That is the
 * point: a locked category that survives a refresh is a dead end the dice can
 * never escape, and the browser's own refresh is the reset a visitor already
 * knows. Nothing here may write storage.
 */
let session: WorkbenchSession | null = null

export function readSession(): WorkbenchSession | null {
  return session
}

export function writeSession(next: WorkbenchSession): void {
  session = next
}

export function clearSession(): void {
  session = null
}

/**
 * The previous release kept this session in sessionStorage, so a tab that was
 * already pinned stays pinned after the upgrade. Drop the stale record once at
 * startup: the in-memory session that follows starts clean.
 */
const LEGACY_DRAFT_KEY = 'influencer:character-draft:v1'

export function dropLegacyDraft(): void {
  try {
    sessionStorage.removeItem(LEGACY_DRAFT_KEY)
  } catch {
    // A disabled store simply has nothing to clean.
  }
}
