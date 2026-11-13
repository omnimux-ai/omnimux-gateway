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
  autocompletion,
  closeCompletion,
  type CompletionContext,
} from '@codemirror/autocomplete'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, placeholder } from '@codemirror/view'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import {
  createMentionState,
  mentionAssets,
  mentionCompositionEnd,
  mentionExternalReset,
  serializeBoundPrompt,
} from '../hooks/use-mention-editor-state'
import type { MentionItem } from '../lib/mention'

import './mention-prompt-editor.css'

interface MentionPromptEditorProps {
  value: string
  onChange: (value: string) => void
  items: readonly MentionItem[]
  placeholder: string
  /** Optional rune ceiling: renders a counter instead of an inline error. */
  maxLength?: number
}

/** The shared code editor has no extension slot for atomic, identity-bound
 * references. This specialized view reuses its installed CodeMirror engine. */
export function MentionPromptEditor(props: MentionPromptEditorProps) {
  const { t } = useTranslation()
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const latest = useRef(props)
  const translate = useRef(t)
  const published = useRef(props.value)
  const configuration = useRef(new Compartment())
  const syncedAssets = useRef(props.items)
  const incomingValue = useRef(props.value)
  latest.current = props
  translate.current = t

  useEffect(() => {
    if (!host.current) return
    const model = createMentionState({
      initialAssets: latest.current.items,
      missingLabel: () => translate.current('Missing reference'),
    })
    function complete(context: CompletionContext) {
      const before = context.matchBefore(/@(?:[A-Za-z]*\s*\d*)?$/)
      if (!before) return null
      if (
        before.from > 0 &&
        /[\w@]/.test(context.state.sliceDoc(before.from - 1, before.from))
      ) {
        return null
      }
      return {
        from: before.from,
        filter: false,
        options: context.state.field(model.assets).map((item) => ({
          label: item.label.replace(/\s+(\d+)$/, '$1'),
          displayLabel: item.name,
          apply(
            editorView: EditorView,
            _completion: unknown,
            from: number,
            to: number
          ) {
            const current = editorView.state
              .field(model.assets)
              .find((asset) => asset.id === item.id)
            closeCompletion(editorView)
            if (!current) return
            const insert = `${current.label} `
            editorView.dispatch({
              changes: { from, to, insert },
              selection: { anchor: from + insert.length },
              userEvent: 'input.complete',
            })
          },
          type: item.type,
          item,
        })),
        validFor: /^@(?:[A-Za-z]*\s*\d*)?$/,
      }
    }
    const editor = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: latest.current.value,
        extensions: [
          ...model.extensions,
          EditorView.lineWrapping,
          EditorView.domEventHandlers({
            compositionend(_event, editorView) {
              queueMicrotask(() => {
                if (view.current === editorView) {
                  editorView.dispatch({
                    effects: mentionCompositionEnd.of(true),
                  })
                }
              })
              return false
            },
          }),
          configuration.current.of([
            placeholder(
              `${latest.current.placeholder} · ${translate.current('Type @ to reference an asset')}`
            ),
            EditorView.contentAttributes.of({
              'aria-label': translate.current('Prompt'),
              'aria-multiline': 'true',
            }),
          ]),
          autocompletion({
            override: [complete],
            activateOnTyping: true,
            interactionDelay: 0,
            icons: false,
            defaultKeymap: true,
            selectOnOpen: true,
            addToOptions: [
              {
                position: 10,
                render(completion) {
                  const item = (
                    completion as typeof completion & { item: MentionItem }
                  ).item
                  const element = document.createElement(
                    item.type === 'video' ? 'video' : 'img'
                  )
                  element.className = 'mention-prompt-thumb'
                  element.setAttribute('aria-hidden', 'true')
                  if (item.thumbnail) {
                    element.setAttribute('src', item.thumbnail)
                  }
                  if (element instanceof HTMLVideoElement) {
                    element.muted = true
                    element.preload = 'metadata'
                  } else if (element instanceof HTMLImageElement) {
                    element.alt = ''
                  }
                  return element
                },
              },
            ],
          }),
          EditorView.updateListener.of((update) => {
            if (
              !update.docChanged &&
              !update.transactions.some((transaction) =>
                transaction.effects.some(
                  (effect) =>
                    effect.is(mentionAssets) || effect.is(mentionCompositionEnd)
                )
              )
            ) {
              return
            }
            const plain = serializeBoundPrompt(
              update.state.doc.toString(),
              update.state.field(model.mentions),
              update.state.field(model.assets)
            )
            if (plain !== published.current) {
              published.current = plain
              latest.current.onChange(plain)
            }
          }),
        ],
      }),
    })
    view.current = editor
    // Recognized pasted/initial bracket syntax is sent as canonical plain text.
    const initial = serializeBoundPrompt(
      editor.state.doc.toString(),
      editor.state.field(model.mentions),
      latest.current.items
    )
    if (initial !== published.current) {
      published.current = initial
      latest.current.onChange(initial)
    }
    return () => {
      editor.destroy()
      view.current = null
    }
  }, [])

  useEffect(() => {
    const editor = view.current
    if (!editor) return
    const assetsChanged = syncedAssets.current !== props.items
    const externallyReset =
      props.value !== incomingValue.current && props.value !== published.current
    incomingValue.current = props.value
    if (!assetsChanged && !externallyReset) return
    syncedAssets.current = props.items
    closeCompletion(editor)
    if (externallyReset) {
      published.current = props.value
      editor.dispatch({
        changes: { from: 0, to: editor.state.doc.length, insert: props.value },
        selection: { anchor: 0 },
        effects: [mentionAssets.of(props.items), mentionExternalReset.of(true)],
      })
    } else {
      editor.dispatch({ effects: mentionAssets.of(props.items) })
    }
  }, [props.value, props.items])

  useEffect(() => {
    view.current?.dispatch({
      effects: configuration.current.reconfigure([
        placeholder(
          `${props.placeholder} · ${t('Type @ to reference an asset')}`
        ),
        EditorView.contentAttributes.of({
          'aria-label': t('Prompt'),
          'aria-multiline': 'true',
        }),
      ]),
    })
  }, [props.placeholder, t])

  const intentCount = [...props.value.trim()].length

  return (
    <div className='mention-prompt'>
      <div ref={host} className='mention-prompt-host' />
      {props.maxLength ? (
        <span
          className={
            intentCount > props.maxLength
              ? 'mention-prompt-count over'
              : 'mention-prompt-count'
          }
          aria-hidden='true'
        >
          {intentCount} / {props.maxLength}
        </span>
      ) : null}
    </div>
  )
}
