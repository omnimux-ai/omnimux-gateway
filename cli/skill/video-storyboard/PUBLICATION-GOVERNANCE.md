# Video Storyboard Publication Governance

## Decision

`canvas/video-storyboard` remains a public business skill because its primary value is lightweight storyboard-table planning and shot-level creative judgment.

It is not treated as a code-execution skill.

## Runtime Boundary

Runtime-readable files:

- `SKILL.md`
- `references/script-styles.json`
- `references/output-contract.md`
- `references/examples.md`

The skill must not require a generated frame, final video, Canvas node, persisted workspace, or model selection result unless the user explicitly asks for that downstream artifact and a visible tool proves it.

## Public Use Conditions

- Use for storyboard tables, script-to-shot planning, reference breakdown, prompt-ready shot notes, revisions, and diagnosis.
- Do not start image/video generation simply because this skill is selected.
- Do not persist Canvas results unless an explicit downstream Canvas tool call succeeds.
- Do not claim artifacts beyond the storyboard table unless a visible tool result exists.
