package apimart

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func roleImage(role string) map[string]any {
	return map[string]any{"url": "https://cdn.example.com/" + role + ".png", "role": role}
}

func request(operation string, fields map[string]any) map[string]any {
	out := map[string]any{"operation": operation}
	for key, value := range fields {
		out[key] = value
	}
	return out
}

func validPhaseOneCases() []struct {
	name      string
	model     string
	operation string
	body      map[string]any
} {
	var cases []struct {
		name      string
		model     string
		operation string
		body      map[string]any
	}
	for _, model := range []string{"seedance-2-0", "seedance-2-0-fast", "seedance-2-0-mini"} {
		cases = append(cases,
			struct {
				name      string
				model     string
				operation string
				body      map[string]any
			}{model + "/text", model, opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "duration": 5, "resolution": "720p", "size": "16:9"})},
			struct {
				name      string
				model     string
				operation string
				body      map[string]any
			}{model + "/first", model, opFirstFrame, request(opFirstFrame, map[string]any{"image_with_roles": []any{roleImage("first_frame")}})},
			struct {
				name      string
				model     string
				operation string
				body      map[string]any
			}{model + "/first-last", model, opFirstLastFrame, request(opFirstLastFrame, map[string]any{"image_with_roles": []any{roleImage("first_frame"), roleImage("last_frame")}})},
			struct {
				name      string
				model     string
				operation string
				body      map[string]any
			}{model + "/reference", model, opVideoMultiRef, request(opVideoMultiRef, map[string]any{"image_urls": []any{"asset://portrait"}})},
		)
	}
	cases = append(cases,
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/text", "seedance-2-5", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "duration": 30})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/first", "seedance-2-5", opFirstFrame, request(opFirstFrame, map[string]any{"prompt": "move", "size": "adaptive", "image_with_roles": []any{roleImage("first")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/first-last", "seedance-2-5", opFirstLastFrame, request(opFirstLastFrame, map[string]any{"prompt": "transition", "size": "adaptive", "image_with_roles": []any{roleImage("first"), roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/reference", "seedance-2-5", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"prompt": "reference", "audio_urls": []any{"asset://voice"}, "omni_reference_task_type": "reference"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/edit", "seedance-2-5", opVideoEdit, request(opVideoEdit, map[string]any{"prompt": "edit video", "video_urls": []any{"asset://source"}, "size": "adaptive", "duration": -1, "output_format": "mov", "omni_reference_task_type": "edit"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5/extend", "seedance-2-5", opVideoExtend, request(opVideoExtend, map[string]any{"prompt": "continue video", "image_urls": []any{"asset://look"}, "video_urls": []any{"asset://source"}, "audio_urls": []any{"asset://sound"}, "size": "adaptive", "duration": 8, "omni_reference_task_type": "extend"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/text", "seedance-2-5-flash", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "duration": 10, "resolution": "720p", "size": "16:9"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/first", "seedance-2-5-flash", opFirstFrame, request(opFirstFrame, map[string]any{"prompt": "move", "size": "adaptive", "resolution": "480p", "image_with_roles": []any{roleImage("first")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/first-last", "seedance-2-5-flash", opFirstLastFrame, request(opFirstLastFrame, map[string]any{"prompt": "transition", "size": "adaptive", "image_with_roles": []any{roleImage("first"), roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/reference", "seedance-2-5-flash", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"prompt": "reference", "audio_urls": []any{"https://cdn.example.com/voice.mp3"}, "omni_reference_task_type": "reference"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/edit", "seedance-2-5-flash", opVideoEdit, request(opVideoEdit, map[string]any{"prompt": "edit video", "video_urls": []any{"https://cdn.example.com/source.mp4"}, "size": "adaptive", "duration": -1, "output_format": "mov", "omni_reference_task_type": "edit"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"seedance-2-5-flash/extend", "seedance-2-5-flash", opVideoExtend, request(opVideoExtend, map[string]any{"prompt": "continue video", "video_urls": []any{"https://cdn.example.com/source.mp4"}, "size": "adaptive", "duration": 8, "omni_reference_task_type": "extend"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"magi-2/text", "magi-2", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "duration": 5, "resolution": "1080p", "aspect_ratio": "16:9", "generate_audio": true, "seed": 4294967295})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"magi-2/first", "magi-2", opFirstFrame, request(opFirstFrame, map[string]any{"aspect_ratio": "9:16", "image_with_roles": []any{roleImage("first")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"magi-2/first-last", "magi-2", opFirstLastFrame, request(opFirstLastFrame, map[string]any{"aspect_ratio": "21:9", "image_with_roles": []any{roleImage("first"), roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"magi-2/reference", "magi-2", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"audio_urls": []any{"https://cdn.example.com/voice.mp3"}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/text", "wan-3.0", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "720p", "duration": -1})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/first", "wan-3.0", opFirstFrame, request(opFirstFrame, map[string]any{"generation_type": "frame", "image_with_roles": []any{roleImage("first")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/first-last", "wan-3.0", opFirstLastFrame, request(opFirstLastFrame, map[string]any{"generation_type": "frame", "image_with_roles": []any{roleImage("first"), roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/reference", "wan-3.0", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"generation_type": "reference", "audio_urls": []any{"https://cdn.example.com/audio.mp3"}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/document", "wan-3.0", opDocumentToVideo, request(opDocumentToVideo, map[string]any{"file_url": "https://cdn.example.com/brief.pdf"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"wan/webpage", "wan-3.0", opWebpageToVideo, request(opWebpageToVideo, map[string]any{"link_url": "https://example.com/article"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"h3/text", "minimax-h3", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "aspect_ratio": "16:9", "resolution": "2K"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"h3/first", "minimax-h3", opFirstFrame, request(opFirstFrame, map[string]any{"prompt": "move", "aspect_ratio": "adaptive", "image_with_roles": []any{roleImage("first")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"h3/end", "minimax-h3", opEndFrame, request(opEndFrame, map[string]any{"prompt": "finish", "aspect_ratio": "adaptive", "image_with_roles": []any{roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"h3/first-last", "minimax-h3", opFirstLastFrame, request(opFirstLastFrame, map[string]any{"prompt": "transition", "aspect_ratio": "adaptive", "image_with_roles": []any{roleImage("first"), roleImage("last")}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"h3/reference", "minimax-h3", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"prompt": "reference", "image_urls": []any{"https://cdn.example.com/ref.png"}})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"grok/text", "grok-imagine-video-1-5", opTextToVideo, request(opTextToVideo, map[string]any{"prompt": "city", "duration": 15, "resolution": "1080p", "aspect_ratio": "3:2"})},
		struct {
			name      string
			model     string
			operation string
			body      map[string]any
		}{"grok/reference", "grok-imagine-video-1-5", opVideoMultiRef, request(opVideoMultiRef, map[string]any{"prompt": "reference", "image_urls": []any{"https://cdn.example.com/a.png", "https://cdn.example.com/a.png"}})},
	)
	return cases
}

func TestPhaseOneVideoContracts_AllDocumentedModesHaveValidFixture(t *testing.T) {
	for _, test := range validPhaseOneCases() {
		t.Run(test.name, func(t *testing.T) {
			normalized, err := validatePhaseOneVideoRequest(test.model, test.body)
			require.NoError(t, err)
			assert.Equal(t, test.operation, normalized["operation"])
			if test.model == "wan-3.0" && test.operation == opTextToVideo {
				assert.Equal(t, "720P", normalized["resolution"])
			}
		})
	}
}

func TestSeedanceVideoRoutingRequest(t *testing.T) {
	for _, tc := range []struct {
		name         string
		body         map[string]any
		want         modelcontracts.VideoRequest
		wantTaskType string
	}{
		{
			name: "frame roles and explicit parameter precedence",
			body: map[string]any{
				"prompt": "transition", "image_with_roles": []any{roleImage("first"), roleImage("last")},
				"size": "adaptive", "aspect_ratio": "16:9", "duration": 8.0, "seconds": "10", "draft": false,
			},
			want: modelcontracts.VideoRequest{
				Operation:  opFirstLastFrame,
				Slots:      map[string]int{"prompt": 1, "first_frame": 1, "last_frame": 1},
				Parameters: map[string]any{"aspect_ratio": "adaptive", "duration": 8},
			},
		},
		{
			name: "reference aliases and explicit auto duration and silent audio",
			body: map[string]any{
				"prompt": "reference", "images": []any{"asset://photo"},
				"audio_urls": []any{"asset://sound"}, "seconds": "-1", "audio": false,
				"resolution": "720p", "output_format": "mov",
			},
			want: modelcontracts.VideoRequest{
				Operation:  opVideoMultiRef,
				Slots:      map[string]int{"prompt": 1, "reference_images": 1, "reference_audios": 1},
				Parameters: map[string]any{"duration": -1, "generate_audio": false, "resolution": "720p", "output_format": "mov"},
			},
		},
		{
			name: "edit keeps omission and distinguishes source from auxiliary references",
			body: request(opVideoEdit, map[string]any{
				"prompt": "edit", "video_urls": []any{"asset://source"}, "audio_urls": []any{"asset://sound"},
			}),
			want: modelcontracts.VideoRequest{
				Operation:  opVideoEdit,
				Slots:      map[string]int{"prompt": 1, "source_videos": 1, "reference_audios": 1},
				Parameters: map[string]any{},
			},
			wantTaskType: "edit",
		},
		{
			name: "explicit extension selects the provider task type",
			body: request(opVideoExtend, map[string]any{
				"prompt": "extend", "video_urls": []any{"asset://source"}, "duration": 8,
			}),
			want: modelcontracts.VideoRequest{
				Operation:  opVideoExtend,
				Slots:      map[string]int{"prompt": 1, "source_videos": 1},
				Parameters: map[string]any{"duration": 8},
			},
			wantTaskType: "extend",
		},
		{
			name: "explicit reference video avoids provider auto detection",
			body: request(opVideoMultiRef, map[string]any{
				"prompt": "match the example", "video_urls": []any{"asset://reference"},
			}),
			want: modelcontracts.VideoRequest{
				Operation:  opVideoMultiRef,
				Slots:      map[string]int{"prompt": 1, "reference_videos": 1},
				Parameters: map[string]any{},
			},
			wantTaskType: "reference",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original, err := common.Marshal(tc.body)
			require.NoError(t, err)
			intent, err := VideoRoutingRequest("seedance-2.5", tc.body)
			require.NoError(t, err)
			assert.Equal(t, tc.want, intent)
			normalized, _, err := normalizeManagedVideoRequest("seedance-2.5", tc.body)
			require.NoError(t, err)
			if tc.wantTaskType == "" {
				assert.NotContains(t, normalized, "omni_reference_task_type")
			} else {
				assert.Equal(t, tc.wantTaskType, normalized["omni_reference_task_type"])
			}
			legacy, err := validatePhaseOneVideoRequest("seedance-2.5", tc.body)
			require.NoError(t, err)
			assert.NotContains(t, legacy, "omni_reference_task_type")
			current, err := common.Marshal(tc.body)
			require.NoError(t, err)
			assert.JSONEq(t, string(original), string(current))
		})
	}
}

func TestSeedanceVideoRoutingRejectsUnverifiedForwardedShapes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		body         map[string]any
		want         string
		rejectLegacy bool
	}{
		{
			name: "provider infers the task type for a video reference",
			body: map[string]any{"prompt": "reference", "video_urls": []any{"asset://reference"}},
			want: "set operation",
		},
		{
			name: "explicit subtype conflicts with the selected operation",
			body: request(opVideoExtend, map[string]any{
				"prompt": "extend", "video_urls": []any{"asset://source"}, "omni_reference_task_type": "edit",
			}),
			want:         "omni_reference_task_type extend",
			rejectLegacy: true,
		},
		{
			name: "provider audio flags conflict",
			body: request(opTextToVideo, map[string]any{"prompt": "city", "generate_audio": false, "audio": true}),
			want: "generate_audio and audio must agree",
		},
		{
			name: "provider draft changes resolution",
			body: request(opTextToVideo, map[string]any{"prompt": "city", "draft": true, "resolution": "720p"}),
			want: "draft and draft_task_id are unavailable",
		},
		{
			name: "draft completion inherits unverified inputs",
			body: request(opTextToVideo, map[string]any{"prompt": "city", "draft_task_id": "task_fixture"}),
			want: "draft and draft_task_id are unavailable",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, legacyErr := validatePhaseOneVideoRequest("seedance-2-5", tc.body)
			if tc.rejectLegacy {
				require.ErrorContains(t, legacyErr, tc.want)
			} else {
				require.NoError(t, legacyErr)
			}
			_, err := VideoRoutingRequest("seedance-2-5", tc.body)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestPhaseOneVideoContracts_EveryModeHasRejectedFixture(t *testing.T) {
	seen := map[string]bool{}
	for _, test := range validPhaseOneCases() {
		key := test.model + "/" + test.operation
		if seen[key] {
			continue
		}
		seen[key] = true
		var invalid map[string]any
		switch test.operation {
		case opTextToVideo:
			invalid = request(test.operation, nil)
		case opFirstFrame, opEndFrame, opVideoMultiRef, opVideoExtend, opDocumentToVideo, opWebpageToVideo:
			invalid = request(test.operation, map[string]any{"prompt": "instruction"})
		case opFirstLastFrame:
			invalid = request(test.operation, map[string]any{"prompt": "instruction", "image_with_roles": []any{roleImage("first")}})
		case opVideoEdit:
			invalid = request(test.operation, map[string]any{"prompt": "edit", "video_urls": []any{"asset://source"}, "duration": 5, "size": "adaptive"})
		default:
			t.Fatalf("missing invalid fixture for %s", test.operation)
		}
		_, err := validatePhaseOneVideoRequest(test.model, invalid)
		require.Error(t, err, fmt.Sprintf("%s must reject its invalid fixture", key))
	}
}

func TestPhaseOneVideoContracts_UnknownPublishedMaxStaysUnbounded(t *testing.T) {
	manyHTTPS := make([]any, 25)
	for index := range manyHTTPS {
		manyHTTPS[index] = fmt.Sprintf("https://cdn.example.com/%d.png", index)
	}
	_, err := validatePhaseOneVideoRequest("grok-imagine-video-1-5", request(opVideoMultiRef, map[string]any{
		"prompt": "references", "image_urls": manyHTTPS,
	}))
	require.NoError(t, err)
}

func TestPhaseOneVideoContracts_WanPublishedImageBoundary(t *testing.T) {
	images := make([]any, 10)
	for index := range images {
		images[index] = fmt.Sprintf("https://cdn.example.com/%d.png", index)
	}
	_, err := validatePhaseOneVideoRequest("wan-3.0", request(opVideoMultiRef, map[string]any{
		"image_urls": images, "generation_type": "reference",
	}))
	require.NoError(t, err)
	images = append(images, "https://cdn.example.com/10.png")
	_, err = validatePhaseOneVideoRequest("wan-3.0", request(opVideoMultiRef, map[string]any{
		"image_urls": images, "generation_type": "reference",
	}))
	require.ErrorContains(t, err, "at most 10 reference images")
}

func TestPhaseOneVideoContracts_WanDocumentFormats(t *testing.T) {
	_, err := validatePhaseOneVideoRequest("wan-3.0", request(opDocumentToVideo, map[string]any{
		"file_url": "https://cdn.example.com/brief.pages?signature=fixed",
	}))
	require.NoError(t, err)
	_, err = validatePhaseOneVideoRequest("wan-3.0", request(opDocumentToVideo, map[string]any{
		"file_url": "https://cdn.example.com/archive.zip",
	}))
	require.ErrorContains(t, err, "supported APIMart document format")
}

func TestPhaseOneVideoContracts_ReferenceRules(t *testing.T) {
	_, err := validatePhaseOneVideoRequest("seedance-2-0", request(opVideoMultiRef, map[string]any{
		"audio_urls": []any{"asset://audio"},
	}))
	require.ErrorContains(t, err, "cannot be used alone")

	_, err = validatePhaseOneVideoRequest("seedance-2-5", request(opVideoMultiRef, map[string]any{
		"prompt": "audio only", "audio_urls": []any{"asset://audio"}, "omni_reference_task_type": "reference",
	}))
	require.NoError(t, err)

	_, err = validatePhaseOneVideoRequest("minimax-h3", request(opFirstFrame, map[string]any{
		"prompt":           "mixed",
		"image_with_roles": []any{roleImage("first")},
		"image_urls":       []any{"https://cdn.example.com/ref.png"},
	}))
	require.ErrorContains(t, err, "mutually exclusive")
}

// The gateway's generic video body uses `images`; APIMart names the same
// reference collection `image_urls`. The contract must translate that alias so
// seedance video_multi_ref requests produced by the recreation worker pass.
func TestPhaseOneVideoContracts_GenericImagesAlias(t *testing.T) {
	normalized, err := validatePhaseOneVideoRequest("seedance-2-5", request(opVideoMultiRef, map[string]any{
		"prompt":                   "recreate the reference",
		"images":                   []any{"https://cdn.example.com/hero.png", "https://cdn.example.com/detail.png"},
		"resolution":               "480p",
		"seconds":                  "4",
		"aspect_ratio":             "9:16",
		"omni_reference_task_type": "reference",
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"https://cdn.example.com/hero.png", "https://cdn.example.com/detail.png"}, normalized["image_urls"])
	assert.NotContains(t, normalized, "images")

	_, err = validatePhaseOneVideoRequest("seedance-2-5", request(opVideoMultiRef, map[string]any{
		"prompt":     "conflict",
		"images":     []any{"https://cdn.example.com/a.png"},
		"image_urls": []any{"https://cdn.example.com/b.png"},
	}))
	require.ErrorContains(t, err, "cannot both be set")
}

func TestPhaseOneVideoContracts_GrokHTTPSAndDuplicateOrder(t *testing.T) {
	body := request(opVideoMultiRef, map[string]any{
		"prompt": "keep order",
		"image_urls": []any{
			"https://cdn.example.com/b.png",
			"https://cdn.example.com/a.png",
			"https://cdn.example.com/b.png",
		},
	})
	normalized, err := validatePhaseOneVideoRequest("grok-imagine-video-1-5", body)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://cdn.example.com/b.png",
		"https://cdn.example.com/a.png",
		"https://cdn.example.com/b.png",
	}, normalized["image_urls"])

	body["image_urls"] = []any{"http://cdn.example.com/a.png"}
	_, err = validatePhaseOneVideoRequest("grok-imagine-video-1-5", body)
	require.ErrorContains(t, err, "HTTPS")
}

func TestPhaseOneVideoContracts_PromptUsesUnicodeLength(t *testing.T) {
	_, err := validatePhaseOneVideoRequest("grok-imagine-video-1-5", request(opTextToVideo, map[string]any{
		"prompt": strings.Repeat("界", 8001),
	}))
	require.ErrorContains(t, err, "8000")
}

func TestOmniFlashExtCaps(t *testing.T) {
	require.True(t, isOmniFlashExtModel("Omni-Flash-Ext"))
	require.True(t, isOmniFlashExtModel("gemini-omni-1.1-flash-task"))
	require.True(t, isOmniFlashExtModel("  gemini-omni-1.1-flash-ext  "))
	require.False(t, isOmniFlashExtModel("seedance-2-5"))

	// 覆盖区间内：≤1080P 与 4–10 秒，大小写不敏感（且不改写调用方传来的值）。
	for _, resolution := range []string{"360p", "720p", "1080P", "1080p"} {
		body := map[string]any{"resolution": resolution, "duration": 10}
		require.NoError(t, validateOmniFlashExtCaps(body))
		require.Equal(t, resolution, body["resolution"], "校验不得改写上游参数口径")
	}
	for _, duration := range []int{4, 6, 8, 10} {
		require.NoError(t, validateOmniFlashExtCaps(map[string]any{"resolution": "720p", "duration": duration}))
	}
	// 未传时长交给上游默认档（$0.35/次）。
	require.NoError(t, validateOmniFlashExtCaps(map[string]any{"resolution": "720p"}))
	require.NoError(t, validateOmniFlashExtCaps(map[string]any{}))

	// 超档必须拒绝：4K（上游 $0.75–0.90/次 > 售价）、超时长、自动时长 -1、过短。
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"resolution": "4k", "duration": 4}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"resolution": "4K", "duration": 4}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"4k": true, "duration": 4}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"quality": "4k", "duration": 4}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"duration": 11}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"duration": 30}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"duration": -1}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"duration": 2}))
	require.Error(t, validateOmniFlashExtCaps(map[string]any{"resolution": "1440p", "duration": 4}))
}

func TestPhaseOneVideoContracts_Magi2AndSeedance25FlashBoundaries(t *testing.T) {
	// MAGI-2 rejects adaptive ratio, non-1080p resolution, -1 duration, http media URL,
	// prompt >15000 runes, seed > 4294967295, unsupported fields, and >10/5/5 references.
	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"magi-2 adaptive ratio", request(opFirstFrame, map[string]any{"aspect_ratio": "adaptive", "image_with_roles": []any{roleImage("first")}}), "aspect_ratio"},
		{"magi-2 720p rejected", request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "720p"}), "resolution"},
		{"magi-2 auto duration rejected", request(opTextToVideo, map[string]any{"prompt": "city", "duration": -1}), "duration"},
		{"magi-2 duration 3 rejected", request(opTextToVideo, map[string]any{"prompt": "city", "duration": 3}), "duration"},
		{"magi-2 prompt >15000 runes", request(opTextToVideo, map[string]any{"prompt": strings.Repeat("风", 15001)}), "15000"},
		{"magi-2 http image rejected", request(opVideoMultiRef, map[string]any{"image_urls": []any{"http://cdn.example.com/a.png"}}), "HTTPS"},
		{"magi-2 output_format rejected", request(opTextToVideo, map[string]any{"prompt": "city", "output_format": "mp4"}), "output_format"},
		{"magi-2 seed overflow rejected", request(opTextToVideo, map[string]any{"prompt": "city", "seed": int64(4294967296)}), "4294967295"},
		{"magi-2 http callback rejected", request(opTextToVideo, map[string]any{"prompt": "city", "callback_url": "http://example.com/cb"}), "callback_url"},
		{"magi-2 >5 videos rejected", request(opVideoMultiRef, map[string]any{"video_urls": []any{"https://a/1.mp4", "https://a/2.mp4", "https://a/3.mp4", "https://a/4.mp4", "https://a/5.mp4", "https://a/6.mp4"}}), "5 reference videos"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validatePhaseOneVideoRequest("magi-2", tc.body)
			require.ErrorContains(t, err, tc.want)
		})
	}

	// Seedance 2.5 Flash rejects 1080p/4k, >10 images, >5 videos, >5 audios, draft, and non-adaptive first_frame.
	elevenImages := make([]any, 11)
	for i := range elevenImages {
		elevenImages[i] = fmt.Sprintf("https://cdn.example.com/%d.png", i)
	}
	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"flash 1080p rejected", request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "1080p"}), "resolution"},
		{"flash 4k rejected", request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "4k"}), "resolution"},
		{"flash >10 images rejected", request(opVideoMultiRef, map[string]any{"prompt": "ref", "image_urls": elevenImages}), "10 image"},
		{"flash >5 audios rejected", request(opVideoMultiRef, map[string]any{"prompt": "ref", "audio_urls": []any{"https://a/1.mp3", "https://a/2.mp3", "https://a/3.mp3", "https://a/4.mp3", "https://a/5.mp3", "https://a/6.mp3"}}), "5 audio"},
		{"flash first_frame requires adaptive", request(opFirstFrame, map[string]any{"prompt": "move", "size": "16:9", "image_with_roles": []any{roleImage("first")}}), "adaptive"},
		{"flash draft rejected", request(opTextToVideo, map[string]any{"prompt": "city", "draft": true}), "draft"},
		{"flash resolution/quality conflict rejected", request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "480p", "quality": "720p"}), "resolution and quality must match"},
		{"flash quality 1080p rejected", request(opTextToVideo, map[string]any{"prompt": "city", "quality": "1080p"}), "resolution"},
		{"flash pixel size conflicts with resolution", request(opTextToVideo, map[string]any{"prompt": "city", "resolution": "720p", "size": "854x480"}), "conflicts with size"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validatePhaseOneVideoRequest("seedance-2-5-flash", tc.body)
			require.ErrorContains(t, err, tc.want)
		})
	}

	// Dotted spelling seedance-2.5-flash must still pass through phase-one validation.
	_, err := validatePhaseOneVideoRequest("seedance-2.5-flash", request(opTextToVideo, map[string]any{
		"prompt": "city", "resolution": "4k",
	}))
	require.ErrorContains(t, err, "resolution")

	// OpenAI-style WxH pixel sizes normalize to resolution + aspect ratio for seedance-2-5-flash.
	normalized, err := validatePhaseOneVideoRequest("seedance-2-5-flash", request(opTextToVideo, map[string]any{
		"prompt": "city", "size": "854x480", "duration": 5,
	}))
	require.NoError(t, err)
	assert.Equal(t, "480p", normalized["resolution"])
	assert.Equal(t, "16:9", normalized["size"])
}

// The published executable set must not drift from operationAllowed. The
// catalog is a promise a client renders and submits from; if the validator
// accepts fewer operations than the contract, the client offers dead modes.
func TestPublishedExecutableOperationsMatchAPIMartContract(t *testing.T) {
	want := map[string][]string{
		"seedance-2-0":           {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef},
		"seedance-2-0-fast":      {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef},
		"seedance-2-0-mini":      {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef},
		"seedance-2-5":           {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef, opVideoEdit, opVideoExtend},
		"seedance-2-5-flash":     {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef, opVideoEdit, opVideoExtend},
		"magi-2":                 {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef},
		"wan-3.0":                {opTextToVideo, opFirstFrame, opFirstLastFrame, opVideoMultiRef, opDocumentToVideo, opWebpageToVideo},
		"minimax-h3":             {opTextToVideo, opFirstFrame, opEndFrame, opFirstLastFrame, opVideoMultiRef},
		"grok-imagine-video-1-5": {opTextToVideo, opVideoMultiRef},
	}

	for model, expected := range want {
		contract, ok := modelcontracts.For(model)
		require.True(t, ok, "model %s has no published contract", model)
		got := make([]string, 0, len(contract.Operations))
		for _, operation := range contract.Operations {
			if operation.Executable {
				got = append(got, operation.ID)
			}
		}
		assert.ElementsMatch(t, expected, got, "model %s executable operations drifted from operationAllowed", model)
	}
}
