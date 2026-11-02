# OmniMux package skill tree

**One skill = one top-level directory = one Agent-discoverable `name`.**

```text
skill/
├── manifest.json
├── omnimux/                      # shared CLI + secrets
├── recreate-viral-video/
├── slideshow-creation/
├── video-script-creation/
├── video-analysis/
├── video-creation/
├── video-prompt-generation/
├── video-storyboard/
├── video-storyboard-planning/
├── ugc-commerce-video-skill/
├── image-creation/
├── image-recreation/
├── social-card/
├── product-image-set/
├── multi-angle-product-image/
├── a-plus-content/
├── infographic/
└── …
```

## Install

```bash
omnimux skill install --user
omnimux skill install --project
omnimux skill status
omnimux skill update --project
```

Default **link** → each `.agents/skills/<id>` points at package `skill/<id>`.

## Roles

| role | ids |
| --- | --- |
| shared | `omnimux` |
| business (video) | recreate-viral-video, video-script-creation, video-analysis, video-creation, video-prompt-generation, video-storyboard, video-storyboard-planning, ugc-commerce-video-skill |
| business (image/图文) | slideshow-creation, image-creation, image-recreation, social-card, product-image-set, multi-angle-product-image, a-plus-content, infographic |
