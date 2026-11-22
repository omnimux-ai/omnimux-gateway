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

export function looksLikePersonalValue(value: string): boolean {
  if (value.includes('@') || value.includes('＠')) return true
  if (/^(?:\d{1,3}\.){3}\d{1,3}(?:\.|:\d+)?$/.test(value)) return true

  const ipv6 =
    /^\[?([\da-f.]*:[\da-f.]*:[\da-f:.]*)(?:%[\w.-]+)?(?:\](?::\d+)?)?$/i.exec(
      value
    )?.[1]
  if (ipv6) {
    try {
      new URL(`http://[${ipv6}]`)
      return true
    } catch {
      // A colon-delimited catalog ID can resemble an IPv6 address.
    }
  }

  if (/^tel:/i.test(value)) return true

  // Older UTM snapshots removed URI separators. An unknown suffix needs a
  // full-length subscriber to distinguish it from a short channel label.
  const legacyTel = /^tel([\d.-]+)([a-z][\w.-]*)?$/i.exec(value)
  if (legacyTel) {
    const [, subscriber, parameter] = legacyTel
    const digits = subscriber.replaceAll(/\D/g, '').length
    const knownParameter =
      !parameter ||
      /^(?:ext(?=[.\d-]|$)|isub|npdi|enumdi|(?:rn|cic)(?:[.-]*\d|-context)|tgrp|trunk-context)/i.test(
        parameter
      )
    const minDigits = knownParameter ? 7 : 10
    if (digits >= minDigits && digits <= 19) return true
  }
  // A local phone-context can survive even when cleanup removed all * and #.
  if (/^tel[\da-f.-]*(?:(?:ext|isub)[\w.-]*)?phone-context/i.test(value)) {
    return true
  }

  const phone = value.replace(
    /(?:ext(?:ension)?|[x#])[\s()._-]*\d[\d\s()._-]*$/i,
    ''
  )
  if (!/^[+\d().\s_-]+$/.test(phone)) return false
  // Include international dial prefixes within the phone-shaped length range.
  const digits = phone.replaceAll(/\D/g, '').length
  return digits >= 7 && digits <= 19
}
