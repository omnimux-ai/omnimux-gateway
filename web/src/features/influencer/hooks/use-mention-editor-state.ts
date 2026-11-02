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
import {
  type EditorSelection,
  EditorState,
  StateEffect,
  StateField,
  Transaction,
  type Extension,
} from '@codemirror/state'
import {
  Decoration,
  EditorView,
  WidgetType,
  keymap,
  type DecorationSet,
} from '@codemirror/view'

import { MENTION_PATTERN, type MentionItem } from '../lib/mention'

export interface BoundMention {
  from: number
  to: number
  item: MentionItem | undefined
  type: 'video' | 'image'
  index: number
}

export const mentionAssets = StateEffect.define<readonly MentionItem[]>()
const restoreMentions = StateEffect.define<readonly BoundMention[]>()
export const mentionExternalReset = StateEffect.define<boolean>()
export const mentionCompositionEnd = StateEffect.define<boolean>()
const historyReplay = StateEffect.define<boolean>()

export function parseBoundMentions(
  text: string,
  assets: readonly MentionItem[]
): BoundMention[] {
  const result: BoundMention[] = []
  const pattern = new RegExp(MENTION_PATTERN.source, MENTION_PATTERN.flags)
  for (const match of text.matchAll(pattern)) {
    const type =
      (match[1] || match[3]).toLowerCase() === 'video' ? 'video' : 'image'
    const index = Number(match[2] || match[4])
    if (!Number.isSafeInteger(index) || index < 1 || index > 30) continue
    result.push({
      from: match.index,
      to: match.index + match[0].length,
      type,
      index,
      item: assets.find(
        (asset) => asset.type === type && asset.index === index
      ),
    })
  }
  return result
}

export function resolveBoundMention(
  mention: BoundMention,
  assets: readonly MentionItem[]
): MentionItem | undefined {
  // An unresolved reference never binds to a subsequently uploaded asset.
  if (!mention.item) return undefined
  return assets.find((asset) => asset.id === mention.item?.id)
}

export function serializeBoundPrompt(
  text: string,
  mentions: readonly BoundMention[],
  assets: readonly MentionItem[]
): string {
  let cursor = 0
  let result = ''
  for (const mention of mentions) {
    result += text.slice(cursor, mention.from)
    const item = resolveBoundMention(mention, assets)
    if (item) result += item.label
    // No @ or bracket token may accidentally resolve against another asset.
    else {
      result += `[Missing reference: ${mention.type === 'video' ? 'Video' : 'Image'} ${mention.index}]`
    }
    cursor = mention.to
  }
  return result + text.slice(cursor)
}

function mapMentions(
  mentions: readonly BoundMention[],
  transaction: Transaction
): BoundMention[] {
  return mentions.flatMap((mention) => {
    let changed = false
    transaction.changes.iterChangedRanges((from, to) => {
      if (from < mention.to && to > mention.from) changed = true
      if (from === to && from > mention.from && from < mention.to) {
        changed = true
      }
    })
    if (changed) return []
    return [
      {
        ...mention,
        from: transaction.changes.mapPos(mention.from, 1),
        to: transaction.changes.mapPos(mention.to, -1),
      },
    ]
  })
}

class MentionWidget extends WidgetType {
  constructor(
    readonly label: string,
    readonly missing: boolean,
    readonly missingLabel: string
  ) {
    super()
  }

  eq(other: MentionWidget): boolean {
    return (
      this.label === other.label &&
      this.missing === other.missing &&
      this.missingLabel === other.missingLabel
    )
  }

  toDOM(): HTMLElement {
    const dom = document.createElement('span')
    dom.className = 'mention-prompt-chip'
    dom.dataset.missing = String(this.missing)
    dom.textContent = this.missing
      ? `${this.label} · ${this.missingLabel}`
      : this.label
    dom.setAttribute('aria-label', dom.textContent)
    return dom
  }
}

interface Snapshot {
  doc: string
  selection: EditorSelection
  mentions: readonly BoundMention[]
}

/** CodeMirror is installed without @codemirror/commands; history stores document
 * and reference identities together, so undo cannot rebind a deleted asset. */
export function createMentionState(options: {
  initialAssets: readonly MentionItem[]
  missingLabel: () => string
}): {
  extensions: Extension[]
  mentions: StateField<readonly BoundMention[]>
  assets: StateField<readonly MentionItem[]>
} {
  const assets = StateField.define<readonly MentionItem[]>({
    create: () => options.initialAssets,
    update(value, transaction) {
      for (const effect of transaction.effects) {
        if (effect.is(mentionAssets)) return effect.value
      }
      return value
    },
  })
  const mentions = StateField.define<readonly BoundMention[]>({
    create: (state) =>
      parseBoundMentions(state.doc.toString(), state.field(assets)),
    update(value, transaction) {
      for (const effect of transaction.effects) {
        if (effect.is(restoreMentions)) return effect.value
      }
      const compositionEnded = transaction.effects.some((effect) =>
        effect.is(mentionCompositionEnd)
      )
      if (!transaction.docChanged && !compositionEnded) return value
      const currentAssets = transaction.state.field(assets)
      if (
        transaction.effects.some((effect) => effect.is(mentionExternalReset))
      ) {
        return parseBoundMentions(transaction.newDoc.toString(), currentAssets)
      }
      const retained = mapMentions(value, transaction)
      if (transaction.isUserEvent('input.type.compose')) return retained
      const text = transaction.newDoc.toString()
      const parsed = parseBoundMentions(text, currentAssets).filter(
        (candidate) => {
          // Keep an unfinished @Image1 editable so the next digit can make 10–30.
          return (
            !transaction.isUserEvent('input.type') ||
            candidate.to !== transaction.newSelection.main.head ||
            text[candidate.from] !== '@'
          )
        }
      )
      return [
        ...retained,
        ...parsed.filter(
          (candidate) =>
            !retained.some(
              (old) => candidate.from < old.to && candidate.to > old.from
            )
        ),
      ].sort((a, b) => a.from - b.from)
    },
  })
  const decorations = StateField.define<DecorationSet>({
    create: decorate,
    update: (_value, transaction) => decorate(transaction.state),
    provide: (field) => EditorView.decorations.from(field),
  })
  function decorate(state: EditorState): DecorationSet {
    const currentAssets = state.field(assets)
    return Decoration.set(
      state.field(mentions).map((mention) => {
        const item = resolveBoundMention(mention, currentAssets)
        const label =
          item?.name ??
          `${mention.type === 'video' ? 'Video' : 'Image'} ${mention.index}`
        return Decoration.replace({
          widget: new MentionWidget(`@${label}`, !item, options.missingLabel()),
        }).range(mention.from, mention.to)
      })
    )
  }
  const undo: Snapshot[] = []
  const redo: Snapshot[] = []
  function snapshot(state: EditorState): Snapshot {
    return {
      doc: state.doc.toString(),
      selection: state.selection,
      mentions: state.field(mentions),
    }
  }
  function replay(view: EditorView, from: Snapshot[], to: Snapshot[]): boolean {
    const previous = from.pop()
    if (!previous) return false
    to.push(snapshot(view.state))
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: previous.doc },
      selection: previous.selection,
      effects: [restoreMentions.of(previous.mentions), historyReplay.of(true)],
      annotations: Transaction.userEvent.of('history'),
      scrollIntoView: true,
    })
    return true
  }
  function deleteMention(view: EditorView, backwards: boolean): boolean {
    const selection = view.state.selection.main
    const touched = view.state.field(mentions).filter((mention) => {
      if (!selection.empty) {
        return selection.from < mention.to && selection.to > mention.from
      }
      if (backwards) {
        return selection.head > mention.from && selection.head <= mention.to
      }
      return selection.head >= mention.from && selection.head < mention.to
    })
    if (!touched.length) return false
    const from = Math.min(
      selection.from,
      ...touched.map((mention) => mention.from)
    )
    const to = Math.max(selection.to, ...touched.map((mention) => mention.to))
    view.dispatch({
      changes: { from, to },
      selection: { anchor: from },
      userEvent: 'delete',
    })
    return true
  }
  function moveAcross(view: EditorView, backwards: boolean): boolean {
    const selection = view.state.selection.main
    if (!selection.empty) return false
    const mention = view.state
      .field(mentions)
      .find((item) =>
        backwards
          ? selection.head > item.from && selection.head <= item.to
          : selection.head >= item.from && selection.head < item.to
      )
    if (!mention) return false
    view.dispatch({
      selection: { anchor: backwards ? mention.from : mention.to },
      scrollIntoView: true,
    })
    return true
  }
  return {
    mentions,
    assets,
    extensions: [
      assets,
      mentions,
      decorations,
      EditorView.atomicRanges.of((view) => view.state.field(decorations)),
      EditorView.clipboardOutputFilter.of((_text, state) => {
        const bindings = state.field(mentions)
        return state.selection.ranges
          .map((range) => {
            const line = state.doc.lineAt(range.head)
            let from = range.empty ? line.from : range.from
            let to = range.empty ? line.to : range.to
            const included = bindings.filter(
              (mention) => mention.from < to && mention.to > from
            )
            for (const mention of included) {
              from = Math.min(from, mention.from)
              to = Math.max(to, mention.to)
            }
            return serializeBoundPrompt(
              state.sliceDoc(from, to),
              included.map((mention) => ({
                ...mention,
                from: mention.from - from,
                to: mention.to - from,
              })),
              state.field(assets)
            )
          })
          .join(state.lineBreak)
      }),
      EditorState.transactionFilter.of((transaction) => {
        if (
          !transaction.docChanged ||
          transaction.effects.some(
            (effect) =>
              effect.is(mentionExternalReset) || effect.is(historyReplay)
          )
        ) {
          return transaction
        }
        const changes: { from: number; to: number; insert: string }[] = []
        transaction.changes.iterChanges((from, to, _fromB, _toB, inserted) => {
          changes.push({ from, to, insert: inserted.toString() })
        })
        if (changes.length !== 1) return transaction
        const change = changes[0]
        const touched = transaction.startState
          .field(mentions)
          .filter(
            (mention) => change.from < mention.to && change.to > mention.from
          )
        if (!touched.length) return transaction
        const from = Math.min(
          change.from,
          ...touched.map((mention) => mention.from)
        )
        const to = Math.max(change.to, ...touched.map((mention) => mention.to))
        if (from === change.from && to === change.to) return transaction
        return {
          changes: { from, to, insert: change.insert },
          selection: { anchor: from + change.insert.length },
          effects: transaction.effects,
          userEvent: transaction.annotation(Transaction.userEvent),
          scrollIntoView: transaction.scrollIntoView,
        }
      }),
      keymap.of([
        {
          key: 'Mod-z',
          run: (view) => replay(view, undo, redo),
          preventDefault: true,
        },
        {
          key: 'Mod-Shift-z',
          run: (view) => replay(view, redo, undo),
          preventDefault: true,
        },
        {
          key: 'Mod-y',
          run: (view) => replay(view, redo, undo),
          preventDefault: true,
        },
        { key: 'Backspace', run: (view) => deleteMention(view, true) },
        { key: 'Delete', run: (view) => deleteMention(view, false) },
        { key: 'ArrowLeft', run: (view) => moveAcross(view, true) },
        { key: 'ArrowRight', run: (view) => moveAcross(view, false) },
        {
          key: 'Enter',
          run: (view) => {
            view.dispatch(view.state.replaceSelection('\n'), {
              userEvent: 'input',
            })
            return true
          },
        },
      ]),
      EditorView.updateListener.of((update) => {
        for (const transaction of update.transactions) {
          if (
            transaction.effects.some((effect) =>
              effect.is(mentionExternalReset)
            )
          ) {
            undo.length = 0
            redo.length = 0
          } else if (
            transaction.docChanged &&
            !transaction.effects.some((effect) => effect.is(historyReplay))
          ) {
            if (
              !transaction.isUserEvent('input.type.compose') ||
              transaction.isUserEvent('input.type.compose.start')
            ) {
              undo.push(snapshot(transaction.startState))
              if (undo.length > 100) undo.shift()
            }
            redo.length = 0
          }
        }
      }),
    ],
  }
}
