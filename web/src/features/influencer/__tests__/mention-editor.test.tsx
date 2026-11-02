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
import { startCompletion } from '@codemirror/autocomplete'
import { EditorView, runScopeHandlers } from '@codemirror/view'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { MentionPromptEditor } from '../components/mention-prompt-editor'
import type { MentionItem } from '../lib/mention'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
afterEach(cleanup)

const imageA: MentionItem = {
  id: 'image:https://assets/a.png',
  type: 'image',
  index: 1,
  name: 'Image 1',
  label: '@Image 1',
  thumbnail: 'https://assets/a.png',
}
const imageB: MentionItem = {
  id: 'image:https://assets/b.png',
  type: 'image',
  index: 2,
  name: 'Image 2',
  label: '@Image 2',
  thumbnail: 'https://assets/b.png',
}
const video: MentionItem = {
  id: 'video:https://assets/a.mp4',
  type: 'video',
  index: 1,
  name: 'Video 1',
  label: '@Video 1',
  thumbnail: 'https://assets/a.mp4',
}
const items = [video, imageA, imageB]

function Harness(props: { initial?: string; items?: MentionItem[] }) {
  const [value, setValue] = useState(props.initial ?? '')
  return (
    <>
      <MentionPromptEditor
        value={value}
        onChange={setValue}
        items={props.items ?? items}
        placeholder='Describe movement'
      />
      <output aria-label='Plain prompt'>{value}</output>
    </>
  )
}
function editor(): EditorView {
  const content = screen.getByRole('textbox', { name: 'Prompt' })
  const view = EditorView.findFromDOM(content)
  if (!view) throw new Error('Missing editor')
  return view
}
function typeText(text: string) {
  const view = editor()
  act(() => {
    view.dispatch(view.state.replaceSelection(text), { userEvent: 'input' })
  })
}
function key(
  key: string,
  options: { ctrlKey?: boolean; shiftKey?: boolean } = {}
) {
  editor().focus()
  fireEvent.keyDown(screen.getByRole('textbox', { name: 'Prompt' }), {
    key,
    ...options,
  })
}

describe('mention prompt editor', () => {
  test('recognizes bracket image/video syntax and publishes a plain prompt', () => {
    render(<Harness initial={'用<Picture1>参考<Video1>\n<Image2>'} />)
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent(
      '用@Image 1参考@Video 1 @Image 2'
    )
    expect(
      within(screen.getByRole('textbox', { name: 'Prompt' })).getByText(
        '@Image 1'
      )
    ).toBeInTheDocument()
    expect(screen.getByText('@Video 1')).toBeInTheDocument()
  })

  test('shows only uploaded candidates at the caret and accepts keyboard selection', async () => {
    render(<Harness />)
    typeText('前文 @')
    act(() => {
      startCompletion(editor())
    })
    const option = await screen.findByRole('option', { name: 'Video 1' })
    expect(option.querySelector('video')).toHaveAttribute(
      'src',
      video.thumbnail
    )
    expect(screen.getAllByRole('option')).toHaveLength(3)
    key('ArrowDown')
    key('Enter')
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent(
      '前文 @Image 1'
    )
    expect(
      within(screen.getByRole('textbox', { name: 'Prompt' })).getByText(
        '@Image 1'
      )
    ).toBeInTheDocument()
  })

  test.each(['Backspace', 'Delete'])(
    '%s removes the entire reference without removing its asset',
    (deleteKey) => {
      render(<Harness initial='@Image 1' />)
      act(() => {
        editor().dispatch({
          selection: {
            anchor: deleteKey === 'Backspace' ? editor().state.doc.length : 0,
          },
        })
      })
      key(deleteKey)
      expect(screen.getByLabelText('Plain prompt')).toBeEmptyDOMElement()
      expect(items).toHaveLength(3)
      key('z', { ctrlKey: true })
      expect(
        within(screen.getByRole('textbox', { name: 'Prompt' })).getByText(
          '@Image 1'
        )
      ).toBeInTheDocument()
      key('z', { ctrlKey: true, shiftKey: true })
      expect(screen.getByLabelText('Plain prompt')).toBeEmptyDOMElement()
    }
  )

  test('retains original asset identity when a preceding image is removed', () => {
    const { rerender } = render(
      <Harness initial='@Image 2' items={[imageA, imageB]} />
    )
    rerender(
      <Harness
        initial='@Image 2'
        items={[{ ...imageB, index: 1, name: 'Image 1', label: '@Image 1' }]}
      />
    )
    expect(
      within(screen.getByRole('textbox', { name: 'Prompt' })).getByText(
        '@Image 1'
      )
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent('@Image 1')
    rerender(<Harness initial='@Image 2' items={[{ ...imageA, index: 1 }]} />)
    expect(
      screen.getByLabelText('@Image 2 · Missing reference')
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Plain prompt')).not.toHaveTextContent(
      '@Image'
    )
  })

  test('does not bind a missing reference when an image is uploaded later', () => {
    const { rerender } = render(<Harness initial='@Image 1' items={[]} />)
    rerender(<Harness initial='@Image 1' items={[imageA]} />)
    expect(screen.getByText(/@Image 1 · Missing reference/)).toBeInTheDocument()
    expect(screen.getByLabelText('Plain prompt')).not.toHaveTextContent(
      '@Image'
    )
  })

  test('partial selection replacement removes the complete capsule', () => {
    render(<Harness initial='@Image 1' />)
    act(() => {
      editor().dispatch({
        changes: { from: 2, to: 4, insert: '文字' },
        userEvent: 'input.paste',
      })
    })
    expect(editor().state.doc.toString()).toBe('文字')
    expect(screen.queryByText('@Image 1')).not.toBeInTheDocument()
  })

  test('asset changes close stale menu candidates rather than binding their old number', async () => {
    const { rerender } = render(<Harness items={[imageA]} />)
    typeText('@')
    act(() => {
      startCompletion(editor())
    })
    await screen.findByRole('option', { name: 'Image 1' })
    rerender(
      <Harness
        items={[{ ...imageB, index: 1, name: 'Image 1', label: '@Image 1' }]}
      />
    )
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent('@')
  })

  test('external reset clears the draft and does not allow undo to restore it', () => {
    const onChange = vi.fn()
    const { rerender } = render(
      <MentionPromptEditor
        value='初稿'
        onChange={onChange}
        items={items}
        placeholder='Describe movement'
      />
    )
    act(() => {
      editor().dispatch({ selection: { anchor: 2 } })
    })
    typeText('更新')
    rerender(
      <MentionPromptEditor
        value=''
        onChange={onChange}
        items={items}
        placeholder='Describe movement'
      />
    )
    key('z', { ctrlKey: true })
    expect(editor().state.doc.toString()).toBe('')
  })

  test('keeps Chinese text, cursor and newline through controlled rerenders', () => {
    render(<Harness />)
    typeText('中文')
    key('Enter')
    typeText('第二行')
    expect(editor().state.doc.toString()).toBe('中文\n第二行')
    expect(editor().state.selection.main.head).toBe(6)
  })

  test('typing a multi-digit reference does not prematurely bind its first digit', () => {
    const image30 = {
      ...imageB,
      index: 30,
      name: 'Image 30',
      label: '@Image 30',
    }
    render(<Harness items={[imageA, image30]} />)
    act(() => {
      editor().dispatch({
        changes: { from: 0, insert: '@Image3' },
        selection: { anchor: 7 },
        userEvent: 'input.type',
      })
    })
    expect(screen.queryByText(/Missing reference/)).not.toBeInTheDocument()
    act(() => {
      editor().dispatch({
        changes: { from: 7, insert: '0 ' },
        selection: { anchor: 9 },
        userEvent: 'input.type',
      })
    })
    expect(
      within(screen.getByRole('textbox', { name: 'Prompt' })).getByText(
        '@Image 30'
      )
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent('@Image 30')
  })

  test('composition updates stay unwrapped until completion and undo as one edit', async () => {
    render(<Harness />)
    act(() => {
      editor().dispatch({
        changes: { from: 0, insert: '@Image' },
        selection: { anchor: 6 },
        userEvent: 'input.type.compose.start',
      })
    })
    act(() => {
      editor().dispatch({
        changes: { from: 6, insert: '1' },
        selection: { anchor: 7 },
        userEvent: 'input.type.compose',
      })
    })
    expect(screen.queryByText('@Image 1')).not.toBeInTheDocument()
    fireEvent.compositionEnd(screen.getByRole('textbox', { name: 'Prompt' }))
    await screen.findByText('@Image 1')
    // The engine intentionally suppresses keys immediately after IME commit;
    // exercise the public history keymap without faking an elapsed delay.
    act(() => {
      runScopeHandlers(
        editor(),
        new KeyboardEvent('keydown', { key: 'z', ctrlKey: true }),
        'editor'
      )
    })
    expect(screen.getByLabelText('Plain prompt')).toBeEmptyDOMElement()
  })

  test('mouse selection replaces the trigger with a reference', async () => {
    render(<Harness />)
    typeText('@')
    act(() => {
      startCompletion(editor())
    })
    fireEvent.mouseDown(await screen.findByRole('option', { name: 'Image 2' }))
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent('@Image 2')
  })

  test('pastes plain text and converts recognized references without injecting HTML', () => {
    render(<Harness />)
    fireEvent.paste(screen.getByRole('textbox', { name: 'Prompt' }), {
      clipboardData: {
        getData: (type: string) =>
          type === 'text/plain'
            ? '<Picture1>\n中文<b>text</b>'
            : '<img src=x onerror=alert(1)>',
        files: [],
      },
    })
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent(
      '@Image 1 中文<b>text</b>'
    )
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  test('hidden draft keeps reference identity while assets change', () => {
    const onChange = vi.fn()
    const { rerender } = render(
      <div hidden>
        <MentionPromptEditor
          value='@Image 1'
          onChange={onChange}
          items={[imageA]}
          placeholder='Describe movement'
        />
      </div>
    )
    rerender(
      <div hidden>
        <MentionPromptEditor
          value='@Image 1'
          onChange={onChange}
          items={[{ ...imageB, index: 1, label: '@Image 1' }]}
          placeholder='Describe movement'
        />
      </div>
    )
    rerender(
      <div>
        <MentionPromptEditor
          value='@Image 1'
          onChange={onChange}
          items={[{ ...imageB, index: 1, label: '@Image 1' }]}
          placeholder='Describe movement'
        />
      </div>
    )
    expect(
      screen.getByLabelText('@Image 1 · Missing reference')
    ).toBeInTheDocument()
    expect(onChange).toHaveBeenLastCalledWith('[Missing reference: Image 1]')
  })

  test('external prompt and assets update together binds the replacement asset', () => {
    const onChange = vi.fn()
    const { rerender } = render(
      <MentionPromptEditor
        value='old'
        onChange={onChange}
        items={[imageA]}
        placeholder='Prompt'
      />
    )
    rerender(
      <MentionPromptEditor
        value='<Picture 1>'
        onChange={onChange}
        items={[{ ...imageB, index: 1, name: 'Image 1', label: '@Image 1' }]}
        placeholder='Prompt'
      />
    )
    expect(screen.queryByText(/Missing reference/)).not.toBeInTheDocument()
    expect(onChange).toHaveBeenLastCalledWith('@Image 1')
  })

  test('copy after renumbering exports the current label rather than another asset number', () => {
    const imageC = {
      ...imageA,
      id: 'image:c',
      index: 3,
      name: 'Image 3',
      label: '@Image 3',
    }
    const { rerender } = render(
      <Harness initial='@Image 2' items={[imageA, imageB, imageC]} />
    )
    rerender(
      <Harness
        initial='@Image 2'
        items={[
          { ...imageB, index: 1, name: 'Image 1', label: '@Image 1' },
          { ...imageC, index: 2, name: 'Image 2', label: '@Image 2' },
        ]}
      />
    )
    act(() =>
      editor().dispatch({
        selection: { anchor: 0, head: editor().state.doc.length },
      })
    )
    editor().focus()
    const setData = vi.fn()
    fireEvent.copy(screen.getByRole('textbox', { name: 'Prompt' }), {
      clipboardData: { setData, clearData: vi.fn() },
    })
    expect(setData).toHaveBeenCalledWith('text/plain', '@Image 1')
  })

  test('Escape closes the caret menu without changing the prompt', async () => {
    render(<Harness />)
    typeText('@')
    act(() => {
      startCompletion(editor())
    })
    await screen.findByRole('listbox')
    key('Escape')
    await waitFor(() =>
      expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    )
    expect(screen.getByLabelText('Plain prompt')).toHaveTextContent('@')
  })
})
