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
  /(?:e?xt(?:n|ensi[oó]?n|ens[aã]?o)?|poste|int(?:erno)?|ramal|anexo|dob|доб)[\s.:=(),_#-]*(?:n\.?[°o]?[\s.:=(),_#-]*)?(?:\p{Decimal_Number}[\p{Decimal_Number}\s()._-]*#?)?$|[x#~][\s.:=(),_#-]*\p{Decimal_Number}[\p{Decimal_Number}\s()._-]*#?$/iu

const FORMATTED_DATE_LABEL =
  /^(?:19|20)\d{2}([._-])(?:0[1-9]|1[0-2])\1(?:0[1-9]|[12]\d|3[01])(?:[._-][a-z][\w.-]*)?$/i

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
  if (/^tel:/i.test(dialValue)) return true
  if (FORMATTED_DATE_LABEL.test(dialValue)) return false

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
    .replace(PHONE_EXTENSION_SUFFIX, '')
    // An attached x can also survive removal of non-ASCII extension digits.
    .replace(/([\p{Decimal_Number})])x[._-]*$/iu, '$1')
  if (!/^[+\p{Decimal_Number}().\s_-]+(?:[,;][,;\s]*)?$/u.test(phone)) {
    return false
  }
  // Include international dial prefixes within the phone-shaped length range.
  const digits = phone.match(/\p{Decimal_Number}/gu)?.length ?? 0
  return digits >= 7 && digits <= 19
}
