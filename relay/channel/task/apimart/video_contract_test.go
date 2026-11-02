package apimart

import (
	"fmt"
	"strings"
	"testing"

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
