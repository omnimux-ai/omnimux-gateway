package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func motionPlanTask(input string) *model.Task {
	return &model.Task{Action: ActionInfluencerMotion, Properties: model.Properties{Input: input}}
}

func stubMotionPlanner(t *testing.T, plan service.InfluencerMotionPlan, err error, seen *service.InfluencerMotionPlanInput) {
	t.Helper()
	previous := influencerMotionPlanner
	t.Cleanup(func() { influencerMotionPlanner = previous })
	influencerMotionPlanner = func(_ context.Context, in service.InfluencerMotionPlanInput) (service.InfluencerMotionPlan, error) {
		if seen != nil {
			*seen = in
		}
		return plan, err
	}
}

func TestPlanInfluencerMotionPromptReplacesFixedFramingForMotion(t *testing.T) {
	var seen service.InfluencerMotionPlanInput
	stubMotionPlanner(t, service.InfluencerMotionPlan{Prompt: "【主体】planned【场景】x"}, nil, &seen)
	out := map[string]any{"prompt": "fixed", "videos": []any{"https://files.example/v.mp4"}, "images": []any{"https://files.example/i.png"}}
	planInfluencerMotionPrompt(context.Background(), motionPlanTask(`{"mode":"motion","prompt":"沙发绿色","duration_sec":4,"subject_image_tags":["角色"]}`), out)
	assert.Equal(t, "【主体】planned【场景】x", out["prompt"])
	assert.Equal(t, "沙发绿色", seen.UserPrompt, "the caller's words go to the planner, not appended to a fixed framing")
	assert.Equal(t, []string{"角色"}, seen.ImageTags)
	assert.Equal(t, "https://files.example/v.mp4", seen.VideoURL)
}

func TestPlanInfluencerMotionPromptKeepsFixedFramingWhenPlannerFails(t *testing.T) {
	stubMotionPlanner(t, service.InfluencerMotionPlan{}, errors.New("down"), nil)
	out := map[string]any{"prompt": "fixed", "videos": []any{"https://files.example/v.mp4"}, "images": []any{"https://files.example/i.png"}}
	planInfluencerMotionPrompt(context.Background(), motionPlanTask(`{"mode":"motion"}`), out)
	assert.Equal(t, "fixed", out["prompt"])
}

func TestPlanInfluencerMotionPromptLeavesObjectSwapUntouched(t *testing.T) {
	stubMotionPlanner(t, service.InfluencerMotionPlan{}, nil, nil)
	previous := influencerMotionPlanner
	influencerMotionPlanner = func(context.Context, service.InfluencerMotionPlanInput) (service.InfluencerMotionPlan, error) {
		t.Fatal("object swap must not be planned")
		return service.InfluencerMotionPlan{}, nil
	}
	t.Cleanup(func() { influencerMotionPlanner = previous })
	request := validMotionRequest()
	request["mode"], request["prompt"] = "replace", ""
	recorder, probe := runMotionPrepare(t, request)
	require.True(t, probe.reached, recorder.Body.String())
	input, err := common.Marshal(probe.meta)
	require.NoError(t, err)
	planInfluencerMotionPrompt(context.Background(), motionPlanTask(string(input)), probe.body)
	assert.Equal(t, influencerMotionPrompts["replace"], probe.body["prompt"])
	assert.Equal(t, []any{motionTestImage}, probe.body["images"])
	assert.Equal(t, []any{motionTestFrozenVideo}, probe.body["videos"])
}

func TestPlanInfluencerMotionPromptKeepsAnimalSpeciesPriority(t *testing.T) {
	stubMotionPlanner(t, service.InfluencerMotionPlan{Prompt: "【主体】cat【场景】x"}, nil, nil)
	out := map[string]any{"videos": []any{"https://files.example/v.mp4"}, "images": []any{"https://files.example/i.png"}}
	planInfluencerMotionPrompt(context.Background(), motionPlanTask(`{"mode":"motion","subject_character_type":"cat"}`), out)
	prompt, _ := out["prompt"].(string)
	require.Contains(t, prompt, "【主体】cat")
	assert.Contains(t, prompt, "PRIORITY: the saved cat species")
}
