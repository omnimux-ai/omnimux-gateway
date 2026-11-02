# Image Creation Publication Governance

## Decision

`image/image-creation` remains a public business skill because its primary value is visual creative direction, prompt construction, reference-aware planning, generation decisioning, and diagnosis.

It is not treated as a code-execution skill.

## Runtime Boundary

Runtime-readable entrypoints:

- `SKILL.md`
- `references/compliance-rewrite-guide.md`
- `references/ecommerce-poster-prompt-pack.md`
- `references/prompt-dimensions.md`
- `references/prompt-enhancement-guides.md`
- `references/style-gallery.md`

The public package must not depend on a bundled prompt-search script or a third-party prompt corpus. Prompt examples used in responses must come from current user input, explicit references, or visible tool results.

## Public Use Conditions

- Use for prompt-only work, visual briefs, image generation, reference-guided planning, poster/social visuals, variants, and visual diagnosis.
- Do not generate images unless the user explicitly requests an image artifact.
- Do not claim `imageUrl`, task id, provider status, or batch result unless a visible generation tool actually returned it.
- Do not fabricate prompt-gallery matches, source URLs, creator names, or reference images.
- Do not run bundled code as part of normal skill activation.

## Removed From Public Package

The previous prompt-gallery search script and bundled prompt corpus were removed from the public skill package because skill publication collects all files under the skill directory. Keeping third-party prompt data in the published package would make the public skill depend on a non-core corpus and unclear redistribution rights.
