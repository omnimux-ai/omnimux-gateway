# Social Card Publication Governance

## Decision

`social-card/social-card` remains a public business skill because its primary value is social-card planning, copy, layout judgment, rendering guidance, and delivery QA.

It is not treated as a code-execution skill.

## Runtime Boundary

Core runtime entrypoint:

- `SKILL.md`

Reference files under `references/` provide optional method detail and rendering guidance.

Bundled support files:

- `assets/template-editorial-card.html`
- `assets/template-swiss-card.html`
- `assets/magazine-bg-webgl.js`
- `validate-social-deck.mjs`

These support files are not hidden Agent runtime steps. They may be read or used only when the requested mode requires rendering, QA, or template authoring, and only through an explicit visible tool/command path.

## Public Use Conditions

- Use for social-card planning, copy packs, reference breakdown, visual direction, rendered card sets, template persistence, and diagnosis.
- Do not render PNGs for planning/copy/diagnosis-only requests.
- Do not persist templates unless the user explicitly asks for a reusable template, saved project result, or template-library handoff.
- Do not execute bundled JavaScript simply because the skill is selected.
- Do not claim PNG URLs, template IDs, or validator pass results unless the corresponding tool or command actually produced them.

## Code-Bearing Files

`assets/magazine-bg-webgl.js` is a front-end background asset for generated HTML.

`validate-social-deck.mjs` is a validator support file. It can support QA workflows, but the public skill remains useful without it because non-rendering modes complete through text contracts and rendering modes complete through visible tool results.
