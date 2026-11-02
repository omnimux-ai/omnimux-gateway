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
import { act, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, expect, it, vi } from 'vitest'

import { LegalConsent } from '../legal-consent'
import { TermsFooter } from '../terms-footer'

const status = {
  user_agreement_enabled: true,
  privacy_policy_enabled: true,
  legal_consent_required: true,
}

afterEach(async () => {
  await i18next.changeLanguage('en')
})

it('updates both footer variants and policy links when the interface language changes', async () => {
  const signIn = 'By clicking sign in, you agree to our'
  const signUp = 'By creating an account, you agree to our'
  i18next.addResourceBundle('vi', 'translation', {
    [signIn]: 'Khi đăng nhập, bạn đồng ý với',
    [signUp]: 'Khi tạo tài khoản, bạn đồng ý với',
    'User Agreement': 'Thỏa thuận người dùng',
    'Privacy Policy': 'Chính sách quyền riêng tư',
    and: 'và',
  })
  const { rerender } = render(<TermsFooter status={status} />)
  expect(screen.getByText(signIn, { exact: false })).toBeInTheDocument()
  await act(async () => {
    await i18next.changeLanguage('vi')
  })
  expect(
    screen.getByText('Khi đăng nhập, bạn đồng ý với', { exact: false })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('link', { name: 'Thỏa thuận người dùng' })
  ).toHaveAttribute('href', '/user-agreement')
  expect(
    screen.getByRole('link', { name: 'Chính sách quyền riêng tư' })
  ).toHaveAttribute('href', '/privacy-policy')
  rerender(<TermsFooter status={status} variant='sign-up' />)
  expect(
    screen.getByText('Khi tạo tài khoản, bạn đồng ý với', { exact: false })
  ).toBeInTheDocument()
  rerender(
    <TermsFooter
      status={{
        ...status,
        user_agreement_enabled: false,
        privacy_policy_enabled: false,
      }}
    />
  )
  expect(screen.queryByRole('link')).not.toBeInTheDocument()
})

it('translates the consent connector while retaining its required unchecked state', async () => {
  i18next.addResourceBundle('vi', 'translation', {
    'I have read and agree to the': 'Tôi đã đọc và đồng ý với',
    'User Agreement': 'Thỏa thuận người dùng',
    'Privacy Policy': 'Chính sách quyền riêng tư',
    and: 'và',
  })
  await i18next.changeLanguage('vi')
  const onCheckedChange = vi.fn()
  render(
    <LegalConsent
      status={status}
      checked={false}
      onCheckedChange={onCheckedChange}
    />
  )
  expect(screen.getByRole('checkbox')).not.toBeChecked()
  expect(screen.getByText('và', { exact: false })).toBeInTheDocument()
  expect(
    screen.queryByText('and the', { exact: false })
  ).not.toBeInTheDocument()
  expect(onCheckedChange).not.toHaveBeenCalled()
})
