package billing_setting

// Built-in token prices use actual USD per million tokens. Keep new model
// defaults here instead of splitting them across the legacy ratio tables.
var builtinBillingExpr = map[string]string{
	// https://developers.openai.com/api/docs/pricing (Standard, 2026-09-09).
	// The Images API reports image output in output_tokens, normalized to c.
	"gpt-image-2":            `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	"gpt-image-2.5-sunburst": `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	"gpt-image-2.5-flare":    `tier("standard", p * 5 + cr * 1.25 + img * 8 + img_cr * 2 + c * 30)`,
	// https://developers.openai.com/api/docs/models/gpt-6-astra
	// Standard pricing; the long-context rates apply to the whole request.
	// Do not infer service-tier discounts from incoming request parameters:
	// channels filter service_tier by default, so it may not reach the upstream.
	"gpt-6-astra": `len <= 272000 ? tier("standard", p * 10 + c * 50 + cr * 1 + cc * 12.5) : tier("long_context", p * 20 + c * 75 + cr * 2 + cc * 25)`,
	// https://docs.volcengine.com/docs/6561/1359370 (Doubao-Seed-TTS-3.0, 2026-11-25).
	// Post-paid 1.5 CNY / 10k billable chars (150 CNY / 1M chars, ~21 USD / 1M chars);
	// Han ideographs count as 2 chars, other runes as 1 char, reported in prompt tokens (p).
	"seed-tts-3.0":        `tier("standard", p * 21)`,
	"doubao-seed-tts-3.0": `tier("standard", p * 21)`,
	// https://ai.google.dev/gemini-api/docs/pricing (Standard Paid Tier, 2026-11-27).
	// Thinking tokens are included in output tokens (c); context caching is implicit read-only (cr).
	// gemini-4-carbon uses its standard rate ($4/$20/$0.20, effective 2026-12-01; promo $2/$10/$0.10 through 2026-11-30).
	"gemini-4-hydrogen": `tier("standard", p * 1.5 + c * 7.5 + cr * 0.075)`,
	"gemini-4-neon":     `tier("standard", p * 3 + c * 15 + cr * 0.15)`,
	"gemini-4-carbon":   `tier("standard", p * 4 + c * 20 + cr * 0.2)`,
	// https://docs.typesafe.ai/models (TypeSafe Jev, 2026-10-14).
	// Billed per input token ($0.042 / 1M input tokens); output tokens are free.
	// Served by the embedded typesafe task plugin (usageSchema: input_tokens).
	"jev-1.14.0":  `tier("base", u("input_tokens") * 0.042 / 1000000)`,
	"jev-1.13.0":  `tier("base", u("input_tokens") * 0.042 / 1000000)`,
	"jev-latest":  `tier("base", u("input_tokens") * 0.042 / 1000000)`,
	"jev-preview": `tier("base", u("input_tokens") * 0.042 / 1000000)`,
}
