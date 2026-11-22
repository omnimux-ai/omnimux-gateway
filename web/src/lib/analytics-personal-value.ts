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

const PHONE_NUMBER_SEGMENT = /(?:[+(]\s*)?\p{Nd}(?:[\p{Nd}\s().-]*\p{Nd})?/gu
const UNDERSCORE_PHONE_GROUPS =
  /(?<![\p{Nd}_])(?:\p{Nd}{1,4}_){2,}\p{Nd}{1,4}(?![\p{Nd}_])/gu
const DECIMAL_DIGITS = /\p{Nd}/gu
const FORMATTED_DATE_LABEL =
  /^(\d{4}([._-])\d{2}\2\d{2})(?:[._-][a-z][\w.-]*)?$/i
const CALENDAR_DATE_SEGMENT =
  /(?:^|[\s()_])(\d{4}([._-])\d{2}\2\d{2})(?=$|[\s()_])/g
const LEADING_DATE_WITH_DIAL_SEPARATOR = /^(\d{4}([.-])\d{2}\2\d{2})[.-]/
const TRAILING_DATE_WITH_DIAL_SEPARATOR = /[.-](\d{4}([.-])\d{2}\2\d{2})$/
const SNAPSHOT_PREFIX = /(?:[a-z]-|^o)$/i
const SNAPSHOT_VERSION_DATE =
  /^\d{1,2}(?:[.-]\d{1,2}){0,2}-(\d{4}-\d\d-\d\d|\d{8})(?=$|[\s().-])/
const LEGACY_ISO_SNAPSHOT = /^\d{1,3}(?:\.\d{1,3}){0,2}-(\d{4}-\d\d-\d\d)$/
const COMPACT_SUBSCRIBER_DIAL = /^\d{2}-\d{2}-\d{8}$/
const PHONE_LABEL_BEFORE_NUMBER =
  /(?<![\p{L}\p{Nd}])(?:phone|fax|t[eé]?l(?:[eé]?(?:ph|f)on[eo]?)?)-$/iu
const VERSION_SEGMENT =
  /(^|[^\p{L}\p{Nd}])v\d+(?:\.\d+)+(?:-((?:0|[1-9]\d*)(?:\.(?:0|[1-9]\d*))*)(?=$|[\s_:/,;]))?(?!\.?\p{Nd})/giu
const UUID =
  /^[\da-f]{8}-[\da-f]{4}-[1-8][\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/i

function isCalendarDate(value: string): boolean {
  const digits = value.replaceAll(/[._-]/g, '')
  const iso = `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6)}`
  return new Date(iso).toJSON()?.slice(0, 10) === iso
}

function countDecimalDigits(value: string): number {
  return value.match(DECIMAL_DIGITS)?.length ?? 0
}

function hasDialLength(value: string): boolean {
  const digits = countDecimalDigits(value)
  return digits >= 9 && digits <= 19
}

function separateCalendarDate(match: string, date: string): string {
  return isCalendarDate(date) ? ':' : match
}

function hasPhoneInSeparatedSegment(segment: string): boolean {
  const withoutDates = segment
    .replaceAll(CALENDAR_DATE_SEGMENT, separateCalendarDate)
    // Older snapshots could join a formatted date and a number at either edge.
    .replace(LEADING_DATE_WITH_DIAL_SEPARATOR, separateCalendarDate)
    .replace(TRAILING_DATE_WITH_DIAL_SEPARATOR, separateCalendarDate)
  for (const part of withoutDates.split(':')) {
    const digits = countDecimalDigits(part)
    if (digits < 9) continue
    if (digits <= 19 || part.split(/\s+/).some(hasDialLength)) return true
  }
  return false
}

function catalogSnapshotLength(
  prefix: string,
  segment: string,
  digits: number
): number {
  if (PHONE_LABEL_BEFORE_NUMBER.test(prefix)) return 0

  // Two dialing groups and a compact subscriber still identify a number after
  // first-touch storage strips a language-specific label.
  if (SNAPSHOT_PREFIX.test(prefix)) {
    const snapshot = SNAPSHOT_VERSION_DATE.exec(segment)
    if (
      snapshot &&
      !COMPACT_SUBSCRIBER_DIAL.test(snapshot[0]) &&
      isCalendarDate(snapshot[1])
    ) {
      return snapshot[0].length
    }
  }

  if (digits >= 15 || !prefix.endsWith('-')) return 0
  const isoDate = LEGACY_ISO_SNAPSHOT.exec(segment)?.[1]
  return isoDate && isCalendarDate(isoDate) ? segment.length : 0
}

function hasEmbeddedPhoneNumber(value: string): boolean {
  // Complete UUIDs can have long digit groups and are valid catalog IDs.
  if (UUID.test(value)) return false

  // A delimited numeric prerelease shorter than a dial segment is version metadata.
  const withoutVersions = value.replaceAll(
    VERSION_SEGMENT,
    (_match, boundary: string, prerelease?: string) => {
      const digits = countDecimalDigits(prerelease ?? '')
      return `${boundary}:${digits >= 9 ? prerelease : ''}`
    }
  )
  for (const { 0: segment, index } of withoutVersions.matchAll(
    PHONE_NUMBER_SEGMENT
  )) {
    const digits = countDecimalDigits(segment)
    if (digits < 9) continue

    const snapshotLength = catalogSnapshotLength(
      withoutVersions.slice(0, index),
      segment,
      digits
    )
    if (snapshotLength > 0) {
      if (hasPhoneInSeparatedSegment(segment.slice(snapshotLength))) return true
    } else if (digits <= 19 || hasPhoneInSeparatedSegment(segment)) {
      return true
    }
  }

  // Preserve short underscore-delimited dial groups without joining whole numbers.
  for (const [groups] of withoutVersions.matchAll(UNDERSCORE_PHONE_GROUPS)) {
    if (hasDialLength(groups)) return true
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
  const digits = countDecimalDigits(phone)
  return digits >= 7 && digits <= 19
}
