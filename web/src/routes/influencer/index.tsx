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
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { LanguageSwitcher } from '@/components/language-switcher'
import { AppHeader } from '@/components/layout/components/app-header'
import { Button } from '@/components/ui/button'
import { SidebarProvider } from '@/components/ui/sidebar'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { InfluencerWorkbench } from '@/features/influencer'
import { resolveAuthentication } from '@/lib/auth-session'
import { useAuthStore } from '@/stores/auth-store'

const searchSchema = z.object({
  recreate: z.string().optional(),
})

export const Route = createFileRoute('/influencer/')({
  validateSearch: searchSchema,
  beforeLoad: async () => {
    // Guests may browse the workbench; resolve only to tell visitor from user.
    await resolveAuthentication()
  },
  component: InfluencerPage,
})

function GuestHeaderActions() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  return (
    <div className='ms-auto flex shrink-0 items-center gap-1 sm:gap-2'>
      <Button
        variant='ghost'
        size='sm'
        onClick={() =>
          navigate({ to: '/sign-in', search: { redirect: '/influencer' } })
        }
      >
        {t('Sign in')}
      </Button>
      <Button
        size='sm'
        onClick={() =>
          navigate({ to: '/sign-up', search: { redirect: '/influencer' } })
        }
      >
        <span className='hidden sm:inline'>{t('Create a free account')}</span>
        <span className='sm:hidden'>{t('Sign up')}</span>
      </Button>
      <LanguageSwitcher />
    </div>
  )
}

function InfluencerPage() {
  const isGuest = useAuthStore((s) => !s.auth.user || !s.auth.accessToken)
  return (
    <LayoutProvider>
      <SearchProvider>
        <SidebarProvider
          defaultOpen={false}
          className='h-dvh w-full flex-col overflow-hidden bg-[var(--bg-app)]'
        >
          {isGuest ? (
            /* No notification bell / profile menu without an account; the
               sign-in pair is the conversion surface. Search/config stay
               hidden too — they only make sense inside the signed-in shell. */
            <AppHeader
              showTopNav={false}
              showSearch={false}
              showNotifications={false}
              showConfigDrawer={false}
              showProfileDropdown={false}
              rightContent={<GuestHeaderActions />}
            />
          ) : (
            <AppHeader />
          )}
          <main className='min-h-0 w-full flex-1 overflow-hidden'>
            <InfluencerWorkbench />
          </main>
        </SidebarProvider>
      </SearchProvider>
    </LayoutProvider>
  )
}
