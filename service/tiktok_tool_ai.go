package service

import (
	"context"
	"time"
)

// TikTokToolAIChat executes an internal structured-JSON prompt on the shared
// planner channel transport. Tests replace this variable to avoid upstream calls.
var TikTokToolAIChat = tiktokToolAIChatViaChannel

// StripJSONFence removes markdown code fences around model JSON replies.
func StripJSONFence(text string) string {
	return influencerMotionStripFence(text)
}

func tiktokToolAIChatViaChannel(ctx context.Context, modelName, group, system, userPrompt string, check func(string) error) (string, error) {
	boundedCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	content := []map[string]any{{"type": "text", "text": userPrompt}}
	result, err := influencerPlannerChatViaChannel(boundedCtx, modelName, group, system, content, check, newInfluencerMotionPlanSession())
	return result.Text, err
}
