package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type browserVideoUploadDriver struct {
	*presignTestDriver
	gets, heads int
}

func (d *browserVideoUploadDriver) Head(_ context.Context, key string) (*storage.ObjectMetadata, error) {
	d.heads++
	object, ok := d.objects[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return &storage.ObjectMetadata{Size: int64(len(object.body)), ContentType: object.contentType}, nil
}
func (d *browserVideoUploadDriver) Get(context.Context, string) (io.ReadCloser, string, int64, error) {
	d.gets++
	return nil, "", 0, errors.New("browser-checked upload must not be read for server verification")
}

func browserUploadMetadata(size int64) service.InfluencerVideoMetadata {
	return service.InfluencerVideoMetadata{Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24, Container: "mp4", VideoCodec: "h264", SizeBytes: size}
}
func browserVideoTicketBody(size int64) map[string]any {
	body := ticketBody()
	body["file_size"] = size
	body["purpose"] = "influencer_reference_video"
	body["video_metadata"] = browserUploadMetadata(size)
	return body
}

func TestBrowserVideoLargeDirectUploadConfirmsWithoutReadingMedia(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &browserVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
	storage.SetGlobalDriver(driver)
	size := MaxUploadBytes + 1
	request := browserVideoTicketBody(int64(size))
	metadata := browserUploadMetadata(int64(size))
	metadata.Duration, metadata.VideoDuration, metadata.AudioCodec = 7.072, 7.041667, "aac"
	request["video_metadata"] = metadata
	ticket := requestTicket(t, router, request).Data
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, "influencer-reference/browser-upload", record.UploadPath)
	// Transfer an actual >100MiB payload. Its content deliberately carries no
	// server-readable movie index: only the browser is the media acceptance gate.
	content := make([]byte, size)
	copy(content, "browser-checked source")
	driver.put(t, record.StorageKey, content, "video/mp4")
	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, 1, driver.heads)
	assert.Zero(t, driver.gets)
	record, err = model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.FileStatusActive, record.Status)
	assert.Equal(t, int64(size), record.FileSize)
}

func TestBrowserVideoUploadRequiresBoundDeclarationsBeforeTicket(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing metadata", func(r map[string]any) { delete(r, "video_metadata") }},
		{"size differs", func(r map[string]any) { r["file_size"] = int64(5678) }},
		{"mime differs", func(r map[string]any) { r["mime_type"] = "image/png" }},
		{"visual duration cannot follow container", func(r map[string]any) {
			m := browserUploadMetadata(1234)
			m.VideoDuration = 4.2
			r["video_metadata"] = m
		}},
		{"visual duration below four cannot extend", func(r map[string]any) {
			m := browserUploadMetadata(1234)
			m.VideoDuration = 3.999
			r["video_metadata"] = m
		}},
		{"declarations differ beyond two frames", func(r map[string]any) {
			m := browserUploadMetadata(1234)
			m.Duration = 4.5
			m.VideoDuration = 4.2
			r["video_metadata"] = m
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := newPresignTestDriver(t, "https://cdn.example")
			storage.SetGlobalDriver(driver)
			request := browserVideoTicketBody(1234)
			test.change(request)
			response := postJSON(t, router, "/api/v1/files/upload/presign", request)
			assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			assert.Zero(t, driver.presigns)
		})
	}
}

func TestBrowserVideoPurposeCannotBeAddedAtConfirmation(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), size: MaxUploadBytes + 1, contentType: "video/mp4", prefix: []byte("not a movie")}
	storage.SetGlobalDriver(driver)
	request := ticketBody()
	request["file_size"] = MaxUploadBytes + 1
	request["upload_path"] = "influencer-reference/browser-upload"
	ticket := requestTicket(t, router, request).Data
	record, err := model.GetFileByFileId(ticket.FileID)
	require.NoError(t, err)
	assert.NotEqual(t, "influencer-reference/browser-upload", record.UploadPath)
	response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID, "purpose": "influencer_reference_video", "video_metadata": browserUploadMetadata(MaxUploadBytes + 1)})
	assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	assert.Positive(t, driver.readBytes, "ordinary uploads retain existing content acceptance")
}

func TestBrowserVideoLargeStreamFallbackDoesNotInspectContainer(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
	storage.SetGlobalDriver(driver)
	size := int64(MaxUploadBytes + 1)
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	require.NoError(t, writer.WriteField("purpose", "influencer_reference_video"))
	declared := browserUploadMetadata(size)
	declared.Duration, declared.VideoDuration, declared.AudioCodec = 7.072, 7.041667, "aac"
	metadata, err := common.Marshal(declared)
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("video_metadata", string(metadata)))
	_, err = writer.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="clip.mp4"`}, "Content-Type": {"video/mp4"}})
	require.NoError(t, err)
	prefix := bytes.Clone(framing.Bytes())
	require.NoError(t, writer.Close())
	suffix := framing.Bytes()[len(prefix):]
	body := io.MultiReader(bytes.NewReader(prefix), videoUploadBody([]byte("browser accepted source"), size), bytes.NewReader(suffix))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", body)
	request.ContentLength = size + int64(len(prefix)+len(suffix))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, int(size), driver.uploadedSize)
	var record model.File
	require.NoError(t, model.DB.First(&record).Error)
	assert.Equal(t, "influencer-reference/browser-upload", record.UploadPath)
	assert.True(t, strings.HasPrefix(record.StorageKey, "influencer-reference/browser-upload/"))
}

func TestBrowserVideoAudioPaddingAtPresignAndStreamEntrances(t *testing.T) {
	for _, ingress := range []string{"presign", "stream"} {
		for _, test := range []struct {
			name               string
			container, picture float64
			audio, mime        string
			sizeDifference     int64
			accept             bool
		}{
			{"reported seven second clip", 7.072, 7.041667, "aac", "video/mp4", 0, true},
			{"four second audio padded clip", 4.064, 4.041667, "aac", "video/mp4", 0, true},
			{"production fractional clip", 5.717007, 5.708333, "aac", "video/mp4", 0, true},
			{"consistent fractional clip", 7.2, 7.2, "aac", "video/mp4", 0, true},
			{"extra tail without audio within two frames", 7.072, 7.041667, "", "video/mp4", 0, true},
			{"too much audio tail", 7.2, 7, "aac", "video/mp4", 0, false},
			{"declarations beyond two frames", 4.5, 4.2, "aac", "video/mp4", 0, false},
			{"mismatched MIME", 7.072, 7.041667, "aac", "image/png", 0, false},
			{"mismatched byte count", 7.072, 7.041667, "aac", "video/mp4", 1, false},
		} {
			t.Run(ingress+"/"+test.name, func(t *testing.T) {
				router := setupFileTestEnv(t)
				driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
				storage.SetGlobalDriver(driver)
				const actualSize = int64(6650581)
				metadata := service.InfluencerVideoMetadata{Duration: test.container, VideoDuration: test.picture, Width: 480, Height: 854, FrameRate: 24, VideoCodec: "h264", AudioCodec: test.audio, Container: "mp4", SizeBytes: actualSize + test.sizeDifference}
				var response *httptest.ResponseRecorder
				if ingress == "presign" {
					request := browserVideoTicketBody(actualSize)
					request["mime_type"], request["video_metadata"] = test.mime, metadata
					response = postJSON(t, router, "/api/v1/files/upload/presign", request)
				} else {
					var body bytes.Buffer
					writer := multipart.NewWriter(&body)
					require.NoError(t, writer.WriteField("purpose", "influencer_reference_video"))
					rawMetadata, err := common.Marshal(metadata)
					require.NoError(t, err)
					require.NoError(t, writer.WriteField("video_metadata", string(rawMetadata)))
					part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="video (1).mp4"`}, "Content-Type": {test.mime}})
					require.NoError(t, err)
					// Deliberately not an encoded video: this path must consume the
					// declaration without server-side media-fact verification.
					_, err = io.Copy(part, videoUploadBody([]byte("browser checked source"), actualSize))
					require.NoError(t, err)
					require.NoError(t, writer.Close())
					request := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", &body)
					request.Header.Set("Content-Type", writer.FormDataContentType())
					response = httptest.NewRecorder()
					router.ServeHTTP(response, request)
				}
				if !test.accept {
					assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
					assert.Zero(t, driver.presigns)
					assert.Zero(t, driver.uploadedSize)
					return
				}
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var record model.File
				require.NoError(t, model.DB.First(&record).Error)
				assert.Equal(t, "influencer-reference/browser-upload", record.UploadPath)
				assert.Equal(t, 42, record.UserId)
				assert.Equal(t, actualSize, record.FileSize)
				assert.Zero(t, driver.readBytes)
				if ingress == "presign" {
					assert.Equal(t, 1, driver.presigns)
					assert.Zero(t, driver.uploadedSize)
				} else {
					assert.Zero(t, driver.presigns)
					assert.Equal(t, int(actualSize), driver.uploadedSize)
				}
			})
		}
	}
}

func TestBrowserVideoConfirmationRetainsObjectSizeAndTypeChecks(t *testing.T) {
	for _, test := range []struct {
		name, mime string
		actualSize int
	}{
		{"size-mismatch", "video/mp4", 1235},
		{"type-mismatch", "image/png", 1234},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &browserVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
			storage.SetGlobalDriver(driver)
			ticket := requestTicket(t, router, browserVideoTicketBody(1234)).Data
			record, err := model.GetFileByFileId(ticket.FileID)
			require.NoError(t, err)
			driver.put(t, record.StorageKey, make([]byte, test.actualSize), test.mime)
			response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
			assert.Equal(t, http.StatusConflict, response.Code, response.Body.String())
			assert.Zero(t, driver.gets)
			assert.Equal(t, 1, driver.deletes)
			record, err = model.GetFileByFileId(ticket.FileID)
			require.NoError(t, err)
			assert.Equal(t, model.FileStatusUploadRejected, record.Status)
		})
	}
}
