package advancedcustom

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	accountMonitorShapeContextKey = "advancedcustom.account-monitor-shape"
	accountMonitorShapeMaxBody    = 2 * 1024 * 1024
	accountMonitorShapeMaxSlots   = 32
	accountMonitorShapeMaxBytes   = 4 * 1024
	accountMonitorShapeObserved   = "observed"
	accountMonitorShapeTooLarge   = "too_large"
	accountMonitorShapeLimit      = "summary_limit"
)

// AccountMonitorShapeSlot contains only existence and a closed JSON type. Slot
// positions are fixed by schema version and model; upstream keys are never used
// as output names. Nonempty is present only for objects and arrays.
type AccountMonitorShapeSlot struct {
	Present  bool   `json:"present"`
	Type     string `json:"type"`
	Nonempty *bool  `json:"nonempty,omitempty"`
}

// AccountMonitorShapeEvidence is a bounded diagnostic, not evidence that the
// upstream payload is a valid profile or post collection.
type AccountMonitorShapeEvidence struct {
	SchemaVersion int                       `json:"schema_version"`
	Model         string                    `json:"model"`
	Status        string                    `json:"status"`
	Slots         []AccountMonitorShapeSlot `json:"slots,omitempty"`
}

type accountMonitorShapeCollector struct {
	model    string
	evidence *AccountMonitorShapeEvidence
}

// EnableAccountMonitorShapeDiagnostic is called only by the bounded management
// probe on its own internal context. No request field or header is inspected.
func EnableAccountMonitorShapeDiagnostic(c *gin.Context, model string) bool {
	if c == nil || (model != "tiktok-user" && model != "x-posts") {
		return false
	}
	policy := dto.AdvancedCustomResponseValidationTikHubUser
	if model == "x-posts" {
		policy = dto.AdvancedCustomResponseValidationTikHubPosts
	}
	// Preserve the internal API's bounded transport even when called directly.
	EnableAccountMonitorProbe(c, model, policy)
	c.Set(accountMonitorShapeContextKey, &accountMonitorShapeCollector{model: model})
	return true
}

func GetAccountMonitorShapeEvidence(c *gin.Context) *AccountMonitorShapeEvidence {
	collector := accountMonitorShapeCollectorFromContext(c)
	if collector == nil {
		return nil
	}
	return collector.evidence
}

func accountMonitorShapeCollectorFromContext(c *gin.Context) *accountMonitorShapeCollector {
	if c == nil {
		return nil
	}
	value, exists := c.Get(accountMonitorShapeContextKey)
	if !exists {
		return nil
	}
	collector, ok := value.(*accountMonitorShapeCollector)
	if !ok {
		return nil
	}
	return collector
}

func accountMonitorShapeCollectorFor(c *gin.Context, model, policy string) *accountMonitorShapeCollector {
	collector := accountMonitorShapeCollectorFromContext(c)
	if collector == nil || collector.model != model {
		return nil
	}
	if (model == "tiktok-user" && policy == dto.AdvancedCustomResponseValidationTikHubUser) ||
		(model == "x-posts" && policy == dto.AdvancedCustomResponseValidationTikHubPosts) {
		return collector
	}
	return nil
}

func (collector *accountMonitorShapeCollector) setStatus(status string) {
	collector.evidence = boundAccountMonitorShapeEvidence(&AccountMonitorShapeEvidence{
		SchemaVersion: accountMonitorShapeSchemaVersion(collector.model), Model: collector.model, Status: status,
	})
}

func (collector *accountMonitorShapeCollector) collect(body []byte, validationErr error) {
	if validationErr == nil {
		return
	}
	if (collector.model != "tiktok-user" || validationErr.Error() != "account monitoring upstream profile is unrecognized") &&
		(collector.model != "x-posts" || validationErr.Error() != "account monitoring upstream posts are unrecognized") {
		return
	}
	// A matching error is returned only after the existing validator has decoded
	// this same body. Projection neither introduces a new successful shape nor
	// enumerates keys, arrays or unknown wrapper paths.
	root := gjson.ParseBytes(body)
	data := root.Get("data")
	payload := root
	if data.IsObject() || data.IsArray() {
		payload = data
	}
	if collector.model == "tiktok-user" && accountMonitorBusinessFailure(accountMonitorProfileLayer(payload)) {
		return
	}
	slots := []AccountMonitorShapeSlot{accountMonitorShapeSlot(root), accountMonitorShapeSlot(data), accountMonitorShapeSlot(payload)}
	if collector.model == "tiktok-user" {
		slots = append(slots, accountMonitorTikTokShapeSlots(payload)...)
	} else {
		slots = append(slots, accountMonitorXShapeSlots(payload)...)
	}
	collector.evidence = boundAccountMonitorShapeEvidence(&AccountMonitorShapeEvidence{
		SchemaVersion: accountMonitorShapeSchemaVersion(collector.model), Model: collector.model, Status: accountMonitorShapeObserved, Slots: slots,
	})
}

// TikTok slots 3..21 are fixed, falsifiable type hypotheses only. In particular,
// userInfo/user_info do not become supported profile containers through this
// diagnostic. Slots 0..2 always describe R, R.data and the existing payload P.
func accountMonitorTikTokShapeSlots(payload gjson.Result) []AccountMonitorShapeSlot {
	paths := [...]string{
		"userInfo", "userInfo.user", "userInfo.stats",
		"userInfo.user.nickname", "userInfo.user.signature", "userInfo.user.avatarLarger",
		"userInfo.stats.followerCount", "userInfo.stats.videoCount",
		"user", "user.nickname", "nickname", "data", "result",
		"user_info", "user_info.nickname", "user_info.follower_count",
		"userInfo.nickname", "userInfo.followerCount", "userInfo.videoCount",
	}
	slots := make([]AccountMonitorShapeSlot, 0, len(paths))
	for _, path := range paths {
		slots = append(slots, accountMonitorShapeSlot(payload.Get(path)))
	}
	return slots
}

// X schema 3 keeps the previous fixed root/intermediate types and appends the
// observed direct timeline array after all previous source candidates.
// Diagnostic hypotheses such as tweets do not become supported post lists.
func accountMonitorXShapeSlots(payload gjson.Result) []AccountMonitorShapeSlot {
	paths := [...]string{
		"items", "posts", "edges", "data", "result", "list", "media", "user", "user.result",
		"user.result.timeline_v2", "user.result.timeline_v2.timeline", "user.result.timeline",
		"timeline", "timeline_v2", "timeline_v2.timeline", "data.user", "data.user.result",
		"tweets", "data.tweets", "result.tweets",
	}
	slots := make([]AccountMonitorShapeSlot, 0, 29)
	for _, path := range paths {
		slots = append(slots, accountMonitorShapeSlot(payload.Get(path)))
	}
	var source gjson.Result
	for _, path := range [...]string{"items", "posts", "edges", "data", "result", "list", "media", "tweets", "data.tweets", "result.tweets", "timeline"} {
		if value := payload.Get(path); value.IsArray() {
			source = value
			break
		}
	}
	first := accountMonitorShapeFirst(source)
	node := first.Get("node")
	for _, value := range [...]gjson.Result{source, first, node, first.Get("id"), first.Get("post_id"), first.Get("tweet_id"), first.Get("rest_id"), node.Get("id"), node.Get("rest_id")} {
		slots = append(slots, accountMonitorShapeSlot(value))
	}
	return slots
}

func accountMonitorShapeSchemaVersion(model string) int {
	if model == "x-posts" {
		return 3
	}
	return 1
}

func accountMonitorShapeFirst(value gjson.Result) gjson.Result {
	if !value.IsArray() {
		return gjson.Result{}
	}
	return value.Get("0")
}

func accountMonitorShapeSlot(value gjson.Result) AccountMonitorShapeSlot {
	slot := AccountMonitorShapeSlot{Present: value.Exists(), Type: "missing"}
	if !slot.Present {
		return slot
	}
	switch {
	case value.IsObject():
		slot.Type = "object"
	case value.IsArray():
		slot.Type = "array"
	case value.Type == gjson.Null:
		slot.Type = "null"
	case value.Type == gjson.True || value.Type == gjson.False:
		slot.Type = "boolean"
	case value.Type == gjson.Number:
		slot.Type = "number"
	case value.Type == gjson.String:
		slot.Type = "string"
	}
	if slot.Type == "object" || slot.Type == "array" {
		// The validated JSON delimiters are known here. Checking their interior
		// distinguishes empty containers without enumerating keys or items.
		nonempty := len(strings.TrimSpace(value.Raw[1:len(value.Raw)-1])) != 0
		slot.Nonempty = &nonempty
	}
	return slot
}

func boundAccountMonitorShapeEvidence(evidence *AccountMonitorShapeEvidence) *AccountMonitorShapeEvidence {
	if evidence == nil || (evidence.Model != "tiktok-user" && evidence.Model != "x-posts") {
		return nil
	}
	version := accountMonitorShapeSchemaVersion(evidence.Model)
	limited := &AccountMonitorShapeEvidence{SchemaVersion: version, Model: evidence.Model, Status: accountMonitorShapeLimit}
	if evidence.SchemaVersion != version || len(evidence.Slots) > accountMonitorShapeMaxSlots {
		return limited
	}
	if evidence.Status != accountMonitorShapeObserved && evidence.Status != accountMonitorShapeTooLarge && evidence.Status != accountMonitorShapeLimit {
		return limited
	}
	if evidence.Status != accountMonitorShapeObserved && len(evidence.Slots) != 0 {
		return limited
	}
	if evidence.Status == accountMonitorShapeObserved && evidence.Model == "x-posts" && len(evidence.Slots) != 32 {
		return limited
	}
	for _, slot := range evidence.Slots {
		switch slot.Type {
		case "missing":
			if slot.Present || slot.Nonempty != nil {
				return limited
			}
		case "null", "boolean", "number", "string":
			if !slot.Present || slot.Nonempty != nil {
				return limited
			}
		case "object", "array":
			if !slot.Present || slot.Nonempty == nil {
				return limited
			}
		default:
			return limited
		}
	}
	encoded, err := common.Marshal(evidence)
	if err != nil || len(encoded) > accountMonitorShapeMaxBytes {
		return limited
	}
	return evidence
}
