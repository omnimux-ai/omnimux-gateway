package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareInfluencerMotionPreservesAnimalTypeWithoutProviderFields(t *testing.T) {
	for _, characterType := range []string{"cat", "dog"} {
		t.Run(characterType, func(t *testing.T) {
			request := validMotionRequest()
			request["subject_character_type"] = characterType
			recorder, probe := runMotionPrepare(t, request)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
			require.True(t, probe.reached)
			assert.Contains(t, probe.body["prompt"], "anthropomorphic "+characterType)
			assert.Contains(t, probe.body["prompt"], "fur")
			assert.Contains(t, probe.body["prompt"], "tail")
			assert.Contains(t, probe.body["prompt"], "PRIORITY: the saved "+characterType+" species")
			assert.Equal(t, characterType, probe.meta["subject_character_type"])
			assert.NotContains(t, probe.body, "subject_character_type")
			assert.NotContains(t, probe.body, "source_task_id")
		})
	}
}

func TestPrepareInfluencerMotionRejectsUnknownCharacterTypeBeforeDistribution(t *testing.T) {
	request := validMotionRequest()
	request["subject_character_type"] = "dragon"
	recorder, probe := runMotionPrepare(t, request)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.False(t, probe.reached)
}

func TestPrepareInfluencerMotionSourceTaskPinsSpeciesAndArtifact(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousAddress, previousPublic, previousSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = "https://gateway.example", "", "species-local-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = previousAddress, previousPublic, previousSecret
		require.NoError(t, sqlDB.Close())
	})
	for index, raw := range []string{"null", "{}", `{"tier":null,"selection":{}}`, `{"tier":"","selection":{}}`} {
		taskID := fmt.Sprintf("corrupt-source-%d", index)
		task := model.Task{TaskID: taskID, UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: raw, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/corrupt.png"}}
		require.NoError(t, db.Create(&task).Error)
		imageURL, err := service.BuildTaskArtifactContentURL(taskID, "image")
		require.NoError(t, err)
		t.Run("corrupt source "+raw, func(t *testing.T) {
			request := validMotionRequest()
			request["source_task_id"] = taskID
			request["subject_image_url"] = imageURL
			recorder, probe := runMotionPrepare(t, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, probe.reached)
		})
	}
	for _, task := range []model.Task{
		{TaskID: "cat-source", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/cat.png"}},
		{TaskID: "dog-source", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"dogs","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/dog.png"}},
		{TaskID: "human-source", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"normal","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/human.png"}},
		{TaskID: "foreign-source", UserId: 99, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"dogs","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/dog.png"}},
		{TaskID: "pending-source", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusInProgress, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/pending.png"}},
		{TaskID: "wrong-action-source", UserId: 41, Action: ActionInfluencerMotion, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/wrong.png"}},
		{TaskID: "legacy-board", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"kind":"multiview","parent_task_id":"cat-source"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/board.png"}},
	} {
		require.NoError(t, db.Create(&task).Error)
	}
	catURL, err := service.BuildTaskArtifactContentURL("cat-source", "image")
	require.NoError(t, err)
	boardURL, err := service.BuildTaskArtifactContentURL("legacy-board", "image")
	require.NoError(t, err)
	t.Run("only first image is authoritative", func(t *testing.T) {
		request := validMotionRequest()
		request["source_task_id"] = "cat-source"
		request["subject_image_url"] = catURL
		request["subject_image_urls"] = []string{motionTestImage, catURL}
		recorder, probe := runMotionPrepare(t, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.False(t, probe.reached)
	})
	for _, tc := range []struct {
		name, taskID, imageURL, declaredType string
		status                               int
	}{
		{name: "source supplies type", taskID: "cat-source", imageURL: catURL, status: http.StatusNoContent},
		{name: "declared matches source", taskID: "cat-source", imageURL: catURL, declaredType: "cat", status: http.StatusNoContent},
		{name: "old child inherits parent", taskID: "legacy-board", imageURL: boardURL, status: http.StatusNoContent},
		{name: "declared type conflicts", taskID: "cat-source", imageURL: catURL, declaredType: "dog", status: http.StatusBadRequest},
		{name: "foreign task", taskID: "foreign-source", imageURL: catURL, status: http.StatusNotFound},
		{name: "missing task", taskID: "missing-source", imageURL: catURL, status: http.StatusNotFound},
		{name: "unrelated image", taskID: "cat-source", imageURL: motionTestImage, status: http.StatusBadRequest},
		{name: "wrong artifact origin", taskID: "cat-source", imageURL: strings.Replace(catURL, "gateway.example", "other.example", 1), status: http.StatusBadRequest},
		{name: "missing artifact signature", taskID: "cat-source", imageURL: strings.Split(catURL, "?")[0], status: http.StatusBadRequest},
		{name: "non-image artifact key", taskID: "cat-source", imageURL: strings.Replace(catURL, "/image/", "/video/", 1), status: http.StatusBadRequest},
		{name: "pending source", taskID: "pending-source", imageURL: catURL, status: http.StatusBadRequest},
		{name: "wrong action source", taskID: "wrong-action-source", imageURL: catURL, status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := validMotionRequest()
			request["source_task_id"] = tc.taskID
			request["subject_image_url"] = tc.imageURL
			if tc.declaredType != "" {
				request["subject_character_type"] = tc.declaredType
			}
			recorder, probe := runMotionPrepare(t, request)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status != http.StatusNoContent {
				assert.False(t, probe.reached)
				return
			}
			assert.Contains(t, probe.body["prompt"], "anthropomorphic cat")
			assert.Equal(t, "cat", probe.meta["subject_character_type"])
			assert.Equal(t, tc.taskID, probe.meta["source_task_id"])
			assert.NotContains(t, probe.body, "source_task_id")
		})
	}
	for _, tc := range []struct {
		taskID, characterType, identity string
	}{
		{"cat-source", "cat", "拟人猫身份、猫物种与双足结构"},
		{"dog-source", "dog", "拟人狗身份、狗物种与双足结构"},
		{"human-source", "human", "人物身份"},
		{"legacy-board", "cat", "拟人猫身份、猫物种与双足结构"},
	} {
		t.Run("replace clothing from "+tc.taskID, func(t *testing.T) {
			imageURL, err := service.BuildTaskArtifactContentURL(tc.taskID, "image")
			require.NoError(t, err)
			request := validMotionRequest()
			request["mode"], request["source_task_id"] = "replace", tc.taskID
			request["subject_image_url"] = imageURL
			const instruction = "只换衣服，保留原人物的脸。"
			supplement := strings.Repeat("字", influencerMotionMaxPromptLen-utf8.RuneCountInString(instruction)) + instruction
			require.Equal(t, influencerMotionMaxPromptLen, utf8.RuneCountInString(supplement))
			request["prompt"] = supplement
			recorder, probe := runMotionPrepare(t, request)
			require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
			prompt, ok := probe.body["prompt"].(string)
			require.True(t, ok)
			assert.True(t, strings.HasPrefix(prompt, influencerMotionPrompts["replace"]))
			assert.Contains(t, prompt, "仅用于身份替换时采用其已核实身份："+tc.identity)
			assert.Contains(t, prompt, "局部引用仅作用对应部分。用户补充的明确意图优先。")
			framing := strings.TrimSuffix(prompt, "\n\n用户补充：\n"+supplement)
			for _, specific := range []string{"服饰", "衣物", "物品", "袖长", "类别", "同类"} {
				assert.NotContains(t, framing, specific, "verified identity must not prescribe a category-specific edit")
			}
			assert.True(t, strings.HasSuffix(prompt, "\n\n用户补充：\n"+supplement))
			assert.NotContains(t, prompt, "PRIORITY:")
			assert.NotContains(t, prompt, "take precedence over conflicting user instructions")
			assert.LessOrEqual(t, utf8.RuneCountInString(prompt), 4600)
			assert.Equal(t, supplement, probe.meta["prompt"])
			assert.Equal(t, tc.characterType, probe.meta["subject_character_type"])
			assert.Equal(t, tc.taskID, probe.meta["source_task_id"])
			assert.NotContains(t, probe.body, "subject_character_type")
			assert.NotContains(t, probe.body, "source_task_id")
		})
	}
}

func TestPublishInfluencerPresetDefaultAnimalKeepsTypeAndEmptySelection(t *testing.T) {
	setupInfluencerPresetTest(t)
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	task := &model.Task{TaskID: "default-cat-preset", UserId: influencerPresetTestUser, Status: model.TaskStatusSuccess, Action: ActionInfluencerSheet, Properties: model.Properties{Input: `{"tier":"cats","selection":{},"brief":""}`}}
	require.NoError(t, model.DB.Create(task).Error)
	artwork := influencerPresetTestPNG(t, 16, 9)
	recorder := influencerPresetTestPublish(t, engine, map[string]string{"task_id": task.TaskID, "name": "Default cat"}, map[string][]byte{"preview": artwork})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Data struct {
			Tier      string              `json:"tier"`
			Selection map[string][]string `json:"selection"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "cats", response.Data.Tier)
	assert.Empty(t, response.Data.Selection)
}

func TestPublishInfluencerPresetRejectsUnknownSavedType(t *testing.T) {
	setupInfluencerPresetTest(t)
	engine := influencerPresetTestEngine(influencerPresetTestUser)
	task := &model.Task{TaskID: "unknown-type-preset", UserId: influencerPresetTestUser, Status: model.TaskStatusSuccess, Action: ActionInfluencerSheet, Properties: model.Properties{Input: `{"tier":"retired","selection":{"gender":["female"]}}`}}
	require.NoError(t, model.DB.Create(task).Error)
	recorder := influencerPresetTestPublish(t, engine, map[string]string{"task_id": task.TaskID, "name": "Unknown"}, map[string][]byte{"preview": influencerPresetTestPNG(t, 16, 9)})
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	var count int64
	require.NoError(t, model.DB.Model(&model.InfluencerPreset{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestPrepareInfluencerMotionUnknownUploadUsesNeutralCharacter(t *testing.T) {
	request := validMotionRequest()
	recorder, probe := runMotionPrepare(t, request)
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	assert.Contains(t, probe.body["prompt"], "character from reference image 1")
	assert.NotContains(t, probe.body["prompt"], "Animate the person")
	assert.NotContains(t, probe.meta, "subject_character_type")
}
