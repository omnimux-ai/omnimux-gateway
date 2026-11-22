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

// Older UTM snapshots can retain a word marker after stripping the extension digits.
const PHONE_EXTENSION_SUFFIX =
  /(?:e?xt(?:n|ensi[oó]?n|ens[aã]?o)?|poste|int(?:erno)?|ramal|anexo|dob|доб)[\s.:=(),_#-]*(?:(?:num[eé]?ro|n\.?[°o]?)[\s.:=(),_#-]*)?(?:\p{Decimal_Number}[\p{Decimal_Number}\s()._-]*#?)?$|[x#~][\s.:=(),_#-]*\p{Decimal_Number}[\p{Decimal_Number}\s()._-]*#?$/iu

const PHONE_NUMBER_SEGMENT = /(?:[+(]\s*)?\p{Nd}(?:[\p{Nd}\s()._-]*\p{Nd})?/gu
const DECIMAL_DIGITS = /\p{Nd}/gu
const FORMATTED_DATE_LABEL =
  /^(\d{4}([._-])\d{2}\2\d{2})(?:[._-][a-z][\w.-]*)?$/i
const CALENDAR_DATE_SEGMENT =
  /(?:^|[\s()._-])(\d{4}([._-]?)\d{2}\2\d{2})(?=$|[\s()._-])/g
const VERSION_SEGMENT =
  /(^|[^\p{L}\p{Nd}])v\d+(?:\.\d+)+(?:-((?:0|[1-9]\d*)(?:\.(?:0|[1-9]\d*))*)(?=$|[\s_:/,;]))?(?!\.?\p{Nd})/giu
const UUID =
  /^[\da-f]{8}-[\da-f]{4}-[1-8][\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/i

function isCalendarDate(value: string): boolean {
  const digits = value.replaceAll(/[._-]/g, '')
  const iso = `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6)}`
  return new Date(iso).toJSON()?.slice(0, 10) === iso
}

function hasEmbeddedPhoneNumber(value: string): boolean {
  // Complete UUIDs can have long digit groups and are valid catalog IDs.
  if (UUID.test(value)) return false

  // A delimited numeric prerelease shorter than a dial segment is version metadata.
  const withoutVersions = value.replaceAll(
    VERSION_SEGMENT,
    (_match, boundary: string, prerelease?: string) => {
      const digits = prerelease?.match(DECIMAL_DIGITS)?.length ?? 0
      return `${boundary}:${digits >= 9 ? prerelease : ''}`
    }
  )
  for (const { 0: segment, index } of withoutVersions.matchAll(
    PHONE_NUMBER_SEGMENT
  )) {
    const digits = segment.match(DECIMAL_DIGITS)?.length ?? 0
    if (digits < 9) continue

    if (digits > 19) {
      // Dates can make a separate number's digit count exceed the dial range.
      const remaining = segment.replaceAll(
        CALENDAR_DATE_SEGMENT,
        (match, date: string) => (isCalendarDate(date) ? ':' : match)
      )
      const hasNumber = remaining.split(':').some((part) => {
        const count = part.match(DECIMAL_DIGITS)?.length ?? 0
        return count >= 9 && count <= 19
      })
      if (hasNumber) return true
      continue
    }

    // A catalog snapshot can pair a short major or dotted version with an ISO date.
    const snapshotDate = /^\d{1,3}(?:\.\d{1,3}){0,2}-(\d{4}-\d\d-\d\d)$/.exec(
      segment
    )?.[1]
    if (
      digits < 15 &&
      withoutVersions[index - 1] === '-' &&
      snapshotDate &&
      isCalendarDate(snapshotDate)
    ) {
      continue
    }
    return true
  }
  return false
}

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

  const dialValue = value.normalize('NFKC')
  if (/^tel:/i.test(dialValue) || hasEmbeddedPhoneNumber(dialValue)) return true

  const date = FORMATTED_DATE_LABEL.exec(dialValue)?.[1]
  if (date && isCalendarDate(date)) return false

  // Older UTM snapshots removed URI separators. An unknown suffix needs a
  // full-length subscriber to distinguish it from a short channel label.
  const legacyTel = /^tel([\d.-]+)([a-z][\w.-]*)?$/i.exec(dialValue)
  if (legacyTel) {
    const [, subscriber, parameter] = legacyTel
    const digits = subscriber.replaceAll(/\D/g, '').length
    const knownParameter =
      !parameter ||
      PHONE_EXTENSION_SUFFIX.exec(parameter)?.index === 0 ||
      /^(?:x[._-]*$|ext(?=[.\d-]|$)|isub|npdi|enumdi|(?:rn|cic)(?:[.-]*\d|-context)|tgrp|trunk-context)/i.test(
        parameter
      )
    const minDigits = knownParameter ? 7 : 10
    if (digits >= minDigits && digits <= 19) return true
  }
  // A local phone-context can survive even when cleanup removed all * and #.
  if (/^tel[\da-f.-]*(?:(?:ext|isub)[\w.-]*)?phone-context/i.test(dialValue)) {
    return true
  }

  const phone = dialValue
    .replace(/^t[eé]?l[.:\s]*/iu, '')
    .replace(PHONE_EXTENSION_SUFFIX, '')
    // An attached x can also survive removal of non-ASCII extension digits.
    .replace(/([\p{Decimal_Number})])x[._-]*$/iu, '$1')
  if (!/^[+\p{Decimal_Number}().\s_-]+(?:[,;][,;\s]*)?$/u.test(phone)) {
    return false
  }
  // Include international dial prefixes within the phone-shaped length range.
  const digits = phone.match(DECIMAL_DIGITS)?.length ?? 0
  return digits >= 7 && digits <= 19
}
