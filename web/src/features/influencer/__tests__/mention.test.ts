import { describe, expect, it } from 'vitest'

import {
  getAvailableMentions,
  normalizeMentionText,
  tokenizePrompt,
} from '../lib/mention'

describe('mention utilities', () => {
  describe('normalizeMentionText', () => {
    it('normalizes <Picture X> into @Image X', () => {
      expect(normalizeMentionText('Let <Picture 1> do something')).toBe(
        'Let @Image 1 do something'
      )
      expect(normalizeMentionText('<picture 12> dances')).toBe(
        '@Image 12 dances'
      )
    })

    it('normalizes <Video X> into @Video X', () => {
      expect(normalizeMentionText('Follow motion of <Video 1>')).toBe(
        'Follow motion of @Video 1'
      )
      expect(normalizeMentionText('<video 2> movement')).toBe(
        '@Video 2 movement'
      )
    })

    it('preserves existing @Video and @Image mentions and fixes spacing', () => {
      expect(normalizeMentionText('@video1 and @image3')).toBe(
        '@Video 1 and @Image 3'
      )
      expect(normalizeMentionText('@Video 1 and @Image 2')).toBe(
        '@Video 1 and @Image 2'
      )
    })

    it('leaves plain text untouched', () => {
      expect(normalizeMentionText('a happy dog running')).toBe(
        'a happy dog running'
      )
      expect(normalizeMentionText('')).toBe('')
    })
  })

  describe('tokenizePrompt', () => {
    it('tokenizes plain text without mentions', () => {
      const tokens = tokenizePrompt('hello world')
      expect(tokens).toEqual([{ type: 'text', content: 'hello world' }])
    })

    it('tokenizes @Video 1 and @Image 1 as mention tokens', () => {
      const tokens = tokenizePrompt('@Video 1 and @Image 2 are cool')
      expect(tokens).toHaveLength(4)
      expect(tokens[0]).toEqual({
        type: 'mention',
        content: '@Video 1',
        item: {
          id: 'video-1',
          type: 'video',
          index: 1,
          name: 'Video 1',
          label: '@Video 1',
        },
      })
      expect(tokens[1]).toEqual({ type: 'text', content: ' and ' })
      expect(tokens[2]).toEqual({
        type: 'mention',
        content: '@Image 2',
        item: {
          id: 'image-2',
          type: 'image',
          index: 2,
          name: 'Image 2',
          label: '@Image 2',
        },
      })
      expect(tokens[3]).toEqual({ type: 'text', content: ' are cool' })
    })

    it('recognizes <Picture 1> on the fly when tokenizing', () => {
      const tokens = tokenizePrompt('Animate <Picture 1> like <Video 1>')
      expect(tokens[1]).toMatchObject({
        type: 'mention',
        content: '@Image 1',
      })
      expect(tokens[3]).toMatchObject({
        type: 'mention',
        content: '@Video 1',
      })
    })
  })

  it('preserves email, invalid indices, incomplete tags and HTML as text', () => {
    const text =
      'user@Image1.com @Image 0 @Image 31 @Video 9999999999999999 <Picture 0> <Video 1 <img src=x>'
    expect(normalizeMentionText(text)).toBe(text)
  })

  it('recognizes references embedded in Chinese without losing surrounding prose', () => {
    expect(normalizeMentionText('让<Picture 1>模仿@Video 1的动作')).toBe(
      '让@Image 1模仿@Video 1的动作'
    )
  })

  it('marks missing references without binding them to a different asset', () => {
    const tokens = tokenizePrompt(
      '@Image 2',
      getAvailableMentions({
        hasVideo: false,
        characterImages: ['https://example.com/one.jpg'],
      })
    )
    expect(tokens[0].missing).toBe(true)
    expect(tokens[0].item?.thumbnail).toBeUndefined()
  })

  it('keeps asset identity stable when an earlier image is removed', () => {
    const before = getAvailableMentions({
      hasVideo: false,
      characterImages: ['one', 'two'],
    })
    const after = getAvailableMentions({
      hasVideo: false,
      characterImages: ['two'],
    })
    expect(before[1].id).toBe(after[0].id)
    expect(after[0].label).toBe('@Image 1')
  })

  describe('getAvailableMentions', () => {
    it('returns empty list when no assets uploaded', () => {
      expect(
        getAvailableMentions({ hasVideo: false, characterImages: [] })
      ).toEqual([])
    })

    it('lists Video 1 when video is present', () => {
      const items = getAvailableMentions({
        hasVideo: true,
        videoThumbnail: 'https://example.com/v.mp4',
        characterImages: [],
      })
      expect(items).toHaveLength(1)
      expect(items[0]).toMatchObject({
        id: 'video:https://example.com/v.mp4',
        type: 'video',
        index: 1,
        name: 'Video 1',
        label: '@Video 1',
        thumbnail: 'https://example.com/v.mp4',
      })
    })

    it('lists images up to uploaded count', () => {
      const items = getAvailableMentions({
        hasVideo: true,
        videoThumbnail: 'https://example.com/v.mp4',
        characterImages: [
          'https://example.com/1.png',
          'https://example.com/2.png',
        ],
      })
      expect(items).toHaveLength(3)
      expect(items[0].name).toBe('Video 1')
      expect(items[1].name).toBe('Image 1')
      expect(items[2].name).toBe('Image 2')
    })
  })
})
