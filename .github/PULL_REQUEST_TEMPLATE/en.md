<!--
Agents should select the template using the Pull requests rules in `AGENTS.md`.
-->
# PR Notice

Chinese template: `.github/PULL_REQUEST_TEMPLATE.md`

> [!IMPORTANT]
>
> - The submitter checks the PR body against the verification evidence and keeps the points maintainers need to re-check. Submit only verified content in the body and later comments. An agent submitting the PR follows the repository's independent review process and records the result below.
> - Do not report issues when pass-through is enabled; pass-through sends content as-is and does not go through new-api processing logic.
> - Please complete this template before submitting.

## Related Issue
- For new features, please fill in the Issue number below. If none exists yet, please create one first. Please discuss the feature in the Issue rather than using the PR in its place.
- For large or directional changes, please reach agreement with maintainers in the linked Issue before opening a PR.
- Bug fixes should link a corresponding Issue. Design trade-offs, misunderstandings, or mismatched expectations are a better fit for a discussion or feature request.

- Closes #

## Type of change
- [ ] Bug fix
- [ ] New feature
- [ ] Performance / Refactor
- [ ] Documentation

## Description
(Briefly describe what changed and why it works. Do not paste unfiltered AI-generated text. If that is hard to summarize, consider splitting the scope or aligning with maintainers in an Issue first.)

## Documentation impact
(Follow the [documentation impact rules](../../.agents/rules/development-workflow.md#文档影响). List changes, updated docs, and evidence, or explain why docs are unaffected. For cross-repository updates, include the related task or PR, its status, and the release verification criteria.)

## Proof of Work
(Record how this was verified: the actual commands or steps and the observed results. Stating only that `go build` or tests passed is not valid proof. For UI changes, include a screenshot or recording. For bug fixes, describe the reproduction and the result after the fix.)

## Review record
(For agent-submitted PRs, record the independently reviewed commit SHA, verdict, verification evidence checked, and gaps in verification.)

## Checklist
- [ ] **Not a duplicate:** I have searched existing [Issues](https://github.com/QuantumNous/new-api/issues) and [PRs](https://github.com/QuantumNous/new-api/pulls) and confirmed this is not a duplicate.
- [ ] **Feature issue:** If this PR is a New feature, I have linked a corresponding Issue; if none existed, I created one first.
- [ ] **Prior discussion:** If this is a large or directional change, I have discussed it with maintainers in the linked Issue and reached agreement.
- [ ] **Scope:** This PR is not a Coding Plan, reverse-engineered channel, third-party API wrapper, or a change to the Codex channel type.
- [ ] **Not pass-through:** This PR is not about forwarding behavior after enabling pass-through; pass-through sends content as-is and does not go through new-api processing logic.
- [ ] **Focused change:** This PR is a single focused change and does not include unrelated code.
- [ ] **Local verification:** I verified the changed path and recorded the commands and observed results. Stating only that `go build` or tests passed is not valid proof.
- [ ] **Security:** This change does not include secrets and follows the project's coding guidelines.
