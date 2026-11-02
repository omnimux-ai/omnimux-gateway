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

export interface MentionItem {
  id: string
  type: 'video' | 'image'
  index: number
  name: string
  label: string
  thumbnail?: string
  url?: string
}

export interface MentionToken {
  type: 'text' | 'mention'
  content: string
  item?: MentionItem
  missing?: boolean
}

/** Reference names use horizontal spaces only; prose, email and HTML stay text. */
export const MENTION_PATTERN =
  /(?:(?<![A-Za-z0-9_@])@(Video|Image)[ \t]*(\d+)(?![A-Za-z0-9_])|<(Picture|Image|Video)[ \t]*(\d+)>)/gi

function readMention(match: RegExpExecArray): MentionItem | null {
  const index = Number(match[2] || match[4])
  if (!Number.isSafeInteger(index) || index < 1 || index > 30) return null
  const type =
    (match[1] || match[3]).toLowerCase() === 'video' ? 'video' : 'image'
  const name = `${type === 'video' ? 'Video' : 'Image'} ${index}`
  return { id: `${type}-${index}`, type, index, name, label: `@${name}` }
}

export function normalizeMentionText(text: string): string {
  return tokenizePrompt(text)
    .map((token) => token.content)
    .join('')
}

/** Passing assets marks unresolved references missing rather than binding another asset. */
export function tokenizePrompt(
  text: string,
  availableMentions?: MentionItem[]
): MentionToken[] {
  const tokens: MentionToken[] = []
  let lastIndex = 0
  for (const match of text.matchAll(MENTION_PATTERN)) {
    const reference = readMention(match)
    if (!reference) continue
    if (match.index > lastIndex) {
      tokens.push({ type: 'text', content: text.slice(lastIndex, match.index) })
    }
    const item = availableMentions?.find(
      (candidate) =>
        candidate.type === reference.type && candidate.index === reference.index
    )
    const token: MentionToken = {
      type: 'mention',
      content: reference.label,
      item: item ?? reference,
    }
    if (availableMentions !== undefined && !item) token.missing = true
    tokens.push(token)
    lastIndex = match.index + match[0].length
  }
  if (lastIndex < text.length) {
    tokens.push({ type: 'text', content: text.slice(lastIndex) })
  }
  return tokens
}

export function getAvailableMentions(params: {
  hasVideo: boolean
  videoThumbnail?: string
  videoUrl?: string
  characterImages: string[]
}): MentionItem[] {
  const items: MentionItem[] = []
  if (params.hasVideo) {
    items.push({
      id: `video:${params.videoUrl ?? params.videoThumbnail ?? '1'}`,
      type: 'video',
      index: 1,
      name: 'Video 1',
      label: '@Video 1',
      thumbnail: params.videoThumbnail,
      url: params.videoUrl,
    })
  }
  params.characterImages.slice(0, 30).forEach((url, i) => {
    const index = i + 1
    items.push({
      id: `image:${url}`,
      type: 'image',
      index,
      name: `Image ${index}`,
      label: `@Image ${index}`,
      thumbnail: url,
      url,
    })
  })
  return items
}
