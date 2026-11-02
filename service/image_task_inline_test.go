package service

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inlineTaskPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func inlineTaskSettings(t *testing.T) {
	t.Helper()
	oldSecret, oldAddress, oldPublic := common.CryptoSecret, system_setting.ServerAddress, system_setting.TaskPublicAddress
	common.CryptoSecret = "inline-test-secret"
	system_setting.ServerAddress = "https://images.example"
	system_setting.TaskPublicAddress = ""
	t.Cleanup(func() {
		common.CryptoSecret = oldSecret
		system_setting.ServerAddress = oldAddress
		system_setting.TaskPublicAddress = oldPublic
		storage.ResetGlobalDriver()
	})
}

func TestPersistInlineImageTaskPreservesOriginalAndReadBack(t *testing.T) {
	inlineTaskSettings(t)
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	original := inlineTaskPNG(t)
	task := &model.Task{TaskID: "task_inline_original", UserId: 7}
	result, err := PersistInlineImageTask(t.Context(), task, base64.StdEncoding.EncodeToString(original))
	require.NoError(t, err)
	assert.Equal(t, 3, result.Width)
	assert.Equal(t, 2, result.Height)
	assert.Equal(t, int64(len(original)), result.Bytes)
	assert.Equal(t, sha256.Sum256(original), result.SHA256)
	assert.Contains(t, result.URL, "https://images.example/v1/tasks/task_inline_original/artifacts/image/content?access=")
	stored, _, err := ReadTaskArtifact(t.Context(), task, "image", MaxInlineImageTaskBytes)
	require.NoError(t, err)
	assert.Equal(t, original, stored)
	assert.Empty(t, task.Status, "storing bytes must not invent a successful task")
}

func TestInlineImageTaskRejectsMalformedOrNoncanonicalPNG(t *testing.T) {
	original := inlineTaskPNG(t)
	encoded := base64.StdEncoding.EncodeToString(original)
	badCRC := append([]byte(nil), original...)
	badCRC[len(badCRC)-1] ^= 1
	noIEND := original[:len(original)-12]
	bomb := append([]byte(nil), original...)
	binary.BigEndian.PutUint32(bomb[16:20], 10000)
	binary.BigEndian.PutUint32(bomb[20:24], 4000)
	bomb[24] = 16
	bomb[25] = 6
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	for name, value := range map[string]string{
		"newline": encoded[:8] + "\n" + encoded[8:], "url alphabet": strings.Replace(encoded, "+", "-", 1) + "-", "missing padding": strings.TrimRight(encoded, "="),
		"trailing bytes": base64.StdEncoding.EncodeToString(append(append([]byte(nil), original...), 1)), "second PNG": base64.StdEncoding.EncodeToString(append(append([]byte(nil), original...), original...)),
		"bad CRC": base64.StdEncoding.EncodeToString(badCRC), "missing IEND": base64.StdEncoding.EncodeToString(noIEND), "high bit depth allocation bomb": base64.StdEncoding.EncodeToString(bomb), "not PNG": base64.StdEncoding.EncodeToString([]byte("jpeg")),
	} {
		t.Run(name, func(t *testing.T) { _, _, err := decodeInlineImageTaskPNG(value); require.Error(t, err) })
	}
}

func TestInlineImageTaskRejectsOversizedBase64BeforeDecode(t *testing.T) {
	_, _, err := decodeInlineImageTaskPNG(strings.Repeat("A", base64.StdEncoding.EncodedLen(int(MaxInlineImageTaskBytes))+4))
	require.Error(t, err)
}

type inlineFaultDriver struct {
	storage.StorageDriver
	data                                     []byte
	failUpload, failRead, shortRead, corrupt bool
	cancel                                   context.CancelFunc
	writes, reads                            int
}

func (d *inlineFaultDriver) Upload(ctx context.Context, key string, data []byte, mime string) (string, error) {
	d.writes++
	if d.failUpload {
		return "", errors.New("upload failed")
	}
	d.data = append([]byte(nil), data...)
	if d.cancel != nil {
		d.cancel()
	}
	return "", nil
}
func (d *inlineFaultDriver) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	d.reads++
	if d.failRead {
		return nil, "", 0, errors.New("read failed")
	}
	data := append([]byte(nil), d.data...)
	if d.shortRead {
		data = data[:len(data)-1]
	}
	if d.corrupt {
		data[len(data)/2] ^= 1
	}
	return io.NopCloser(bytes.NewReader(data)), "application/octet-stream", int64(len(data)), nil
}

func TestInlineImageTaskNeverPublishesFailedPersistenceOrReadback(t *testing.T) {
	inlineTaskSettings(t)
	original := inlineTaskPNG(t)
	for _, name := range []string{"upload", "read", "short", "different"} {
		t.Run(name, func(t *testing.T) {
			d := &inlineFaultDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir()), failUpload: name == "upload", failRead: name == "read", shortRead: name == "short", corrupt: name == "different"}
			storage.SetGlobalDriver(d)
			result, err := PersistInlineImageTask(t.Context(), &model.Task{TaskID: "task_inline_failure", UserId: 7}, base64.StdEncoding.EncodeToString(original))
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, 1, d.writes)
			if name != "upload" {
				assert.Equal(t, 1, d.reads)
			}
		})
	}
}

func TestInlineImageTaskRejectsUnsafeIdentityLinkAndCancellationBeforeWriting(t *testing.T) {
	inlineTaskSettings(t)
	d := &inlineFaultDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir())}
	storage.SetGlobalDriver(d)
	encoded := base64.StdEncoding.EncodeToString(inlineTaskPNG(t))
	for _, task := range []*model.Task{nil, {TaskID: "task_valid", UserId: 0}, {TaskID: "../other", UserId: 7}} {
		_, err := PersistInlineImageTask(t.Context(), task, encoded)
		require.Error(t, err)
	}
	system_setting.ServerAddress = "http://images.example"
	_, err := PersistInlineImageTask(t.Context(), &model.Task{TaskID: "task_valid", UserId: 7}, encoded)
	require.Error(t, err)
	system_setting.ServerAddress = "https://images.example"
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = PersistInlineImageTask(ctx, &model.Task{TaskID: "task_valid", UserId: 7}, encoded)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, d.writes)
}

func TestInlineImageTaskCancellationAfterWriteLeavesNoSuccess(t *testing.T) {
	inlineTaskSettings(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	driver := &inlineFaultDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir()), cancel: cancel}
	storage.SetGlobalDriver(driver)
	result, err := PersistInlineImageTask(ctx, &model.Task{TaskID: "task_canceled_after_save", UserId: 7}, base64.StdEncoding.EncodeToString(inlineTaskPNG(t)))
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, result)
	assert.Equal(t, 1, driver.writes)
	assert.Zero(t, driver.reads)
	assert.NotEmpty(t, driver.data, "an object may exist while the submission remains unknown; it is not a receipt")
}

// This is a small genuine grayscale+tRNS PNG: transparent grayscale expands
// to NRGBA/NRGBA64 in the standard decoder, unlike an opaque Gray/Gray16 PNG.
func inlineTransparentGrayPNG(t *testing.T, depth byte) []byte {
	t.Helper()
	rowByDepth := map[byte][]byte{1: {0x40}, 2: {0x18}, 4: {0x01, 0x20}, 8: {0, 1, 2}, 16: {0, 0, 0, 1, 0, 2}}
	row, ok := rowByDepth[depth]
	require.True(t, ok)
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	for range 2 {
		_, err := writer.Write(append([]byte{0}, row...))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[:4], 3)
	binary.BigEndian.PutUint32(header[4:8], 2)
	header[8] = depth
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	for _, chunk := range []struct {
		kind string
		data []byte
	}{{"IHDR", header}, {"tRNS", []byte{0, 1}}, {"IDAT", compressed.Bytes()}, {"IEND", nil}} {
		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(chunk.data)))
		out.Write(length)
		out.WriteString(chunk.kind)
		out.Write(chunk.data)
		hash := crc32.NewIEEE()
		_, err := hash.Write([]byte(chunk.kind))
		require.NoError(t, err)
		_, err = hash.Write(chunk.data)
		require.NoError(t, err)
		binary.BigEndian.PutUint32(length, hash.Sum32())
		out.Write(length)
	}
	return out.Bytes()
}

func TestInlineImageTaskTransparentGrayDecoderAllocation(t *testing.T) {
	for _, depth := range []byte{1, 2, 4, 8, 16} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			data := inlineTransparentGrayPNG(t, depth)
			decoded, err := png.Decode(bytes.NewReader(data))
			require.NoError(t, err)
			if depth == 16 {
				require.IsType(t, &image.NRGBA64{}, decoded)
				assert.Len(t, decoded.(*image.NRGBA64).Pix, 3*2*8)
			} else {
				require.IsType(t, &image.NRGBA{}, decoded)
				assert.Len(t, decoded.(*image.NRGBA).Pix, 3*2*4)
			}
			_, _, _, alpha := decoded.At(1, 0).RGBA()
			assert.Zero(t, alpha)
			width, height, err := validateInlineImageTaskPNG(data)
			require.NoError(t, err)
			assert.Equal(t, 3, width)
			assert.Equal(t, 2, height)
		})
	}
}

func TestInlineImageTaskTransparentGrayBudgetRejectsBeforeDecode(t *testing.T) {
	for _, depth := range []byte{1, 2, 4, 8, 16} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			data := inlineTransparentGrayPNG(t, depth)
			binary.BigEndian.PutUint32(data[16:20], 6000)
			binary.BigEndian.PutUint32(data[20:24], 4000)
			if depth != 16 {
				data[28] = 1
			}
			// Intentionally keep the small fixture's old IHDR CRC. If a regression
			// reaches png.Decode, its header checksum fails before any image allocation;
			// this test never decodes a large image or supplies large compressed pixels.
			require.NotEqual(t, binary.BigEndian.Uint32(data[29:33]), crc32.ChecksumIEEE(data[12:29]))
			_, _, err := validateInlineImageTaskPNG(data)
			require.ErrorContains(t, err, "decoded image exceeds the reviewed memory limit")
		})
	}
}
