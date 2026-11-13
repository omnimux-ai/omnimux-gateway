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
along with this program. If not, see <http://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { resolveDeclared } from './model-id'

/**
 * The billing dimension a public model is sold in.
 *
 * The catalog payload cannot express this: every video model reaches the
 * frontend as `quota_type: 1` with a `model_price` and no `billing_expr`, so a
 * flat charge per invocation and a rate per output second are indistinguishable
 * from the payload alone. `model_price` is the per-invocation price in the first
 * case and the per-second rate in the second.
 */
export type BillingDimension = 'per_call' | 'per_second'

/**
 * The length envelope a per-call model actually accepts.
 *
 * `unknown` is a first-class value: it records that the envelope has not been
 * established, so the form must not offer a chooser over guesses nor submit a
 * value nobody verified.
 */
export type DurationEnvelope =
  | { kind: 'fixed'; seconds: number }
  | { kind: 'selectable'; seconds: number[] }
  | { kind: 'unknown' }

export type BillingDeclaration = {
  dimension: BillingDimension
  /**
   * The billed package's length envelope. Omitted whenever the capability
   * contract already states it, which is the preferred home for it.
   */
  duration?: DurationEnvelope
  /** Accepted output resolutions, when established. */
  resolutions?: string[]
  /** Accepted aspect ratios, when established. */
  ratios?: string[]
}

/**
 * Declared billing facts for public video model ids.
 *
 * Enrolment is how a model stops being guessed at. Absent an entry the form
 * falls back to {@link inferredDimensionFor}, which preserves the historical
 * per-second shape; `scripts/ops/pricing-ledger/` is the evidence source and the
 * drift guard in `__tests__/billing-register-ledger.test.ts` fails when an
 * in-service per-call model is missing here.
 *
 * Scope after the capability contract landed: the contract now supplies
 * everything descriptive — family, aspect ratios, resolutions, output formats,
 * the duration envelope, audio support — so this file must not restate any of
 * it. What remains is the two facts the governance schema cannot express yet:
 *
 * - the **billing dimension** (per call vs per second), which the specs do not
 *   carry at all; and
 * - the **billed package's envelope**, which can be narrower than the vendor
 *   model the spec describes. `minimax-h3-task` is the live case: its spec
 *   correctly records the vendor model's 4–15 s range, while the id is sold as a
 *   fixed-length package. Until the spec schema can say "this id is sold as a
 *   package of N seconds", that one narrowing fact has to live here.
 *
 * A `duration` entry is therefore an exception, not a parallel truth: when the
 * contract already states the package's fixed length, as it now does for
 * `seedance-2-5-task`, the entry here adds nothing and should be dropped.
 */
export const BILLING_DECLARATIONS: Record<string, BillingDeclaration> = {
  // The per-call alias resolves to a *different upstream package* than the
  // per-second baseline, which is why it cannot inherit that model's envelope.
  //
  // Billed line (group `seedance-cheap`, channel 53 → upstream
  // `seedance2.5-9图`): fixed 30 seconds, 720p, 16:9 / 9:16, at most 9
  // reference images, no video or audio references. Three sources agree on the
  // fixed length: the channel's published model table ("固定 30 秒",
  // https://www.weijinapi.top/docs/), its pricing description ("卡人脸只支持30秒")
  // and the pricing ledger ("固定 30 秒"). The resolution and ratio values are
  // that same published table, which lists one resolution per model.
  //
  // Open conflict: `docs/model-governance/specs/seedance-2-5-task.json`
  // records the *vendor model's* general envelope instead — durations 4–30,
  // resolutions 480p / 720p / 1080p, seven aspect ratios — and its own routing
  // note points at the same 9-image line. That file describes what the model can
  // do, not what this billed package accepts, so the package values are used
  // here; the two have not been reconciled. The ledger's resolution field
  // ("以接口返回为准") is likewise unreconciled. Narrowing is the safe
  // direction: offering a resolution the package rejects fails the generation,
  // while withholding one it accepts only costs an option.
  'seedance-2-5-task': {
    dimension: 'per_call',
    // No envelope here any more: `d8a6990b7` corrected the governance spec to
    // the billed package, so the contract states the fixed 30 s, 720p and the
    // two ratios. The register supplies an envelope only where the contract is
    // silent — `minimax-h3-task` below is the remaining case.
    resolutions: ['720p'],
    ratios: ['16:9', '9:16'],
  },
  // In service as per-call; the fixed length is not established by any source
  // read here. Our own catalog description calls it "fixed-duration" without a
  // number, and this id is served by two channels whose upstream packages need
  // not agree, so no length is asserted.
  'seedance-2-0-task': {
    dimension: 'per_call',
    duration: { kind: 'unknown' },
  },
  // Same shape: the catalog says "fixed-duration", the serving channel's
  // upstream tag says 10 or 15 seconds. Those disagree on *which* length, so
  // neither is asserted.
  'seedance-2-0-fast-task': {
    dimension: 'per_call',
    duration: { kind: 'unknown' },
  },
  // 15 seconds, from two agreeing sources: our catalog description ("MiniMax H3
  // Video 15s fixed-duration task video generation.") and the upstream package
  // this id is served by.
  'minimax-h3-task': {
    dimension: 'per_call',
    duration: { kind: 'fixed', seconds: 15 },
  },
  // Catalog description states a per-call fixed package but names no length.
  'gemini-omni-1.1-flash-task': {
    dimension: 'per_call',
    duration: { kind: 'unknown' },
  },
  'grok-imagine-video-1-5-task': {
    dimension: 'per_call',
    duration: { kind: 'unknown' },
  },
}

/**
 * Ledger ids whose recorded billing mode disagrees with itself.
 *
 * `scripts/ops/pricing-ledger/models/*.json` files `veo-3.1` and
 * `veo-3.1-fast` under both `per_call` and `per_second`. Neither is enrolled
 * above, so both keep the historical per-second shape. The conflict is recorded
 * rather than averaged away; drop an id from this set once the ledger is
 * reconciled.
 */
export const LEDGER_BILLING_CONFLICTS: ReadonlySet<string> = new Set([
  'veo-3.1',
  'veo-3.1-fast',
])

/**
 * The repo's own rule for a per-call id (root AGENTS.md, "Pricing & Single
 * Model ID"): per-call video is one public id per capability, spelled with the
 * `-task` suffix, while the per-second baseline keeps the bare id.
 */
const PER_CALL_ID_SUFFIX = /-task$/

/**
 * Billing dimension for a model with no enrolled declaration.
 *
 * `-task` ids are per-call by the repo rule above; everything else keeps the
 * per-second shape that predates this register, so an unenrolled per-second
 * model is never regressed into losing its duration control.
 */
export function inferredDimensionFor(modelId: string): BillingDimension {
  const normalised = modelId.trim().toLowerCase().replaceAll('.', '-')
  return PER_CALL_ID_SUFFIX.test(normalised) ? 'per_call' : 'per_second'
}

/** The enrolled declaration for a model, or undefined when it is not enrolled. */
export function billingDeclarationFor(
  modelId: string
): BillingDeclaration | undefined {
  return resolveDeclared(modelId, BILLING_DECLARATIONS)
}

/** Enrolled dimension when there is one, the inferred default otherwise. */
export function billingDimensionFor(modelId: string): BillingDimension {
  return (
    billingDeclarationFor(modelId)?.dimension ?? inferredDimensionFor(modelId)
  )
}

/** True when the model charges a flat price per invocation. */
export function isPerCall(modelId: string): boolean {
  return billingDimensionFor(modelId) === 'per_call'
}

/** Enrolled length envelope, or undefined when the model is not enrolled. */
export function durationEnvelopeFor(
  modelId: string
): DurationEnvelope | undefined {
  return billingDeclarationFor(modelId)?.duration
}
