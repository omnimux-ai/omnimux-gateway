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
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import { Turnstile } from '@/components/turnstile'
import { useAuthStore } from '@/stores/auth-store'

import { Icon } from '../../components/cn/icons'
import { useCopy } from '../../i18n'
import { TOOL_PATHS, type ToolKind, type ToolLocale } from '../content'
import { useToolResolver, type ToolFailure } from '../lib/use-tool-resolver'
import { ToolPhotoResult, ToolVideoResult } from './tool-result'

const canReadClipboard =
  typeof navigator !== 'undefined' &&
  typeof navigator.clipboard?.readText === 'function'

export function ToolForm(props: {
  kind: ToolKind
  locale: ToolLocale
  initialUrl?: string
}) {
  const landing = useCopy()
  const copy = landing.toolPage
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.auth.user)
  const [value, setValue] = useState(props.initialUrl ?? '')
  const [invalid, setInvalid] = useState(false)

  const signIn = useCallback(() => {
    void navigate({
      to: '/sign-in',
      search: {
        redirect: `${window.location.pathname}${window.location.search}`,
      },
    })
  }, [navigate])

  const onFailure = useCallback(
    (failure: ToolFailure) => {
      const errors = copy.errors
      if (failure === 'invalid_link') setInvalid(true)
      if (failure === 'wrong_kind_photo' || failure === 'wrong_kind_video') {
        const target: ToolKind =
          failure === 'wrong_kind_photo' ? 'photo' : 'video'
        toast.error(
          target === 'photo' ? errors.wrongKindPhoto : errors.wrongKindVideo,
          {
            action: {
              label: target === 'photo' ? errors.toPhoto : errors.toVideo,
              onClick: () =>
                void navigate({
                  href: `${TOOL_PATHS[props.locale][target]}?url=${encodeURIComponent(value.trim())}`,
                }),
            },
          }
        )
        return
      }
      const loginAction = user
        ? undefined
        : { label: landing.nav.login, onClick: signIn }
      const messages: Record<ToolFailure, string> = {
        invalid_link: errors.invalidLink,
        wrong_kind_photo: errors.wrongKindPhoto,
        wrong_kind_video: errors.wrongKindVideo,
        unavailable: errors.unavailable,
        service_unavailable: errors.serviceUnavailable,
        upstream_failed: errors.upstreamFailed,
        quota_exceeded: user ? errors.quotaUser : errors.quotaAnon,
        busy: errors.busy,
        rate_limited: errors.rateLimited,
        turnstile_failed: errors.turnstileFailed,
        turnstile_unavailable: errors.turnstileUnavailable,
        generic: errors.generic,
      }
      const withLogin = failure === 'quota_exceeded' || failure === 'busy'
      toast.error(
        messages[failure],
        withLogin ? { action: loginAction } : undefined
      )
    },
    [
      copy.errors,
      landing.nav.login,
      navigate,
      props.locale,
      signIn,
      user,
      value,
    ]
  )

  const resolver = useToolResolver({ onFailure })
  const loggedIn = resolver.config?.logged_in ?? Boolean(user)
  const quota = resolver.quota
  const remaining = quota ? Math.max(0, quota.limit - quota.used) : null
  const exhausted = remaining === 0
  const unavailable = resolver.config?.enabled === false
  const busy = resolver.phase !== 'idle'

  // A closed tool is the button's own state, so the label carries it and the
  // tooltip names the cause. A used-up quota belongs to the visitor, so the
  // button keeps its verb and the reason stays on the tooltip and the click.
  let blockedReason = ''
  let blockedLabel: string | null = null
  if (unavailable) {
    blockedLabel = copy.unavailable
    blockedReason = copy.errors.serviceUnavailable
  } else if (exhausted) {
    blockedReason = loggedIn ? copy.errors.quotaUser : copy.errors.quotaAnon
  }

  const quotaTemplate = loggedIn ? copy.quotaUser : copy.quotaAnon
  const [quotaLead] = quotaTemplate.split('{remaining}')
  const placeholder =
    props.kind === 'video' ? copy.placeholderVideo : copy.placeholderPhoto
  const siteKey = resolver.config?.turnstile_site_key ?? ''

  return (
    <>
      <form
        className='composer tool-box'
        noValidate
        autoComplete='off'
        onSubmit={(event) => {
          event.preventDefault()
          if (busy) return
          if (blockedReason) {
            toast.error(blockedReason)
            return
          }
          const url = value.trim()
          if (!url) {
            onFailure('invalid_link')
            return
          }
          resolver.submit(url, props.kind)
        }}
      >
        <div className='tool-row'>
          <div className='tool-field' data-invalid={invalid}>
            <Icon name='link' />
            <label className='sr-only' htmlFor='tool-url'>
              {copy.inputLabel}
            </label>
            <input
              id='tool-url'
              type='text'
              inputMode='url'
              value={value}
              placeholder={placeholder}
              aria-invalid={invalid}
              aria-describedby='tool-quota'
              onChange={(event) => {
                setValue(event.target.value)
                setInvalid(false)
              }}
            />
            {canReadClipboard ? (
              <button
                type='button'
                className='tool-paste'
                onClick={async () => {
                  try {
                    const text = await navigator.clipboard.readText()
                    if (!text) return
                    setValue(text)
                    setInvalid(false)
                  } catch {
                    /* Permission denied: manual paste still works. */
                  }
                }}
              >
                {copy.paste}
              </button>
            ) : null}
          </div>
          <button
            type='submit'
            className='btn btn-light tool-go'
            aria-disabled={busy || Boolean(blockedReason) || undefined}
            title={blockedReason || undefined}
            aria-describedby={blockedReason ? 'tool-go-reason' : undefined}
          >
            {busy ? <span className='tool-spin' aria-hidden='true' /> : null}
            {resolver.phase === 'idle' ? (blockedLabel ?? copy.parse) : null}
            {resolver.phase === 'verifying' ? copy.verifying : null}
            {resolver.phase === 'parsing' ? copy.parsing : null}
          </button>
          {blockedReason ? (
            <span className='sr-only' id='tool-go-reason'>
              {blockedReason}
            </span>
          ) : null}
        </div>
        {resolver.challengeOpen && siteKey ? (
          <div className='tool-ts' role='group' aria-label={copy.turnstileAria}>
            <Turnstile
              key={resolver.challenge}
              siteKey={siteKey}
              theme='dark'
              onVerify={(token) => void resolver.onToken(token)}
              onExpire={resolver.onTokenExpire}
            />
          </div>
        ) : null}
        <div className='tool-meta'>
          <span id='tool-quota'>
            {quota && remaining !== null ? (
              <>
                {quotaLead}
                <b>
                  {remaining}/{quota.limit}
                </b>
              </>
            ) : null}
          </span>
        </div>
      </form>
      {resolver.result?.kind === 'video' ? (
        <ToolVideoResult result={resolver.result} />
      ) : null}
      {resolver.result?.kind === 'photo' ? (
        <ToolPhotoResult result={resolver.result} loggedIn={loggedIn} />
      ) : null}
    </>
  )
}
