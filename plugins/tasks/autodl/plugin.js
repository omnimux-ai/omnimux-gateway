// AutoDL.Art ComfyUI workflow plugin (Task Plugin API v1).
//
// Three public model IDs converge 14 upstream ComfyUI workflow endpoints:
//   minimax-h3-video  MiniMax H3 video (text / image / first-last frame /
//                     multi-image + multi-audio / prompt-less lip sync)
//   wan-2-2-animate   Wan 2.2 Animate motion transfer (reference video)
//   indextts-2        IndexTTS-2 speech synthesis and voice cloning (per call)
//
// Endpoint selection has three inputs, in this order:
//   1. ctx.upstreamModel — the channel model_mapping result. It carries either
//      a tier key (standard | fast | pro, see CAP.tiers) or a concrete
//      workflowId, which pins that endpoint outright (operator escape hatch).
//      Without a mapping it equals ctx.model and the default tier applies.
//   2. The request shape, derived from which media slots the caller filled.
//   3. Capability trimming (shape, resolution, duration ceiling) and cost
//      ordering: cheapest upstream cost first, ties broken by workflowId.
// No candidate means the hook throws a user-readable capability error. The
// plugin never silently falls back to another workflow or another tier.
//
// The capability matrix below is generated from
// scripts/ops/pricing-ledger/official-autodl-prices.json by
// scripts/ops/pricing-ledger/gen-autodl-cap.js. The sandbox has no imports, so
// the generator writes this copy and plugins/autodl_plugin_test.go fails if it
// drifts from plugins/tasks/autodl/cap.gen.js.

export const meta = {
  apiVersion: 1,
  key: "autodl",
  name: "AutoDL ComfyUI",
  description: {
    en: "AutoDL.Art ComfyUI workflows: MiniMax H3 video, Wan 2.2 Animate motion transfer, and IndexTTS-2 speech synthesis",
    zh: "AutoDL.Art ComfyUI 工作流：MiniMax H3 视频、Wan 2.2 Animate 动作迁移、IndexTTS-2 语音合成",
  },
  version: "1.0.0",
  author: { name: "OmniMux" },
  website: "https://autodl.art",
  baseUrl: "https://autodl.art",
  auth: "api_key",
  models: ["minimax-h3-video", "wan-2-2-animate", "indextts-2"],
  fetchMode: "per_task",
  usageSchema: {
    // Requested video duration in seconds; H3 workflows accept 1-15, the
    // 12-second variant caps at 12.
    seconds: {
      type: "number",
      unit: "second",
      description: { en: "Video generation unit price", zh: "视频生成单价" },
    },
    // Requested output video resolution tier. 1088p/1440p are offered on the six
    // workflows that accept them (cost 0.06-0.09 CNY/s vs 0.04-0.05). Per-second
    // billing alone has no resolution dimension, so those tiers are priced by
    // resolution in the task billing expression (u("resolution")) -- they must
    // never be sold at the flat 480p/768p rate. 1080p (three workflows,
    // 0.10-0.11 CNY/s) stays unoffered and fails closed.
    resolution: {
      enum: ["480p", "736p", "768p", "1088p", "1440p"],
      enumLabels: {
        "480p": { en: "480p", zh: "480p" },
        "736p": { en: "736p", zh: "736p" },
        "768p": { en: "768p", zh: "768p" },
        "1088p": { en: "1088p", zh: "1088p" },
        "1440p": { en: "1440p", zh: "1440p" },
      },
      description: { en: "Output video resolution", zh: "输出视频分辨率" },
    },
  },
  usageExamples: [
    { label: "768p 5s", facts: { seconds: 5, resolution: "768p" } },
    { label: "480p 10s", facts: { seconds: 10, resolution: "480p" } },
    { label: "736p 15s", facts: { seconds: 15, resolution: "736p" } },
    { label: "1088p 5s", facts: { seconds: 5, resolution: "1088p" } },
    { label: "1440p 5s", facts: { seconds: 5, resolution: "1440p" } },
  ],
  routes: [
    {
      method: "POST",
      path: "/autodl/comfyui/workflow",
      type: "submit",
      decode: "createTask",
      render: "taskCreated",
      models: ["minimax-h3-video", "wan-2-2-animate", "indextts-2"],
    },
    { method: "GET", path: "/autodl/comfyui/workflow/:task_id", type: "query", render: "taskStatus" },
  ],
  protocols: [
    { name: "openai_responses", supports: ["stream", "sync", "background"] },
    { name: "openai_video", models: ["minimax-h3-video", "wan-2-2-animate"] },
  ],
};

/* BEGIN autodl-cap (generated) */
const CAP = {
  "version": 1,
  "source": "scripts/ops/pricing-ledger/official-autodl-prices.json",
  "endpointCount": 20,
  "publicIds": ["indextts-2", "minimax-h3-video", "wan-2-2-animate"],
  "defaultTier": "standard",
  "tiers": {
    "standard": ["indextts2-v1", "minimax_h3_801", "minimax_h3_802", "minimax_h3_803", "minimax_h3_b99_001", "minimax_h3_b99_002", "minimax_h3_b99_003_12s", "minimax_h3_image_audio_to_video", "minimax_h3_image_audio_to_video_v2", "minimax_h3_image_audio_to_video_v2_15s", "minimax_h3_lightx2v", "minimax_h3_lightx2v_no_pic", "minimax_h3_lightx2v_v5", "minimax_h3_lightx2v_v5_15s", "minimax_h3_z0901", "minimax_h3_z0902", "minimax_h3_z0903", "wan2.2animate-v4-motion_retargeting"],
    "fast": ["minimax_h3_zm_u08"],
    "pro": ["minimax_h3_zm_u24"]
  },
  "workflows": {
    "indextts2-v1": {
      "workflowId": "indextts2-v1",
      "publicId": "indextts-2",
      "tier": "standard",
      "billingMode": "unknown",
      "shapes": ["text-to-speech"],
      "resolutions": [],
      "resolutionOptions": {},
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": null,
      "durationField": null,
      "promptField": "prompt_text",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["emo_afraid", "emo_angry", "emo_calm", "emo_control_method", "emo_disgusted", "emo_happy", "emo_melancholic", "emo_random", "emo_ref_audio", "emo_sad", "emo_surprised", "prompt_simple", "prompt_text"],
      "requiredFields": ["prompt_simple", "prompt_text"],
      "usageCount7d": 21738,
      "unitCostCnyPerSec": {
        "default": 0.001
      },
      "unitCostCnyPerSecOffPeak": {
        "default": 0.001
      }
    },
    "minimax_h3_801": {
      "workflowId": "minimax_h3_801",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1376)", "768p横(1376*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "mode", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 107,
      "unitCostCnyPerSec": {
        "480p": 0.05,
        "768p": 0.06,
        "1088p": 0.07,
        "1440p": 0.08
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.04,
        "768p": 0.05,
        "1088p": 0.06,
        "1440p": 0.07
      }
    },
    "minimax_h3_802": {
      "workflowId": "minimax_h3_802",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1376)", "768p横(1376*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "mode", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 786,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05,
        "1088p": 0.06,
        "1440p": 0.07
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04,
        "1088p": 0.05,
        "1440p": 0.06
      }
    },
    "minimax_h3_803": {
      "workflowId": "minimax_h3_803",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-audio", "image-reference"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1376)", "768p横(1376*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "mode", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 515,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05,
        "1088p": 0.06,
        "1440p": 0.07
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04,
        "1088p": 0.05,
        "1440p": 0.06
      }
    },
    "minimax_h3_b99_001": {
      "workflowId": "minimax_h3_b99_001",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["text-to-video"],
      "resolutions": ["736p"],
      "resolutionOptions": {
        "736p": ["736p竖", "736p横", "736p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 771,
      "unitCostCnyPerSec": {
        "736p": 0.06
      },
      "unitCostCnyPerSecOffPeak": {
        "736p": 0.04
      }
    },
    "minimax_h3_b99_002": {
      "workflowId": "minimax_h3_b99_002",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["first-last-frame"],
      "resolutions": ["736p"],
      "resolutionOptions": {
        "736p": ["736p竖", "736p横", "736p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": "first_frame",
      "lastFrameField": "last_frame",
      "videoField": null,
      "fields": ["duration", "first_frame", "last_frame", "prompt", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 959,
      "unitCostCnyPerSec": {
        "736p": 0.06
      },
      "unitCostCnyPerSecOffPeak": {
        "736p": 0.04
      }
    },
    "minimax_h3_b99_003_12s": {
      "workflowId": "minimax_h3_b99_003_12s",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["736p"],
      "resolutionOptions": {
        "736p": ["736p竖", "736p横", "736p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 12,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 7537,
      "unitCostCnyPerSec": {
        "736p": 0.06
      },
      "unitCostCnyPerSecOffPeak": {
        "736p": 0.04
      }
    },
    "minimax_h3_image_audio_to_video": {
      "workflowId": "minimax_h3_image_audio_to_video",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["lip-sync"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横"],
        "768p": ["768p竖", "768p横"]
      },
      "nativeResolutions": [],
      "excludedResolutions": ["1080p"],
      "maxDuration": 15,
      "durationField": "audio_duration",
      "promptField": null,
      "imageFields": ["ref_image_0"],
      "audioFields": ["ref_audio_0"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["audio_duration", "ref_audio_0", "ref_image_0", "resolution"],
      "requiredFields": ["ref_audio_0", "ref_image_0"],
      "usageCount7d": 3113,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_image_audio_to_video_v2": {
      "workflowId": "minimax_h3_image_audio_to_video_v2",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-audio", "image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横"],
        "768p": ["768p竖", "768p横"]
      },
      "nativeResolutions": [],
      "excludedResolutions": ["1080p"],
      "maxDuration": 10,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 11541,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_image_audio_to_video_v2_15s": {
      "workflowId": "minimax_h3_image_audio_to_video_v2_15s",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-audio", "image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横"],
        "768p": ["768p竖", "768p横"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 102721,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_lightx2v": {
      "workflowId": "minimax_h3_lightx2v",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["first-last-frame"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横", "480p(1:1)"],
        "768p": ["768p竖", "768p横", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": "first_frame",
      "lastFrameField": "last_frame",
      "videoField": null,
      "fields": ["duration", "first_frame", "last_frame", "prompt", "resolution"],
      "requiredFields": ["first_frame", "last_frame", "prompt"],
      "usageCount7d": 4763,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_lightx2v_no_pic": {
      "workflowId": "minimax_h3_lightx2v_no_pic",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["text-to-video"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横", "480p(1:1)"],
        "768p": ["768p竖", "768p横", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "resolution"],
      "requiredFields": ["prompt"],
      "usageCount7d": 17118,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_lightx2v_v5": {
      "workflowId": "minimax_h3_lightx2v_v5",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横", "480p(1:1)"],
        "768p": ["768p竖", "768p横", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": ["1080p"],
      "maxDuration": 10,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 60828,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_lightx2v_v5_15s": {
      "workflowId": "minimax_h3_lightx2v_v5_15s",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p竖", "480p横", "480p(1:1)"],
        "768p": ["768p竖", "768p横", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 98286,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_z0901": {
      "workflowId": "minimax_h3_z0901",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["text-to-video"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1344)", "768p横(1344*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": [],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 442,
      "unitCostCnyPerSec": {
        "480p": 0.05,
        "768p": 0.08,
        "1088p": 0.08,
        "1440p": 0.09
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.04,
        "768p": 0.06,
        "1088p": 0.06,
        "1440p": 0.07
      }
    },
    "minimax_h3_z0902": {
      "workflowId": "minimax_h3_z0902",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-reference"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1376)", "768p横(1376*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5"],
      "audioFields": [],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 1131,
      "unitCostCnyPerSec": {
        "480p": 0.05,
        "768p": 0.08,
        "1088p": 0.08,
        "1440p": 0.09
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.04,
        "768p": 0.06,
        "1088p": 0.06,
        "1440p": 0.07
      }
    },
    "minimax_h3_z0903": {
      "workflowId": "minimax_h3_z0903",
      "publicId": "minimax-h3-video",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["image-audio"],
      "resolutions": ["480p", "768p", "1088p", "1440p"],
      "resolutionOptions": {
        "480p": ["480p竖(480*864)", "480p横(864*480)"],
        "768p": ["768p竖(768*1376)", "768p横(1376*768)"],
        "1088p": ["1088p竖(1088*1920)", "1088p横(1920*1088)"],
        "1440p": ["1440p竖(1440*2560)", "1440p横(2560*1440)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 1166,
      "unitCostCnyPerSec": {
        "480p": 0.05,
        "768p": 0.08,
        "1088p": 0.08,
        "1440p": 0.09
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.04,
        "768p": 0.06,
        "1088p": 0.06,
        "1440p": 0.07
      }
    },
    "minimax_h3_zm_u08": {
      "workflowId": "minimax_h3_zm_u08",
      "publicId": "minimax-h3-video",
      "tier": "fast",
      "billingMode": "per_second",
      "shapes": ["image-audio", "image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p横", "480p竖", "480p(1:1)"],
        "768p": ["768p横", "768p竖", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 14427,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "minimax_h3_zm_u24": {
      "workflowId": "minimax_h3_zm_u24",
      "publicId": "minimax-h3-video",
      "tier": "pro",
      "billingMode": "per_second",
      "shapes": ["image-audio", "image-reference"],
      "resolutions": ["480p", "768p"],
      "resolutionOptions": {
        "480p": ["480p横", "480p竖", "480p(1:1)"],
        "768p": ["768p横", "768p竖", "768p(1:1)"]
      },
      "nativeResolutions": [],
      "excludedResolutions": [],
      "maxDuration": 15,
      "durationField": "duration",
      "promptField": "prompt",
      "imageFields": ["ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8"],
      "audioFields": ["ref_audio_0", "ref_audio_1", "ref_audio_2"],
      "imageField": null,
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": null,
      "fields": ["duration", "prompt", "ref_audio_0", "ref_audio_1", "ref_audio_2", "ref_image_0", "ref_image_1", "ref_image_2", "ref_image_3", "ref_image_4", "ref_image_5", "ref_image_6", "ref_image_7", "ref_image_8", "resolution", "seed"],
      "requiredFields": ["prompt"],
      "usageCount7d": 78691,
      "unitCostCnyPerSec": {
        "480p": 0.04,
        "768p": 0.05
      },
      "unitCostCnyPerSecOffPeak": {
        "480p": 0.03,
        "768p": 0.04
      }
    },
    "wan2.2animate-v4-motion_retargeting": {
      "workflowId": "wan2.2animate-v4-motion_retargeting",
      "publicId": "wan-2-2-animate",
      "tier": "standard",
      "billingMode": "per_second",
      "shapes": ["motion"],
      "resolutions": [],
      "resolutionOptions": {},
      "nativeResolutions": ["464*832px(竖版)", "832*464px(横版)"],
      "excludedResolutions": [],
      "maxDuration": null,
      "durationField": null,
      "promptField": null,
      "imageFields": [],
      "audioFields": [],
      "imageField": "ref_image",
      "firstFrameField": null,
      "lastFrameField": null,
      "videoField": "ref_video",
      "fields": ["ref_image", "ref_video", "resolution", "seed"],
      "requiredFields": ["ref_image", "ref_video"],
      "usageCount7d": 1989,
      "unitCostCnyPerSec": {
        "default": 0.04
      },
      "unitCostCnyPerSecOffPeak": {
        "default": 0.03
      }
    }
  }
};
/* END autodl-cap (generated) */

const SUBMIT_PATH_PREFIX = "/api/v1/comfyui/comfyui_workflow/";
const RESULT_PATH_PREFIX = "/api/v1/comfyui/comfyui_workflow/result/";
const DEFAULT_DURATION_SECONDS = 5;
const MAX_DURATION_SECONDS = 3600;
const RESOLUTION_TIER_PATTERN = /^([0-9]{3,4})p/;
// Mirror of OFFERED_RESOLUTIONS in scripts/ops/pricing-ledger/gen-autodl-cap.js:
// the sandbox has no imports, so the runtime copy is hand-written. A drift is
// caught by the 1080p rejection fixture, whose message echoes this list.
const OFFERED_RESOLUTIONS = ["480p", "736p", "768p", "1088p", "1440p"];
const VERTICAL_MARKER = "竖";

// Task actions stay in the repo's snake_case action vocabulary. The request
// shape is the routing concept; the action only labels the persisted task.
const ACTION_BY_SHAPE = {
  "text-to-video": "text_to_video",
  "image-reference": "image_to_video",
  "first-last-frame": "first_last_frame",
  "image-audio": "image_audio",
  "lip-sync": "lip_sync",
  motion: "motion_transfer",
  "text-to-speech": "text_to_speech",
};

// Public request slots. Everything outside this set that the target workflow
// declares as an input passes through verbatim (IndexTTS-2 emotion vectors,
// for example).
const RESERVED_SLOTS = [
  "model",
  "metadata",
  "prompt",
  "prompt_text",
  "input",
  "duration",
  "seconds",
  "audio_duration",
  "resolution",
  "size",
  "seed",
  "images",
  "image",
  "ref_image",
  "audios",
  "audio",
  "ref_audio",
  "video",
  "ref_video",
  "first_frame",
  "first_frame_image",
  "last_frame",
  "last_frame_image",
  "input_reference",
];

function trimmed(value) {
  return String(value === undefined || value === null ? "" : value).trim();
}

function isPlainObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function firstPresent(sources, names) {
  for (const name of names) {
    for (const source of sources) {
      if (!isPlainObject(source)) continue;
      const value = source[name];
      if (value === undefined || value === null || value === "") continue;
      return value;
    }
  }
  return undefined;
}

function pushMedia(target, value) {
  const values = Array.isArray(value) ? value : [value];
  for (const item of values) {
    if (item === undefined || item === null || item === "") continue;
    if (isPlainObject(item)) {
      target.push(item);
      continue;
    }
    const text = trimmed(item);
    if (text) target.push(text);
  }
}

function collectSlots(source, prefix, target) {
  if (!isPlainObject(source)) return;
  const names = [];
  for (const name of Object.keys(source)) {
    if (name.indexOf(prefix) !== 0) continue;
    if (!/^[0-9]+$/.test(name.slice(prefix.length))) continue;
    names.push(name);
  }
  names.sort(function (left, right) {
    return Number(left.slice(prefix.length)) - Number(right.slice(prefix.length));
  });
  for (const name of names) pushMedia(target, source[name]);
}

function normalizeRequest(req) {
  const metadata = isPlainObject(req.metadata) ? req.metadata : {};
  const sources = [req, metadata];
  const images = [];
  const audios = [];
  collectSlots(req, "ref_image_", images);
  collectSlots(req, "ref_audio_", audios);
  collectSlots(metadata, "ref_image_", images);
  collectSlots(metadata, "ref_audio_", audios);
  pushMedia(images, firstPresent(sources, ["ref_image", "image"]));
  pushMedia(images, firstPresent(sources, ["images"]));
  pushMedia(audios, firstPresent(sources, ["ref_audio", "audio"]));
  pushMedia(audios, firstPresent(sources, ["audios"]));
  const firstFrame = firstPresent(sources, ["first_frame", "first_frame_image"]);
  const lastFrame = firstPresent(sources, ["last_frame", "last_frame_image"]);
  if (firstFrame !== undefined) pushMedia(images, firstFrame);
  return {
    prompt: trimmed(firstPresent(sources, ["prompt", "prompt_text", "input"])),
    promptText: firstPresent(sources, ["prompt_text"]) !== undefined,
    duration: firstPresent(sources, ["duration", "seconds", "audio_duration"]),
    resolution: trimmed(firstPresent(sources, ["resolution", "size"])),
    seed: firstPresent(sources, ["seed"]),
    images: images,
    audios: audios,
    firstFrame: firstFrame,
    lastFrame: lastFrame,
    refVideo: firstPresent(sources, ["ref_video", "video"]),
    sources: sources,
  };
}

// Request shapes, highest priority first. Matching is exact: an endpoint only
// competes for the shape its declared inputs actually serve.
function detectShape(norm, publicId) {
  if (publicId === "indextts-2" || norm.promptText) return "text-to-speech";
  if (norm.refVideo !== undefined) return "motion";
  if (norm.firstFrame !== undefined && norm.lastFrame !== undefined) return "first-last-frame";
  if (norm.audios.length && norm.images.length && !norm.prompt) return "lip-sync";
  if (norm.audios.length) return "image-audio";
  if (norm.images.length) return "image-reference";
  return "text-to-video";
}

function resolutionTier(value) {
  const match = RESOLUTION_TIER_PATTERN.exec(trimmed(value).toLowerCase());
  return match ? match[1] + "p" : "";
}

function isOfferedTier(tier) {
  return OFFERED_RESOLUTIONS.indexOf(tier) >= 0;
}

function isKnownPublicId(value) {
  return CAP.publicIds.indexOf(trimmed(value)) >= 0;
}

function isKnownTier(value) {
  return Object.prototype.hasOwnProperty.call(CAP.tiers, trimmed(value));
}

function workflowsForTier(tier) {
  const ids = (CAP.tiers && CAP.tiers[tier]) || [];
  const records = [];
  for (const id of ids) {
    if (CAP.workflows[id]) records.push(CAP.workflows[id]);
  }
  return records;
}

function poolFor(publicId, tier) {
  return workflowsForTier(tier).filter(function (endpoint) {
    return !publicId || endpoint.publicId === publicId;
  });
}

function endpointField(endpoint, name) {
  return name && endpoint.fields.indexOf(name) >= 0;
}

// A request may name the public tier ("768p"), an explicit upstream variant
// ("768p横"), or — for workflows without quality tiers — an upstream-native
// value ("464*832px(竖版)").
function chooseResolution(endpoint, requested) {
  const options = endpoint.resolutionOptions || {};
  if (!requested) {
    const tiers = endpoint.resolutions || [];
    if (!tiers.length) return { option: null, tier: "" };
    const preferred = tiers.indexOf("768p") >= 0 ? "768p" : tiers[0];
    const candidates = options[preferred] || [];
    const chosen = candidates.find(function (value) {
      return value.indexOf(VERTICAL_MARKER) >= 0;
    }) || candidates[0] || null;
    return { option: chosen, tier: chosen ? resolutionTier(chosen) : "" };
  }
  const tier = resolutionTier(requested);
  if (!isOfferedTier(tier)) {
    // Native-only values (Wan 2.2 orientation) carry no billing tier.
    if (!tier && (endpoint.nativeResolutions || []).indexOf(requested) >= 0) return { option: requested, tier: "" };
    return { option: null, tier: "" };
  }
  const candidates = options[tier] || [];
  const chosen = candidates.indexOf(requested) >= 0
    ? requested
    : candidates.find(function (value) {
        return value.indexOf(VERTICAL_MARKER) >= 0;
      }) || candidates[0] || null;
  return { option: chosen, tier: chosen ? resolutionTier(chosen) : "" };
}

function resolutionAccepted(endpoint, requested) {
  if (!requested) return true;
  // Workflows without quality tiers (motion transfer, speech) accept any
  // caller value and ignore it; they bill a single upstream rate.
  if (!(endpoint.resolutions || []).length) return true;
  const tier = resolutionTier(requested);
  if (!isOfferedTier(tier)) return false;
  return (endpoint.resolutions || []).indexOf(tier) >= 0;
}

function durationLimit(endpoint) {
  return typeof endpoint.maxDuration === "number" ? endpoint.maxDuration : MAX_DURATION_SECONDS;
}

// Ranking tier for one endpoint: the requested public tier when the caller
// named one, otherwise the endpoint's own default (768p is the ledger's price
// anchor, so it wins whenever the workflow offers it).
function costTierFor(endpoint, requested) {
  const requestedTier = resolutionTier(requested);
  if (isOfferedTier(requestedTier)) return requestedTier;
  const tiers = endpoint.resolutions || [];
  if (!tiers.length) return "default";
  return tiers.indexOf("768p") >= 0 ? "768p" : tiers[0];
}

function workflowCost(endpoint, tier) {
  if (endpoint.billingMode === "per_call") {
    return typeof endpoint.unitCostCnyPerCall === "number" ? endpoint.unitCostCnyPerCall : Infinity;
  }
  const costs = endpoint.unitCostCnyPerSec || {};
  const value = costs[tier] !== undefined ? costs[tier] : costs.default;
  return typeof value === "number" ? value : Infinity;
}

function capabilitySummary(pool) {
  const byShape = {};
  for (const endpoint of pool) {
    const resolutions = (endpoint.resolutions || []).length ? endpoint.resolutions.join("/") : "any resolution";
    const ceiling = endpoint.maxDuration ? " ≤" + endpoint.maxDuration + "s" : "";
    const label = resolutions + ceiling;
    for (const shape of endpoint.shapes) {
      if (!byShape[shape]) byShape[shape] = [];
      if (byShape[shape].indexOf(label) < 0) byShape[shape].push(label);
    }
  }
  const shapes = Object.keys(byShape).sort();
  return shapes
    .map(function (shape) {
      return shape + " " + byShape[shape].join(", ");
    })
    .join("; ");
}

function capabilityError(scope, shape, norm, duration, pool) {
  const requested = [];
  if (norm.resolution) requested.push("resolution " + norm.resolution);
  requested.push("duration " + duration + "s");
  return new Error(
    "autodl: no workflow in " + scope + " supports shape " + shape + " with " + requested.join(" and ") +
      " (uploaded media: " + norm.images.length + " image(s), " + norm.audios.length + " audio(s)" +
      (norm.refVideo !== undefined ? ", 1 video" : "") + "); supported: " + (capabilitySummary(pool) || "no shape"),
  );
}

function candidatePool(ctx) {
  const upstream = trimmed(ctx && ctx.upstreamModel);
  const model = trimmed(ctx && ctx.model);
  const pinned = CAP.workflows[upstream];
  if (pinned) return { pool: [pinned], tier: pinned.tier, publicId: pinned.publicId, pinned: true };
  const tier = isKnownTier(upstream) ? upstream : CAP.defaultTier;
  const publicId = isKnownPublicId(model) ? model : isKnownPublicId(upstream) ? upstream : "";
  return { pool: poolFor(publicId, tier), tier: tier, publicId: publicId, pinned: false };
}

function requestDuration(norm, publicId, shape) {
  const raw = norm.duration;
  if (raw !== undefined && raw !== null && raw !== "") {
    const value = Number(raw);
    if (!Number.isInteger(value) || value < 1 || value > MAX_DURATION_SECONDS) {
      throw new Error("autodl: duration must be an integer number of seconds between 1 and " + MAX_DURATION_SECONDS);
    }
    return value;
  }
  if (publicId === "indextts-2" || shape === "text-to-speech") {
    const text = String(norm.promptText || norm.prompt || "");
    return Math.max(10, Math.ceil(text.length / 4));
  }
  return DEFAULT_DURATION_SECONDS;
}

function rejectUnsupportedResolution(norm) {
  const tier = resolutionTier(norm.resolution);
  if (!tier || isOfferedTier(tier)) return;
  const offered = [];
  for (const endpoint of Object.keys(CAP.workflows).map(function (id) {
    return CAP.workflows[id];
  })) {
    for (const excluded of endpoint.excludedResolutions || []) {
      if (excluded === tier && offered.indexOf(tier) < 0) offered.push(tier);
    }
  }
  throw new Error(
    "autodl: resolution " + norm.resolution + " is not offered; supported resolutions are " +
      OFFERED_RESOLUTIONS.join(", ") +
      (offered.length ? " (" + tier + " exists upstream but has no published price)" : ""),
  );
}

function selectEndpoint(ctx, norm) {
  rejectUnsupportedResolution(norm);
  const scope = candidatePool(ctx);
  const shape = detectShape(norm, scope.publicId);
  const duration = requestDuration(norm, scope.publicId, shape);
  const matching = scope.pool.filter(function (endpoint) {
    if (endpoint.shapes.indexOf(shape) < 0) return false;
    if (!resolutionAccepted(endpoint, norm.resolution)) return false;
    return duration <= durationLimit(endpoint);
  });
  if (!matching.length) {
    const where = scope.pinned
      ? "pinned workflow " + scope.pool[0].workflowId
      : "tier \"" + scope.tier + "\" of model \"" + (scope.publicId || trimmed(ctx.model) || "any") + "\"";
    throw capabilityError(where, shape, norm, duration, scope.pool);
  }
  matching.sort(function (left, right) {
    const leftCost = workflowCost(left, costTierFor(left, norm.resolution));
    const rightCost = workflowCost(right, costTierFor(right, norm.resolution));
    if (leftCost !== rightCost) return leftCost - rightCost;
    return left.workflowId < right.workflowId ? -1 : left.workflowId > right.workflowId ? 1 : 0;
  });
  const endpoint = matching[0];
  const effective = chooseResolution(endpoint, norm.resolution);
  return { endpoint: endpoint, shape: shape, duration: duration, resolution: effective, tier: scope.tier };
}

function isReservedSlot(key) {
  if (RESERVED_SLOTS.indexOf(key) >= 0) return true;
  return key.indexOf("ref_image_") === 0 || key.indexOf("ref_audio_") === 0;
}

function scalarValue(value) {
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value === "object" && !Array.isArray(value)) return value;
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") return value;
  return undefined;
}

function buildBody(endpoint, norm, selected) {
  const body = {};
  function put(name, value) {
    if (!name || !endpointField(endpoint, name)) return;
    const normalised = scalarValue(value);
    if (normalised === undefined) return;
    if (body[name] !== undefined) return;
    body[name] = normalised;
  }
  // Caller-supplied vendor-only fields first (IndexTTS-2 emotion vectors and
  // the like). Only names this workflow declares survive.
  for (const source of norm.sources) {
    if (!isPlainObject(source)) continue;
    for (const key of Object.keys(source)) {
      if (isReservedSlot(key)) continue;
      put(key, source[key]);
    }
  }
  put(endpoint.promptField, norm.prompt);
  put(endpoint.durationField, selected.duration);
  put("resolution", selected.resolution.option);
  for (let index = 0; index < endpoint.imageFields.length; index += 1) put(endpoint.imageFields[index], norm.images[index]);
  for (let index = 0; index < endpoint.audioFields.length; index += 1) put(endpoint.audioFields[index], norm.audios[index]);
  put(endpoint.imageField, norm.images[0]);
  put(endpoint.firstFrameField, norm.firstFrame);
  put(endpoint.lastFrameField, norm.lastFrame);
  put(endpoint.videoField, norm.refVideo);
  put("seed", norm.seed);
  // The voice-clone reference of IndexTTS-2 is the first uploaded audio.
  put("prompt_simple", norm.audios[0]);
  const missing = endpoint.requiredFields.filter(function (name) {
    return body[name] === undefined;
  });
  if (missing.length) {
    throw new Error("autodl: workflow " + endpoint.workflowId + " requires " + missing.join(", "));
  }
  return body;
}

function resolveSubmission(ctx, requestBody) {
  const req = isPlainObject(requestBody) ? requestBody : {};
  const norm = normalizeRequest(req);
  const selected = selectEndpoint(ctx, norm);
  return {
    endpoint: selected.endpoint,
    shape: selected.shape,
    duration: selected.duration,
    resolution: selected.resolution,
    tier: selected.tier,
    body: buildBody(selected.endpoint, norm, selected),
  };
}

function endpointURL(baseUrl, path) {
  return String(baseUrl || "").replace(/\/+$/, "") + path;
}

// AutoDL authenticates with the raw channel token in Authorization
// (docs/ops/autodl-comfyui-api-survey-zh.md §2.1). authHeader already carries
// the channel-appropriate value for api_key/none auth.
function authorizationValue(ctx) {
  const header = trimmed(ctx && ctx.authHeader);
  if (header) return header;
  return trimmed(ctx && ctx.apiKey);
}

function jsonHeaders(ctx) {
  return {
    "Content-Type": "application/json",
    Accept: "application/json",
    Authorization: authorizationValue(ctx),
  };
}

function payload(body) {
  if (!isPlainObject(body)) return {};
  return isPlainObject(body.data) ? body.data : body;
}

function envelopeCode(body) {
  if (!isPlainObject(body)) return 0;
  const names = ["code", "status", "err_code", "error_code"];
  for (const name of names) {
    const value = body[name];
    if (typeof value === "number" && Number.isFinite(value)) return value;
    if (typeof value === "string" && /^-?[0-9]+$/.test(trimmed(value))) return Number(value);
  }
  return 0;
}

// The upstream envelope is {code|status: <number>, msg?: string, data: {...}}.
// 0 and the HTTP success codes mean "no envelope error".
function envelopeError(body) {
  const code = envelopeCode(body);
  if (!code || code === 200 || code === 201) return "";
  const message = trimmed(body.msg) || trimmed(body.message) || trimmed(body.error_msg) || trimmed(body.error);
  return message || "autodl upstream rejected the request (code " + code + ")";
}

function resultURL(data) {
  const results = payload(data).results;
  const list = Array.isArray(results) ? results : results === undefined || results === null ? [] : [results];
  for (const item of list) {
    if (typeof item === "string") {
      const url = trimmed(item);
      if (url) return url;
      continue;
    }
    if (isPlainObject(item)) {
      const url = trimmed(item.url) || trimmed(item.file_url) || trimmed(item.output);
      if (url) return url;
    }
  }
  return "";
}

function artifactFor(url) {
  const path = String(url).split("?")[0].toLowerCase();
  if (/\.(mp4|mov|mkv)$/.test(path)) return { key: "video", type: "video", mimeType: "video/mp4" };
  if (/\.webm$/.test(path)) return { key: "video", type: "video", mimeType: "video/webm" };
  if (/\.(mp3|m4a|aac)$/.test(path)) return { key: "audio", type: "audio", mimeType: "audio/mpeg" };
  if (/\.wav$/.test(path)) return { key: "audio", type: "audio", mimeType: "audio/wav" };
  if (/\.flac$/.test(path)) return { key: "audio", type: "audio", mimeType: "audio/flac" };
  if (/\.png$/.test(path)) return { key: "image", type: "image", mimeType: "image/png" };
  if (/\.jpe?g$/.test(path)) return { key: "image", type: "image", mimeType: "image/jpeg" };
  return { key: "file", type: "file" };
}

function responsesArtifact(ctx) {
  const artifacts = isPlainObject(ctx && ctx.artifacts) ? ctx.artifacts : {};
  for (const key of Object.keys(artifacts)) {
    const entry = isPlainObject(artifacts[key]) ? artifacts[key] : {};
    const url = trimmed(entry.url);
    if (url) return { url: url, type: trimmed(entry.type) || "video" };
  }
  return null;
}

function escapeAttribute(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

// The Responses surface renders the host-signed capability URL only; the
// short-lived upstream URL in Task.Data never leaves the gateway.
function responsesOutputHTML(ctx) {
  const artifact = responsesArtifact(ctx);
  if (!artifact) throw new Error("video artifact is unavailable");
  const tag = artifact.type === "audio" ? "audio" : "video";
  return "<" + tag + " controls src=\"" + escapeAttribute(artifact.url) + "\"></" + tag + ">";
}

function responsesInput(req) {
  const texts = [];
  const images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!isPlainObject(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!isPlainObject(part)) continue;
        if ((part.type === "input_text" || part.type === "text") && typeof part.text === "string") texts.push(part.text);
        if (part.type === "input_image" || part.type === "image_url") {
          let image = part.image_url;
          if (isPlainObject(image)) image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return {
    prompt: texts
      .filter(function (text) {
        return trimmed(text);
      })
      .join("\n"),
    images: images,
  };
}

function decodeResponsesRequest(ctx) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  const req = ctx.body.value;
  if (!isPlainObject(req)) throw new Error("request body must be an object");
  const model = trimmed(req.model);
  if (!model) throw new Error("model is required");
  if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) {
    throw new Error("input must be a string or array");
  }
  const input = responsesInput(req);
  const requestBody = { model: model, prompt: input.prompt };
  const images = input.images.slice();
  for (const candidate of [req.first_frame, req.last_frame]) {
    if (trimmed(candidate) && images.indexOf(trimmed(candidate)) < 0) images.push(trimmed(candidate));
  }
  if (images.length) requestBody.images = images;
  if (req.first_frame !== undefined) requestBody.first_frame = req.first_frame;
  if (req.last_frame !== undefined) requestBody.last_frame = req.last_frame;
  if (req.ref_video !== undefined) requestBody.ref_video = req.ref_video;
  if (req.seconds !== undefined) requestBody.duration = Number(req.seconds);
  else if (req.duration !== undefined) requestBody.duration = Number(req.duration);
  if (req.size !== undefined) requestBody.resolution = req.size;
  else if (req.resolution !== undefined) requestBody.resolution = req.resolution;
  if (req.seed !== undefined) requestBody.seed = req.seed;
  const norm = normalizeRequest(requestBody);
  const shape = detectShape(norm, isKnownPublicId(model) ? model : "");
  return { kind: "submit", model: model, action: ACTION_BY_SHAPE[shape], requestBody: requestBody };
}

function decodeVideoRequest(ctx) {
  const kind = ctx.body && ctx.body.kind;
  if (kind !== "json" && kind !== "form" && kind !== "multipart") throw new Error("JSON or multipart body required");
  let req = {};
  if (kind === "json") {
    if (!isPlainObject(ctx.body.value)) throw new Error("JSON object required");
    req = Object.assign({}, ctx.body.value);
  } else {
    const fields = ctx.body.fields || {};
    for (const name of Object.keys(fields)) {
      const values = fields[name];
      if (values.length > 1) throw new Error(name + " must be provided once");
      req[name] = values[0];
    }
    if (req.metadata !== undefined) {
      let parsed;
      try {
        parsed = JSON.parse(req.metadata);
      } catch (error) {
        throw new Error("metadata must be a JSON object string");
      }
      if (!isPlainObject(parsed)) throw new Error("metadata must be a JSON object string");
      req.metadata = parsed;
    }
    for (const file of ctx.body.files || []) {
      // AutoDL reference slots take a URL or a self-contained data: URL. The
      // host emits a bare base64 string only for encoding "base64"; any other
      // value becomes data:<mime>;base64,<payload>
      // (relay/channel/task/jsplugin/adaptor.go). AutoDL rejects the bare form
      // with RequestParameterIsWrong, so the placeholder asks for a data URL
      // and forwards the uploaded part's content type when the host declares it.
      if (file.field !== "input_reference" && file.field.indexOf("ref_image_") !== 0 && file.field.indexOf("ref_audio_") !== 0) {
        throw new Error("unexpected file field: " + file.field);
      }
      const slot = file.field === "input_reference" ? "ref_image_0" : file.field;
      const placeholder = { __fileRef: "request_file:" + file.field, encoding: "dataUrl" };
      const mimeType = trimmed(file.mimeType);
      if (mimeType) placeholder.mimeType = mimeType;
      req[slot] = placeholder;
    }
  }
  // The OpenAI Videos create body carries `seconds`; the driver contract uses
  // `duration`, so the normalized request body keeps only the latter.
  if (req.seconds !== undefined) {
    if (req.duration === undefined) req.duration = Number(req.seconds);
    delete req.seconds;
  }
  if (req.duration !== undefined) req.duration = Number(req.duration);
  const model = trimmed(ctx.model);
  if (!model && !trimmed(ctx.upstreamModel)) throw new Error("model is required");
  const norm = normalizeRequest(req);
  const shape = detectShape(norm, isKnownPublicId(model) ? model : "");
  return { kind: "submit", model: model || trimmed(ctx.upstreamModel), action: ACTION_BY_SHAPE[shape], requestBody: req };
}

function openaiVideoRender(task) {
  const statuses = {
    NOT_START: "queued",
    SUBMITTED: "queued",
    QUEUED: "queued",
    IN_PROGRESS: "in_progress",
    SUCCESS: "completed",
    FAILURE: "failed",
  };
  const output = {
    id: task.task_id,
    object: "video",
    model: task.properties && task.properties.origin_model_name ? task.properties.origin_model_name : "",
    status: statuses[task.status] || "unknown",
    progress: Number(String(task.progress || "0").replace("%", "")),
    created_at: task.created_at,
  };
  if (task.updated_at) output.completed_at = task.updated_at;
  const failure = envelopeError(task.data);
  if (failure) output.error = { message: failure, code: String(envelopeCode(task.data) || "") };
  return output;
}

function nativeTaskView(task) {
  const body = isPlainObject(task.data) ? task.data : {};
  const data = isPlainObject(body.data) ? body.data : {};
  return Object.assign({}, body, { data: Object.assign({}, data, { task_id: task.task_id }) });
}

export function buildSubmitRequest(ctx) {
  const plan = resolveSubmission(ctx, ctx.requestBody);
  return {
    url: endpointURL(ctx.baseUrl, SUBMIT_PATH_PREFIX + encodeURIComponent(plan.endpoint.workflowId)),
    method: "POST",
    headers: jsonHeaders(ctx),
    body: plan.body,
    action: ACTION_BY_SHAPE[plan.shape],
  };
}

export function parseSubmitResponse(ctx, response) {
  const body = isPlainObject(response.body) ? response.body : {};
  const failure = envelopeError(body);
  if (failure) throw new Error(failure);
  const data = payload(body);
  const taskId = trimmed(data.task_id);
  if (!taskId) throw new Error("autodl submit failed: the upstream response has no task_id");
  const plan = resolveSubmission(ctx, ctx.requestBody);
  const status = trimmed(data.status).toUpperCase();
  const state = {
    workflowId: plan.endpoint.workflowId,
    publicId: plan.endpoint.publicId,
    shape: plan.shape,
    billingMode: plan.endpoint.billingMode,
    seconds: plan.duration,
    resolution: plan.resolution.tier,
  };
  if (status === "FAILED" || status === "FAILURE") {
    return { taskId: taskId, taskData: body, state: state, immediate: { status: "FAILURE", reason: trimmed(data.msg) || "autodl task failed" } };
  }
  return { taskId: taskId, taskData: body, state: state };
}

export function buildQueryRequest(ctx) {
  return {
    url: endpointURL(ctx.baseUrl, RESULT_PATH_PREFIX + encodeURIComponent(ctx.taskId)),
    method: "GET",
    headers: { Accept: "application/json", Authorization: authorizationValue(ctx) },
  };
}

export function parseTaskResult(_ctx, body) {
  const failure = envelopeError(body);
  if (failure) return { status: "FAILURE", progress: "100%", reason: failure };
  const data = payload(body);
  const status = trimmed(data.status || (isPlainObject(body) ? body.status : "")).toUpperCase();
  if (status === "QUEUED") return { status: "QUEUED", progress: "30%" };
  if (status === "RUNNING") return { status: "IN_PROGRESS", progress: "50%" };
  if (status === "SUCCESS" || status === "SUCCEEDED" || status === "COMPLETED") {
    const result = { status: "SUCCESS", progress: "100%" };
    const url = resultURL(body);
    if (url) result.url = url;
    return result;
  }
  if (status === "FAILED" || status === "FAILURE") {
    return {
      status: "FAILURE",
      progress: "100%",
      reason: trimmed(data.msg) || trimmed(data.message) || trimmed(data.fail_reason) || "autodl task failed",
    };
  }
  return { status: "UNKNOWN", reason: "unrecognized status: " + String(status || "") };
}

// The caller's request is the source of usage facts. The upstream result does
// report a `duration`, but it is the task's processing time, not the clip
// length: on 2026-10-24 a 1.625 s 1088p clip reported `duration: 123`, exactly
// its 17:46:02→17:48:05 wall clock. Billing by it charged ~76× the real length,
// so settle reads the requested duration — the same fact the precharge used,
// and the same fact the per-second contract documents.
// Facts are reported for both usage purposes: a plain per-second ModelPrice
// multiplies `seconds`, and a tiered expression reads `seconds` + `resolution`.
export function extractUsage(ctx) {
  const plan = resolveSubmission(ctx, ctx.requestBody || {});
  if (plan.endpoint.billingMode === "per_call") return null;
  const facts = { seconds: plan.duration };
  if (plan.resolution.tier) facts.resolution = plan.resolution.tier;
  return facts;
}

export function extractUsageOnComplete(ctx, _result, _data) {
  const state = isPlainObject(ctx && ctx.state) ? ctx.state : {};
  if (state.billingMode === "per_call") return null;
  const seconds = Number(state.seconds);
  if (!Number.isFinite(seconds) || seconds <= 0 || seconds > MAX_DURATION_SECONDS) return null;
  const facts = { seconds: seconds };
  if (isOfferedTier(trimmed(state.resolution))) facts.resolution = trimmed(state.resolution);
  return facts;
}

export function listArtifacts(task) {
  if (!task || String(task.status).toUpperCase() !== "SUCCESS") return [];
  const url = resultURL(task.data);
  if (!url) return [];
  return [artifactFor(url)];
}

// Upstream result URLs are short-lived public CDN links, so the proxy request
// is credentialless and forwards no plugin headers.
export function buildContentRequest(ctx) {
  const url = resultURL(ctx.data);
  if (!url) throw new Error("artifact_not_found");
  if (artifactFor(url).key !== ctx.artifactKey) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

export const native = {
  createTask: function (ctx) {
    return decodeVideoRequest(ctx);
  },
  taskCreated: function (_ctx, task) {
    return nativeTaskView(task);
  },
  taskStatus: function (_ctx, task) {
    if (isPlainObject(task.data)) return nativeTaskView(task);
    const statuses = { NOT_START: "QUEUED", SUBMITTED: "QUEUED", QUEUED: "QUEUED", IN_PROGRESS: "RUNNING", SUCCESS: "SUCCESS", FAILURE: "FAILED" };
    return {
      code: 0,
      data: { task_id: task.task_id, status: statuses[task.status] || "QUEUED", msg: task.fail_reason || "" },
    };
  },
  error: function (_ctx, error) {
    return { code: error.code, message: error.message };
  },
};

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      return decodeResponsesRequest(ctx);
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const progressValue = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(progressValue) && progressValue >= 0 && progressValue <= 100 ? progressValue : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesOutputHTML(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE") {
        return {
          events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }],
          state: state,
          done: true,
        };
      }
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx, _task) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesOutputHTML(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "autodl" },
      };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      return decodeVideoRequest(ctx);
    },
    render: function (_ctx, task) {
      return openaiVideoRender(task);
    },
  },
};

export { CAP };
