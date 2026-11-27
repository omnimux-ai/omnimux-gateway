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
import { zodResolver } from '@hookform/resolvers/zod'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const socialOpsSeatsSchema = z.object({
  social_ops_setting: z.object({
    billing_mode: z.enum(['off', 'seat']),
    seat_price_usd: z.coerce.number().min(0),
    seats_cap: z.coerce.number().int().min(0),
    grandfather_days: z.coerce.number().int().min(0),
    grace_days: z.coerce.number().int().min(0),
  }),
})

type SocialOpsSeatsFormValues = z.infer<typeof socialOpsSeatsSchema>

type SocialOpsSeatsSectionProps = {
  defaultValues: SocialOpsSeatsFormValues
}

export function SocialOpsSeatsSection({
  defaultValues,
}: SocialOpsSeatsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<SocialOpsSeatsFormValues>({
      resolver: zodResolver(socialOpsSeatsSchema) as Resolver<
        SocialOpsSeatsFormValues,
        unknown,
        SocialOpsSeatsFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          if (value === undefined || value === null) continue
          if (typeof value === 'object') continue
          let serialized: string | number | boolean = value as
            | string
            | number
            | boolean
          if (typeof value === 'boolean' || typeof value === 'number') {
            serialized = String(value)
          }
          await updateOption.mutateAsync({
            key,
            value: serialized,
          })
        }
      },
    })

  return (
    <SettingsSection title={t('Social account binding seats')}>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Manual renewal only, 30 days per cycle, up to 90 days total, and at most 2 cross-account swaps per cycle.'
            )}
          </p>
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='social_ops_setting.billing_mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Billing mode')}</FormLabel>
                  <Select
                    onValueChange={field.onChange}
                    value={field.value}
                    disabled={updateOption.isPending || isSubmitting}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='off'>
                          {t('off (seat billing disabled)')}
                        </SelectItem>
                        <SelectItem value='seat'>
                          {t('seat (per-seat billing)')}
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t(
                      'Disabled by default; wallet seat card appears only in seat mode.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='social_ops_setting.seat_price_usd'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Seat price (USD / 30 days)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step='0.01'
                      value={field.value ?? ''}
                      onChange={(event) => {
                        const n = event.currentTarget.valueAsNumber
                        field.onChange(Number.isNaN(n) ? 0 : n)
                      }}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('1 USD = 10 credits. Default: 2 (20 credits).')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='social_ops_setting.seats_cap'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Seat limit per user')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value ?? ''}
                      onChange={(event) => {
                        const n = event.currentTarget.valueAsNumber
                        field.onChange(Number.isNaN(n) ? 0 : n)
                      }}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Includes occupied and unexpired vacant seats. Default: 20.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='social_ops_setting.grandfather_days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Transition days for existing accounts')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value ?? ''}
                      onChange={(event) => {
                        const n = event.currentTarget.valueAsNumber
                        field.onChange(Number.isNaN(n) ? 0 : n)
                      }}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      '0 freezes existing accounts without a seat immediately. Default: 0.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='social_ops_setting.grace_days'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiry freeze grace days')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value ?? ''}
                      onChange={(event) => {
                        const n = event.currentTarget.valueAsNumber
                        field.onChange(Number.isNaN(n) ? 0 : n)
                      }}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accounts auto-unlink after grace expires without renewal. Default: 7.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
