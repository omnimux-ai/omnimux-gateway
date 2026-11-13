package controller

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fork: influencer-reference-metadata — the browser-declared reference-video
// facts are persisted with the file and echoed by the upload responses so a
// later consumer never has to read the media again.

func requirePersistedMetadata(t *testing.T, record *model.File, expected service.InfluencerVideoMetadata) {
	t.Helper()
	require.NotEmpty(t, record.VideoMetadata, "browser-reference upload must persist its declaration")
	var stored service.InfluencerVideoMetadata
	require.NoError(t, common.Unmarshal(record.VideoMetadata, &stored))
	assert.Equal(t, expected, stored)
}

func TestBrowserReferenceVideoMetadataPersistsAndEchoesOnConfirm(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &browserVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
	storage.SetGlobalDriver(driver)

	declared := browserUploadMetadata(1234)
	request := browserVideoTicketBody(1234)
	request["video_metadata"] = declared
	ticket := requestTicket(t, router, request).Data

	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	requirePersistedMetadata(t, record, declared)

	driver.put(t, record.StorageKey, make([]byte, 1234), "video/mp4")
	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var uploaded dto.FileUploadResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
	require.NotNil(t, uploaded.Data)
	require.NotNil(t, uploaded.Data.VideoMetadata, "confirm must echo the persisted declaration")
	assert.Equal(t, declared, *uploaded.Data.VideoMetadata)

	// Confirmation promotes the pending row; the declaration it was issued for
	// stays untouched.
	record, err = model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusActive, record.Status)
	requirePersistedMetadata(t, record, declared)
}

func TestBrowserReferenceVideoMetadataPersistsAndEchoesOnStream(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), contentType: "video/mp4"}
	storage.SetGlobalDriver(driver)

	const size = int64(2048)
	declared := browserUploadMetadata(size)
	rawMetadata, err := common.Marshal(declared)
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("purpose", influencerReferenceUploadPurpose))
	require.NoError(t, writer.WriteField("video_metadata", string(rawMetadata)))
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="clip.mp4"`},
		"Content-Type":        {"video/mp4"},
	})
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(make([]byte, size)))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var uploaded dto.FileUploadResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
	require.NotNil(t, uploaded.Data)
	require.NotNil(t, uploaded.Data.VideoMetadata, "stream must echo the persisted declaration")
	assert.Equal(t, declared, *uploaded.Data.VideoMetadata)

	var record model.File
	require.NoError(t, model.DB.First(&record).Error)
	requirePersistedMetadata(t, &record, declared)
	assert.Equal(t, influencerReferenceUploadPath, record.UploadPath)
}

func TestOrdinaryUploadsDoNotAcceptOrEchoVideoMetadata(t *testing.T) {
	declared := browserUploadMetadata(4)

	t.Run("presign-confirm", func(t *testing.T) {
		router := setupFileTestEnv(t)
		driver := newPresignTestDriver(t, "https://cdn.example")
		storage.SetGlobalDriver(driver)

		request := ticketBody()
		request["purpose"] = "another_purpose"
		request["video_metadata"] = declared
		ticket := requestTicket(t, router, request).Data

		record, err := model.GetFileByFileId(ticket.FileID)
		require.NoError(t, err)
		assert.Empty(t, record.VideoMetadata, "only influencer_reference_video may persist a declaration")

		driver.put(t, record.StorageKey, []byte("data"), "video/mp4")
		response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var uploaded dto.FileUploadResponse
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
		require.NotNil(t, uploaded.Data)
		assert.Nil(t, uploaded.Data.VideoMetadata)
		assert.NotContains(t, response.Body.String(), "video_metadata")
	})

	t.Run("stream", func(t *testing.T) {
		router := setupFileTestEnv(t)
		driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), contentType: "video/mp4"}
		storage.SetGlobalDriver(driver)

		rawMetadata, err := common.Marshal(declared)
		require.NoError(t, err)
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("video_metadata", string(rawMetadata)))
		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="file"; filename="clip.mp4"`},
			"Content-Type":        {"video/mp4"},
		})
		require.NoError(t, err)
		_, err = part.Write([]byte("data"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		request := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var uploaded dto.FileUploadResponse
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &uploaded))
		require.NotNil(t, uploaded.Data)
		assert.Nil(t, uploaded.Data.VideoMetadata)
		assert.NotContains(t, response.Body.String(), "video_metadata")

		var record model.File
		require.NoError(t, model.DB.First(&record).Error)
		assert.Empty(t, record.VideoMetadata)
	})
}

func TestBrowserReferenceVideoDeclarationStillRejectedBeforePersisting(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*service.InfluencerVideoMetadata)
	}{
		{"oversized pixels", func(m *service.InfluencerVideoMetadata) { m.Width = 99999 }},
		{"duration above thirty seconds", func(m *service.InfluencerVideoMetadata) { m.Duration, m.VideoDuration = 31, 31 }},
		{"frame rate below twenty four", func(m *service.InfluencerVideoMetadata) { m.FrameRate = 20 }},
		{"declared size does not match upload", func(m *service.InfluencerVideoMetadata) { m.SizeBytes = 1235 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := newPresignTestDriver(t, "https://cdn.example")
			storage.SetGlobalDriver(driver)

			declared := browserUploadMetadata(1234)
			test.change(&declared)
			request := browserVideoTicketBody(1234)
			request["video_metadata"] = declared
			response := postJSON(t, router, "/api/v1/files/upload/presign", request)
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assert.Zero(t, driver.presigns)

			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.Zero(t, count, "a rejected declaration must not leave a pending row")
		})
	}
}

func TestBrowserReferenceVideoConfirmRequiresPersistedDeclaration(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &browserVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
	storage.SetGlobalDriver(driver)

	now := time.Now()
	record := &model.File{
		FileId:       "file_pending_without_metadata",
		UserId:       42,
		FileName:     "clip.mp4",
		OriginalName: "clip.mp4",
		FileSize:     1234,
		MimeType:     "video/mp4",
		StorageKey:   influencerReferenceUploadPath + "/file_pending_without_metadata_clip.mp4",
		UploadPath:   influencerReferenceUploadPath,
		FileUrl:      "https://cdn.example/" + influencerReferenceUploadPath + "/file_pending_without_metadata_clip.mp4",
		CreatedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
		Status:       model.FileStatusUploadPending,
	}
	require.NoError(t, model.CreateFile(record))
	driver.put(t, record.StorageKey, make([]byte, 1234), "video/mp4")

	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": record.FileId})
	assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())

	record, err := model.GetFileByFileId(record.FileId)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusUploadRejected, record.Status)
}
