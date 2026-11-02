package controller

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testVideoUploadMaxBytes = 200 * 1024 * 1024

type videoUploadZeroReader struct{}

func (videoUploadZeroReader) Read(data []byte) (int, error) {
	clear(data)
	return len(data), nil
}

func videoUploadBody(prefix []byte, size int64) io.Reader {
	return io.MultiReader(bytes.NewReader(prefix), io.LimitReader(videoUploadZeroReader{}, size-int64(len(prefix))))
}

func uploadVideoTestBox(kind string, payload []byte) []byte {
	data := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(data[:4], uint32(len(data)))
	copy(data[4:8], kind)
	copy(data[8:], payload)
	return data
}

func uploadVideoMovieIndex(handlerType string) []byte {
	handler := make([]byte, 24)
	copy(handler[8:12], handlerType)
	return uploadVideoTestBox("moov", append(uploadVideoTestBox("mvhd", make([]byte, 100)), uploadVideoTestBox("trak", uploadVideoTestBox("mdia", uploadVideoTestBox("hdlr", handler)))...))
}

type largeVideoUploadDriver struct {
	*presignTestDriver
	prefix       []byte
	suffix       []byte
	size         int64
	contentType  string
	getErr       error
	readBytes    int
	readerClosed bool
	uploadedSize int
}

func (driver *largeVideoUploadDriver) Upload(_ context.Context, key string, data []byte, _ string) (string, error) {
	driver.uploadedSize = len(data)
	return driver.GetPublicURL(key), nil
}

func (driver *largeVideoUploadDriver) Head(context.Context, string) (*storage.ObjectMetadata, error) {
	return &storage.ObjectMetadata{ContentType: driver.contentType, Size: driver.size}, nil
}

func (driver *largeVideoUploadDriver) Get(context.Context, string) (io.ReadCloser, string, int64, error) {
	if driver.getErr != nil {
		return nil, "", 0, driver.getErr
	}
	body := io.MultiReader(videoUploadBody(driver.prefix, driver.size-int64(len(driver.suffix))), bytes.NewReader(driver.suffix))
	return &largeVideoUploadReader{Reader: body, driver: driver}, driver.contentType, driver.size, nil
}

type largeVideoUploadReader struct {
	io.Reader
	driver *largeVideoUploadDriver
}

func (reader *largeVideoUploadReader) Read(data []byte) (int, error) {
	n, err := reader.Reader.Read(data)
	reader.driver.readBytes += n
	return n, err
}

func (reader *largeVideoUploadReader) Close() error {
	reader.driver.readerClosed = true
	return nil
}

func TestFileUploadStreamAcceptsLargeMP4AndRejectsDisguisedFiles(t *testing.T) {
	for _, sample := range []struct {
		name, mimeType string
		prefix         []byte
		wantStatus     int
	}{
		{"valid MP4", "video/mp4", append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), uploadVideoMovieIndex("vide")...), http.StatusOK},
		{"disguised text", "video/mp4", []byte("plain text, not a video"), http.StatusRequestEntityTooLarge},
		{"ordinary file", "application/octet-stream", []byte("ordinary file"), http.StatusRequestEntityTooLarge},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
			storage.SetGlobalDriver(driver)
			filename := "clip.mp4"
			if sample.name == "ordinary file" {
				filename = "file.bin"
			}
			var framing bytes.Buffer
			writer := multipart.NewWriter(&framing)
			_, err := writer.CreatePart(textproto.MIMEHeader{
				"Content-Disposition": {`form-data; name="file"; filename="` + filename + `"`},
				"Content-Type":        {sample.mimeType},
			})
			require.NoError(t, err)
			prefix := append([]byte(nil), framing.Bytes()...)
			require.NoError(t, writer.Close())
			suffix := framing.Bytes()[len(prefix):]
			size := int64(MaxUploadBytes + 1)
			body := io.MultiReader(bytes.NewReader(prefix), videoUploadBody(sample.prefix, size), bytes.NewReader(suffix))
			request := httptest.NewRequest(http.MethodPost, "/api/v1/files/upload/stream", body)
			request.ContentLength = size + int64(len(prefix)+len(suffix))
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, sample.wantStatus, response.Code, response.Body.String())
			if sample.wantStatus == http.StatusOK {
				assert.Equal(t, int(size), driver.uploadedSize)
			} else {
				assert.Zero(t, driver.uploadedSize)
			}
		})
	}
}

func TestFileUploadURLAcceptsMOVAtVideoSizeLimit(t *testing.T) {
	router := setupFileTestEnv(t)
	driver := &largeVideoUploadDriver{presignTestDriver: newPresignTestDriver(t, "https://cdn.example")}
	storage.SetGlobalDriver(driver)
	settings := system_setting.GetFetchSetting()
	previousAllow, previousPorts := settings.AllowPrivateIp, settings.AllowedPorts
	settings.AllowPrivateIp, settings.AllowedPorts = true, []string{"1-65535"}
	t.Cleanup(func() { settings.AllowPrivateIp, settings.AllowedPorts = previousAllow, previousPorts })
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "video/quicktime")
		prefix := append([]byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00qt  "), uploadVideoMovieIndex("vide")...)
		_, _ = io.Copy(writer, videoUploadBody(prefix, testVideoUploadMaxBytes))
	}))
	t.Cleanup(server.Close)
	response := postJSON(t, router, "/api/v1/files/upload/url", map[string]any{"file_url": server.URL + "/clip.mov"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var envelope dto.FileUploadResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data)
	assert.Equal(t, int64(testVideoUploadMaxBytes), envelope.Data.FileSize)
	assert.Equal(t, testVideoUploadMaxBytes, driver.uploadedSize)
}

func TestFilePresignAndConfirmLargeVideoChecksObjectBytes(t *testing.T) {
	for _, sample := range []struct {
		name       string
		prefix     []byte
		getErr     error
		wantStatus int
	}{
		{"valid MP4", append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), uploadVideoMovieIndex("vide")...), nil, http.StatusOK},
		{"MP4 signature without video track", []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), nil, http.StatusConflict},
		{"audio-only MP4", append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), uploadVideoMovieIndex("soun")...), nil, http.StatusConflict},
		{"excessive empty boxes", bytes.Repeat([]byte("\x00\x00\x00\x08free"), 4097), nil, http.StatusConflict},
		{"box size beyond the object", []byte("\xff\xff\xff\xffmdat"), nil, http.StatusConflict},
		{"forged MP4 metadata", []byte("this is not a movie"), nil, http.StatusConflict},
		{"temporary read failure", nil, errors.New("temporary storage failure"), http.StatusServiceUnavailable},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			driver := &largeVideoUploadDriver{
				presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), prefix: sample.prefix,
				size: testVideoUploadMaxBytes, contentType: "video/mp4", getErr: sample.getErr,
			}
			storage.SetGlobalDriver(driver)
			body := ticketBody()
			body["file_size"] = testVideoUploadMaxBytes
			ticket := requestTicket(t, router, body).Data
			response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
			require.Equal(t, sample.wantStatus, response.Code, response.Body.String())
			record, err := model.GetFileByFileId(ticket.FileID)
			require.NoError(t, err)
			switch sample.wantStatus {
			case http.StatusOK:
				assert.Equal(t, model.FileStatusActive, record.Status)
				assert.Equal(t, int64(testVideoUploadMaxBytes), record.FileSize)
			case http.StatusConflict:
				assert.Equal(t, model.FileStatusUploadRejected, record.Status)
				assert.Equal(t, 1, driver.deletes)
			case http.StatusServiceUnavailable:
				assert.Equal(t, model.FileStatusUploadPending, record.Status)
				assert.Zero(t, driver.deletes)
			}
			if sample.getErr == nil {
				assert.Positive(t, driver.readBytes)
				maxRead := testVideoUploadMaxBytes
				if sample.wantStatus == http.StatusOK || sample.name == "excessive empty boxes" || sample.name == "box size beyond the object" {
					maxRead = 64 * 1024
				}
				assert.LessOrEqual(t, driver.readBytes, maxRead)
				assert.True(t, driver.readerClosed)
			}
		})
	}
}

func TestFilePresignUploadRetainsOrdinaryFileLimit(t *testing.T) {
	router := setupFileTestEnv(t)
	storage.SetGlobalDriver(newPresignTestDriver(t, "https://cdn.example"))
	for _, sample := range []struct {
		filename, mimeType string
		size               int64
	}{
		{"large.png", "image/png", MaxUploadBytes + 1},
		{"large.bin", "application/octet-stream", MaxUploadBytes + 1},
		{"clip.mp4", "video/mp4", testVideoUploadMaxBytes + 1},
	} {
		response := postJSON(t, router, "/api/v1/files/upload/presign", map[string]any{
			"file_name": sample.filename, "mime_type": sample.mimeType, "file_size": sample.size,
		})
		assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	}
}

func TestFileConfirmLargeClassicMOVWithTrailingMovieIndex(t *testing.T) {
	movie := uploadVideoMovieIndex("vide")
	for _, sample := range []struct {
		name       string
		tail       []byte
		wantStatus int
	}{
		{"valid classic MOV", movie, http.StatusOK},
		{"mdat without movie structure", nil, http.StatusConflict},
	} {
		t.Run(sample.name, func(t *testing.T) {
			router := setupFileTestEnv(t)
			prefix := make([]byte, 8)
			binary.BigEndian.PutUint32(prefix[:4], uint32(testVideoUploadMaxBytes-len(sample.tail)))
			copy(prefix[4:], "mdat")
			driver := &largeVideoUploadDriver{
				presignTestDriver: newPresignTestDriver(t, "https://cdn.example"), prefix: prefix, suffix: sample.tail,
				size: testVideoUploadMaxBytes, contentType: "video/quicktime",
			}
			storage.SetGlobalDriver(driver)
			ticket := requestTicket(t, router, map[string]any{
				"file_name": "clip.mov", "mime_type": "video/quicktime", "file_size": testVideoUploadMaxBytes,
			}).Data
			response := postJSON(t, router, "/api/v1/files/upload/confirm", map[string]any{"file_id": ticket.FileID})
			require.Equal(t, sample.wantStatus, response.Code, response.Body.String())
			assert.Equal(t, testVideoUploadMaxBytes, driver.readBytes)
			assert.True(t, driver.readerClosed)
		})
	}
}
