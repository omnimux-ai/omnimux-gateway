package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image/png"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// These limits apply only to the reviewed inline image contract; the shared
// artifact store retains its existing limits for other media.
const (
	MaxInlineImageTaskBytes         int64  = 20 << 20
	MaxInlineImageTaskResponseBytes int64  = 32 << 20
	inlineImageTaskMaxDecodedBytes  uint64 = 160 << 20
)

type InlineImageTaskResult struct {
	URL    string
	Width  int
	Height int
	Bytes  int64
	SHA256 [sha256.Size]byte
}

// InlineImageTaskContentURL validates the server-owned result destination
// before the supplier request is sent. It never uses the request Host header.
func InlineImageTaskContentURL(task *model.Task) (string, error) {
	if task == nil || task.UserId <= 0 || !validArtifactStoreTarget(task, "image") {
		return "", fmt.Errorf("inline image task identity is invalid")
	}
	contentURL, err := BuildTaskArtifactContentURL(task.TaskID, "image")
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(contentURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("inline image task requires an HTTPS artifact address")
	}
	return contentURL, nil
}

// PersistInlineImageTask publishes no success until the original PNG is saved
// and independently read back from the same fixed task/image object. Storage
// alone is not an accepted receipt and cannot recover an unknown submission.
func PersistInlineImageTask(ctx context.Context, task *model.Task, encoded string) (*InlineImageTaskResult, error) {
	contentURL, err := InlineImageTaskContentURL(task)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	original, result, err := decodeInlineImageTaskPNG(encoded)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := PersistTaskArtifact(ctx, task, "image", "image/png", original); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	readback, _, err := ReadTaskArtifact(ctx, task, "image", MaxInlineImageTaskBytes)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(readback) != len(original) || sha256.Sum256(readback) != result.SHA256 {
		return nil, fmt.Errorf("inline image artifact readback differs from the original")
	}
	if _, _, err := validateInlineImageTaskPNG(readback); err != nil {
		return nil, err
	}
	result.URL = contentURL
	return result, nil
}

func decodeInlineImageTaskPNG(encoded string) ([]byte, *InlineImageTaskResult, error) {
	if len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(int(MaxInlineImageTaskBytes)) {
		return nil, nil, fmt.Errorf("inline image exceeds the reviewed size limit")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || base64.StdEncoding.EncodeToString(data) != encoded {
		return nil, nil, fmt.Errorf("inline image must use canonical standard base64")
	}
	if int64(len(data)) > MaxInlineImageTaskBytes {
		return nil, nil, fmt.Errorf("inline image exceeds the reviewed size limit")
	}
	width, height, err := validateInlineImageTaskPNG(data)
	if err != nil {
		return nil, nil, err
	}
	return data, &InlineImageTaskResult{Width: width, Height: height, Bytes: int64(len(data)), SHA256: sha256.Sum256(data)}, nil
}

func validateInlineImageTaskPNG(data []byte) (int, int, error) {
	if len(data) < 33 || int64(len(data)) > MaxInlineImageTaskBytes || !bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return 0, 0, fmt.Errorf("inline image must be a complete PNG")
	}
	offset := uint64(8)
	foundIDAT := false
	ended := false
	for offset < uint64(len(data)) {
		if uint64(len(data))-offset < 12 {
			return 0, 0, fmt.Errorf("inline PNG chunk is truncated")
		}
		length := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		end := offset + 12 + length
		if end > uint64(len(data)) {
			return 0, 0, fmt.Errorf("inline PNG chunk is truncated")
		}
		kind := string(data[offset+4 : offset+8])
		if offset == 8 && (kind != "IHDR" || length != 13) || offset != 8 && kind == "IHDR" {
			return 0, 0, fmt.Errorf("inline PNG header is invalid")
		}
		switch kind {
		case "acTL", "fcTL", "fdAT":
			return 0, 0, fmt.Errorf("inline image must contain one still PNG")
		case "IDAT":
			foundIDAT = true
		case "IEND":
			if length != 0 || !foundIDAT || end != uint64(len(data)) {
				return 0, 0, fmt.Errorf("inline PNG must end exactly at IEND")
			}
			ended = true
		}
		offset = end
	}
	if !ended {
		return 0, 0, fmt.Errorf("inline PNG has no complete IEND")
	}
	width, height := uint64(binary.BigEndian.Uint32(data[16:20])), uint64(binary.BigEndian.Uint32(data[20:24]))
	if width == 0 || height == 0 || width > 23170 || height > 23170 || width > 40_000_000/height || width <= height {
		return 0, 0, fmt.Errorf("inline image geometry is outside the reviewed landscape bounds")
	}
	// Match standard png decoder image allocations conservatively. Interlaced
	// images can also allocate a pass image; row buffers are included separately.
	depth, kind, interlace := data[24], data[25], data[28]
	var bytesPerPixel, channels uint64
	switch kind {
	case 0:
		// A legal tRNS chunk makes grayscale decode into NRGBA/NRGBA64.
		// Budget for that expansion even when this particular PNG is opaque.
		bytesPerPixel = 4
		channels = 1
	case 2:
		bytesPerPixel = 4
		channels = 3
	case 3:
		bytesPerPixel = 1
		channels = 1
	case 4:
		bytesPerPixel = 4
		channels = 2
	case 6:
		bytesPerPixel = 4
		channels = 4
	default:
		return 0, 0, fmt.Errorf("inline PNG color type is unsupported")
	}
	if depth == 16 {
		bytesPerPixel *= 2
	}
	decoded := width * height * bytesPerPixel
	if interlace == 1 {
		decoded *= 2
	}
	rowBytes := 1 + (width*channels*uint64(depth)+7)/8
	if decoded+2*rowBytes+2048 > inlineImageTaskMaxDecodedBytes {
		return 0, 0, fmt.Errorf("inline PNG decoded image exceeds the reviewed memory limit")
	}
	decodedImage, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("inline PNG could not be completely decoded: %w", err)
	}
	if decodedImage.Bounds().Dx() != int(width) || decodedImage.Bounds().Dy() != int(height) {
		return 0, 0, fmt.Errorf("inline PNG dimensions are inconsistent")
	}
	return int(width), int(height), nil
}
