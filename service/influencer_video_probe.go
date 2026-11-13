package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const influencerVideoProbeOutputBytes = 2 * 1024 * 1024

// These two MOV notices describe a missing pre-zero AAC index entry; all other
// diagnostics, including skipped external tracks, remain fail-closed.
var influencerVideoAACPrerollWarning = regexp.MustCompile(`^\[mov[^]]* @[^]]*\] st: ([0-7]) edit list:? 1 (?:Missing key frame while searching for timestamp: 0|Cannot find an index entry before timestamp: 0)$`)

var influencerVideoProbeSlots = make(chan struct{}, 2)

// Limited process output prevents user media from growing unbounded buffers.
type influencerVideoProbeBuffer struct {
	bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

// Do not inherit bytes.Buffer.ReadFrom: os/exec's io.Copy would bypass Write
// and its hard output limit via the promoted ReaderFrom fast path.
func (buffer *influencerVideoProbeBuffer) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{buffer}, reader)
}

func (buffer *influencerVideoProbeBuffer) Write(data []byte) (int, error) {
	if len(data) > buffer.limit-buffer.Len() {
		buffer.exceeded = true
		if buffer.cancel != nil {
			buffer.cancel()
		}
		return 0, errors.New("media probe output limit exceeded")
	}
	return buffer.Buffer.Write(data)
}

type influencerVideoProbeEvent struct {
	Type           string `json:"type"`
	Kind           string `json:"media_type"`
	Index          int    `json:"stream_index"`
	PTS            *int64 `json:"pts"`
	BestPTS        *int64 `json:"best_effort_timestamp"`
	Duration       *int64 `json:"duration"`
	PacketDuration *int64 `json:"pkt_duration"`
	Samples        *int64 `json:"nb_samples"`
	Flags          string `json:"flags"`
	SideData       []struct {
		Type    string `json:"side_data_type"`
		Skip    int64  `json:"skip_samples"`
		Discard int64  `json:"discard_padding"`
	} `json:"side_data_list"`
}

type influencerVideoProbeResult struct {
	Streams []struct {
		Index         int    `json:"index"`
		Kind          string `json:"codec_type"`
		Codec         string `json:"codec_name"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		FrameRate     string `json:"avg_frame_rate"`
		Duration      string `json:"duration"`
		DecodedCount  string `json:"nb_read_frames"`
		DeclaredCount string `json:"nb_frames"`
		Aspect        string `json:"sample_aspect_ratio"`
		TimeBase      string `json:"time_base"`
		SampleRate    string `json:"sample_rate"`
		StartPTS      *int64 `json:"start_pts"`
		Disposition   struct {
			AttachedPicture int `json:"attached_pic"`
		} `json:"disposition"`
		SideData []struct {
			Rotation int `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
	Packets []influencerVideoProbeEvent `json:"packets"`
	Frames  []influencerVideoProbeEvent `json:"frames"`
	Events  []influencerVideoProbeEvent `json:"packets_and_frames"`
	Format  struct {
		Name     string `json:"format_name"`
		Start    string `json:"start_time"`
		Duration string `json:"duration"`
		Tags     struct {
			Brand string `json:"major_brand"`
		} `json:"tags"`
	} `json:"format"`
}

// ProbeInfluencerVideoFile only reads a task-owned local copy. The cache wrapper
// supplies seek support without granting the media process file or network protocols.
func ProbeInfluencerVideoFile(parent context.Context, path string) (InfluencerVideoMetadata, error) {
	var empty InfluencerVideoMetadata
	if err := parent.Err(); err != nil {
		return empty, err
	}
	select {
	case influencerVideoProbeSlots <- struct{}{}:
		defer func() { <-influencerVideoProbeSlots }()
	default:
		return empty, ErrInfluencerVideoBusy
	}
	return probeInfluencerVideoFile(parent, path)
}

func probeInfluencerVideoFile(parent context.Context, path string) (InfluencerVideoMetadata, error) {
	var empty InfluencerVideoMetadata
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	file, err := os.Open(path)
	if err != nil {
		return empty, ErrInfluencerVideoUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return empty, ErrInfluencerVideoUnavailable
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > InfluencerVideoMaxBytes {
		return empty, errors.New("参考视频内容超过接收上限或文件无效")
	}
	if err := influencerVideoMOVEdits(ctx, file, info.Size(), 0, info.Size(), 0, new(int)); err != nil {
		return empty, err
	}
	header, err := probeInfluencerVideoStream(ctx, file, false)
	if err != nil {
		return empty, err
	}
	media, err := influencerVideoMetadataFromProbe(header, false)
	if err != nil {
		return empty, err
	}
	media.SizeBytes = info.Size()
	if err := ValidateInfluencerVideoMetadata(media); err != nil {
		return empty, err
	}
	full, err := probeInfluencerVideoStream(ctx, file, true)
	if err != nil {
		return empty, err
	}
	media, err = influencerVideoMetadataFromProbe(full, true)
	if err != nil {
		return empty, err
	}
	media.SizeBytes = info.Size()
	if err := ValidateInfluencerVideoMetadata(media); err != nil {
		return empty, err
	}
	return media, nil
}

// probeInfluencerVideoStream runs one bounded ffprobe pass over an opened,
// seekable media file. full selects the decode-everything entries used by the
// input acceptance gate; header-only passes reuse the same command and parser,
// so a generated-output probe reads headers without decoding the timeline.
func probeInfluencerVideoStream(ctx context.Context, file *os.File, full bool) (influencerVideoProbeResult, error) {
	var result influencerVideoProbeResult
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return result, ErrInfluencerVideoUnavailable
	}
	tool, err := exec.LookPath("ffprobe")
	if err != nil {
		return result, ErrInfluencerVideoUnavailable
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// cache in the deployed 5.1 build uses an unlinked /tmp file; TMPDIR
	// does not isolate it. max_alloc caps one allocation, not process RSS.
	args := []string{"-v", "warning", "-threads", "1", "-flags2", "+skip_manual", "-max_alloc", "16777216", "-protocol_whitelist", "cache,pipe", "-format_whitelist", "mov", "-read_ahead_limit", strconv.FormatInt(InfluencerVideoMaxBytes, 10), "-enable_drefs", "0", "-use_absolute_path", "0"}
	if full {
		args = append(args, "-count_frames", "-show_packets", "-show_frames")
	}
	args = append(args, "-show_entries", "packet=stream_index,pts,duration,flags:packet_side_data=side_data_type,skip_samples,discard_padding:frame=media_type,stream_index,pts,best_effort_timestamp,duration,pkt_duration,nb_samples:stream=index,codec_type,codec_name,width,height,avg_frame_rate,duration,nb_frames,nb_read_frames,sample_aspect_ratio,time_base,sample_rate,start_pts:stream_disposition=attached_pic:stream_side_data=rotation:format=duration,format_name,start_time:format_tags=major_brand", "-of", "json", "cache:pipe:0")
	command := exec.CommandContext(probeCtx, tool, args...)
	command.Stdin = io.LimitReader(file, InfluencerVideoMaxBytes+1)
	stdout := &influencerVideoProbeBuffer{limit: influencerVideoProbeOutputBytes, cancel: cancel}
	stderr := &influencerVideoProbeBuffer{limit: 16 * 1024, cancel: cancel}
	command.WaitDelay = 2 * time.Second
	command.Stdout, command.Stderr = stdout, stderr
	runErr := command.Run()
	if stdout.exceeded || stderr.exceeded {
		return result, ErrInfluencerVideoOutputLimit
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if runErr != nil {
		return result, errors.New("无法完整读取视频，请检查文件")
	}
	if err := common.Unmarshal(stdout.Bytes(), &result); err != nil {
		return result, errors.New("视频检查结果无效")
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if line == "" || (strings.HasPrefix(line, "[cache @") && strings.Contains(line, "Inner protocol failed to seekback end")) {
			continue
		}
		match := influencerVideoAACPrerollWarning.FindStringSubmatch(line)
		allowed := false
		if len(match) == 2 {
			index, _ := strconv.Atoi(match[1])
			for _, stream := range result.Streams {
				if stream.Index == index && stream.Kind == "audio" && stream.Codec == "aac" {
					allowed = true
				}
			}
		}
		if !allowed {
			return result, errors.New("无法完整读取视频，请检查文件")
		}
	}
	return result, nil
}

// influencerVideoOutputProbeMaxBytes mirrors the task artifact buffering bound:
// a generated video larger than this is never held in memory, so it never
// reaches this probe. The bound keeps temp-file and parse cost bounded.
const influencerVideoOutputProbeMaxBytes int64 = 32 << 20

// ProbeInfluencerVideoPayload reads media facts from bytes the server already
// holds for a generated output video. It is header-only (no full decode), never
// downloads anything, and is deliberately not part of reference-input
// acceptance: callers must only pass server-generated output bytes.
func ProbeInfluencerVideoPayload(parent context.Context, data []byte) (InfluencerVideoMetadata, error) {
	return probeInfluencerVideoPayload(parent, data, influencerVideoOutputProbeMaxBytes)
}

func probeInfluencerVideoPayload(parent context.Context, data []byte, maxBytes int64) (InfluencerVideoMetadata, error) {
	var empty InfluencerVideoMetadata
	if err := parent.Err(); err != nil {
		return empty, err
	}
	if len(data) == 0 || int64(len(data)) > maxBytes {
		return empty, errors.New("输出视频字节超过检查上限")
	}
	select {
	case influencerVideoProbeSlots <- struct{}{}:
		defer func() { <-influencerVideoProbeSlots }()
	default:
		return empty, ErrInfluencerVideoBusy
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	file, err := os.CreateTemp("", "influencer-output-probe-*")
	if err != nil {
		return empty, ErrInfluencerVideoUnavailable
	}
	path := file.Name()
	defer func() {
		file.Close()
		os.Remove(path)
	}()
	if _, err := file.Write(data); err != nil {
		return empty, ErrInfluencerVideoUnavailable
	}
	header, err := probeInfluencerVideoStream(ctx, file, false)
	if err != nil {
		return empty, err
	}
	media, err := influencerVideoMetadataFromProbe(header, false)
	if err != nil {
		return empty, err
	}
	if media.Duration <= 0 || media.Width <= 0 || media.Height <= 0 || media.FrameRate <= 0 {
		return empty, errors.New("输出视频媒体信息不完整")
	}
	media.SizeBytes = int64(len(data))
	// A header-only pass has no decoded timeline. Mirror the container duration
	// as the best-effort picture span; consumers still validate before reuse.
	media.VideoDuration = media.Duration
	return media, nil
}

func influencerVideoFiniteNumber(value string) (float64, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, errors.New("invalid media number")
	}
	return number, nil
}

func influencerVideoMetadataFromProbe(result influencerVideoProbeResult, full bool) (InfluencerVideoMetadata, error) {
	var media InfluencerVideoMetadata
	if len(result.Streams) < 1 || len(result.Streams) > 2 || !strings.Contains(result.Format.Name, "mp4") {
		return media, errors.New("参考视频必须为单画面MP4或MOV")
	}
	media.Container = "mp4"
	if strings.TrimSpace(result.Format.Tags.Brand) == "qt" {
		media.Container = "mov"
	}
	duration, err := influencerVideoFiniteNumber(result.Format.Duration)
	if err != nil {
		return media, errors.New("无法读取视频真实时长")
	}
	media.Duration = duration
	videoCount, audioCount := 0, 0
	streamIndexes := make(map[int]*big.Rat)
	for _, stream := range result.Streams {
		base, err := influencerVideoTimeBase(stream.TimeBase)
		if err != nil {
			return media, errors.New("视频时间基无效")
		}
		if stream.Index < 0 || stream.Index > 7 || streamIndexes[stream.Index] != nil {
			return media, errors.New("视频数据流无效")
		}
		streamIndexes[stream.Index] = base
		switch stream.Kind {
		case "video":
			videoCount++
			if stream.Disposition.AttachedPicture != 0 {
				return media, errors.New("参考素材没有有效视频画面")
			}
			media.Width, media.Height, media.VideoCodec = stream.Width, stream.Height, stream.Codec
			if stream.Aspect != "" && stream.Aspect != "1:1" {
				return media, errors.New("参考视频像素比例不受支持")
			}
			for _, side := range stream.SideData {
				if side.Rotation%90 != 0 {
					return media, errors.New("参考视频旋转参数不受支持")
				}
				if side.Rotation%180 != 0 {
					media.Width, media.Height = media.Height, media.Width
				}
			}
			parts := strings.Split(stream.FrameRate, "/")
			if len(parts) != 2 {
				return media, errors.New("无法读取视频帧率")
			}
			numerator, e1 := influencerVideoFiniteNumber(parts[0])
			denominator, e2 := influencerVideoFiniteNumber(parts[1])
			if e1 != nil || e2 != nil || denominator <= 0 {
				return media, errors.New("无法读取视频帧率")
			}
			media.FrameRate = numerator / denominator
		case "audio":
			audioCount++
			media.AudioCodec = stream.Codec
		default:
			return media, errors.New("参考视频包含不受支持的数据流")
		}
		if stream.Duration != "" {
			value, err := influencerVideoFiniteNumber(stream.Duration)
			if err != nil || value <= 0 {
				return media, errors.New("视频时间线无效")
			}
			media.Duration = math.Max(media.Duration, value)
		}
		if full && stream.Kind == "video" {
			decoded, err := strconv.Atoi(stream.DecodedCount)
			if err != nil || decoded <= 0 || decoded > 1800 {
				return media, errors.New("无法完整解码视频")
			}
			if stream.DeclaredCount != "" && stream.DeclaredCount != "N/A" {
				declared, err := strconv.Atoi(stream.DeclaredCount)
				if err != nil || declared != decoded {
					return media, errors.New("视频帧数不完整")
				}
			}
		}
	}
	if videoCount != 1 || audioCount > 1 {
		return media, errors.New("参考视频必须包含一条有效视频画面")
	}
	if !full {
		return media, nil
	}
	if len(result.Events) > 0 {
		if len(result.Packets) > 0 || len(result.Frames) > 0 {
			return media, errors.New("视频时间线结构不明确")
		}
		for _, event := range result.Events {
			switch event.Type {
			case "packet":
				result.Packets = append(result.Packets, event)
			case "frame":
				result.Frames = append(result.Frames, event)
			default:
				return media, errors.New("视频时间线结构无效")
			}
		}
	}
	if len(result.Frames) == 0 {
		return media, errors.New("视频没有完整解码画面")
	}
	return influencerVideoDecodedTimeline(result, media, streamIndexes)
}

// Media integer ratios have small bounded components; adversarial decimal/exponent
// strings never reach arbitrary-precision parsing.
func influencerVideoTimeBase(value string) (*big.Rat, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return nil, errors.New("invalid time base")
	}
	n, e1 := strconv.ParseInt(parts[0], 10, 64)
	d, e2 := strconv.ParseInt(parts[1], 10, 64)
	if e1 != nil || e2 != nil || n <= 0 || d <= 0 || n > 1000000000 || d > 1000000000 {
		return nil, errors.New("invalid time base")
	}
	return big.NewRat(n, d), nil
}

func influencerVideoDecodedTimeline(result influencerVideoProbeResult, media InfluencerVideoMetadata, bases map[int]*big.Rat) (InfluencerVideoMetadata, error) {
	invalid := errors.New("视频呈现时间线不完整或不受支持")
	if len(result.Packets) < 1 || len(result.Packets) > 12000 || len(result.Frames) > 12000 {
		return media, invalid
	}
	if result.Format.Start != "" {
		// ffprobe emits fixed decimal seconds. Reject exponents and bound the
		// string before exact parsing, including values that float64 rounds to zero.
		value := result.Format.Start
		if len(value) > 32 || strings.Trim(value, "+-0123456789.") != "" {
			return media, invalid
		}
		start, ok := new(big.Rat).SetString(value)
		if !ok || start.Sign() != 0 {
			return media, invalid
		}
	}
	media.Duration = 0
	for _, stream := range result.Streams {
		if stream.StartPTS != nil && *stream.StartPTS != 0 {
			return media, invalid
		}
		base := bases[stream.Index]
		var packets, frames []influencerVideoProbeEvent
		for _, event := range result.Packets {
			if bases[event.Index] == nil {
				return media, invalid
			}
			if event.Index == stream.Index {
				packets = append(packets, event)
			}
		}
		for _, event := range result.Frames {
			if bases[event.Index] == nil {
				return media, invalid
			}
			if event.Index == stream.Index {
				frames = append(frames, event)
			}
		}
		if len(packets) == 0 || len(frames) == 0 {
			return media, invalid
		}
		counts, err := strconv.Atoi(stream.DecodedCount)
		if err != nil || counts != len(frames) {
			return media, invalid
		}
		packetByPTS := make(map[int64]influencerVideoProbeEvent, len(packets))
		var skip, discard int64
		for i, packet := range packets {
			if packet.PTS == nil || packet.Duration == nil || *packet.Duration <= 0 || *packet.PTS < -1000000000000 || *packet.PTS > 1000000000000 || *packet.Duration > 1000000000000 {
				return media, invalid
			}
			if _, exists := packetByPTS[*packet.PTS]; exists {
				return media, invalid
			}
			packetByPTS[*packet.PTS] = packet
			for _, side := range packet.SideData {
				if side.Type != "Skip Samples" || side.Skip < 0 || side.Discard < 0 || side.Skip > 65536 || side.Discard > 65536 || (side.Skip > 0 && (i != 0 || skip != 0)) || (side.Discard > 0 && (i != len(packets)-1 || discard != 0)) {
					return media, invalid
				}
				skip += side.Skip
				discard += side.Discard
			}
			if stream.Kind == "video" {
				if *packet.PTS < 0 || strings.ContainsAny(packet.Flags, "DC") || len(packet.SideData) > 0 {
					return media, invalid
				}
				end := new(big.Rat).Mul(new(big.Rat).SetInt(new(big.Int).Add(big.NewInt(*packet.PTS), big.NewInt(*packet.Duration))), base)
				if end.Cmp(big.NewRat(30, 1)) > 0 {
					return media, errors.New("视频实际时长超过30秒，不会自动剪短")
				}
			} else if i > 0 && *packet.PTS <= *packets[i-1].PTS {
				return media, invalid
			}
		}
		var rate int64
		if stream.Kind == "audio" {
			rate, err = strconv.ParseInt(stream.SampleRate, 10, 64)
			if err != nil || rate < 8000 || rate > 192000 || base.Cmp(big.NewRat(1, rate)) != 0 || len(frames) != len(packets) {
				return media, invalid
			}
			if skip > 0 && (*packets[0].PTS >= 0 || *packets[0].PTS != -skip) {
				return media, invalid
			}
		}
		var first, previousPTS, previousEnd, maximum *big.Rat
		var audioRounding int64
		for i, frame := range frames {
			if frame.Kind != stream.Kind || frame.PTS == nil || frame.BestPTS == nil || *frame.PTS != *frame.BestPTS || *frame.PTS < -1000000000000 || *frame.PTS > 1000000000000 {
				return media, invalid
			}
			length := frame.Duration
			if length == nil {
				length = frame.PacketDuration
			}
			if length == nil || *length <= 0 || *length > 1000000000000 || (frame.Duration != nil && frame.PacketDuration != nil && *frame.Duration != *frame.PacketDuration) {
				return media, invalid
			}
			packet, found := packetByPTS[*frame.PTS]
			if !found {
				return media, invalid
			}
			position := new(big.Rat).Mul(new(big.Rat).SetInt64(*frame.PTS), base)
			rawPosition := new(big.Rat).Set(position)
			duration := new(big.Rat).Mul(new(big.Rat).SetInt64(*length), base)
			if stream.Kind == "video" {
				if position.Sign() < 0 || *packet.Duration != *length {
					return media, invalid
				}
			} else {
				if frame.Samples == nil || *frame.Samples <= 0 || *frame.Samples > 65536 || *packet.Duration != *length {
					return media, invalid
				}
				if i != len(frames)-1 {
					// AAC container timestamps can quantize a decoded block by one
					// sample. Preserve their exact span, with no cumulative drift.
					delta := *length - *frame.Samples
					if delta != 0 && (stream.Codec != "aac" || delta < -1 || delta > 1 || skip > 0) {
						return media, invalid
					}
					audioRounding += delta
					if audioRounding < -1 || audioRounding > 1 {
						return media, invalid
					}
				} else if *length > *frame.Samples {
					return media, invalid
				}
				// Container tail duration and decoder padding are two upper
				// bounds on the same block, not two sequential sample cuts.
				samples := *length
				if i == len(frames)-1 {
					if discard > *frame.Samples {
						return media, invalid
					}
					samples = min(samples, *frame.Samples-discard)
				}
				dropped := min(skip, *frame.Samples)
				if dropped > samples {
					return media, invalid
				}
				skip -= dropped
				samples -= dropped
				position.Add(position, big.NewRat(dropped, rate))
				if strings.Contains(packet.Flags, "C") || (strings.Contains(packet.Flags, "D") && samples != 0) {
					return media, invalid
				}
				duration = big.NewRat(samples, rate)
			}
			if previousPTS != nil && rawPosition.Cmp(previousPTS) <= 0 {
				return media, invalid
			}
			previousPTS = rawPosition
			if duration.Sign() == 0 {
				continue
			}
			end := new(big.Rat).Add(position, duration)
			if position.Sign() < 0 || (previousEnd != nil && position.Cmp(previousEnd) < 0) {
				return media, invalid
			}
			if stream.Kind == "audio" && previousEnd != nil && position.Cmp(previousEnd) != 0 {
				return media, invalid
			}
			if end.Cmp(big.NewRat(30, 1)) > 0 {
				return media, errors.New("视频实际时长超过30秒，不会自动剪短")
			}
			if first == nil {
				first = new(big.Rat).Set(position)
			}
			maximum, previousEnd = end, end
		}
		if first == nil || maximum == nil || skip != 0 {
			return media, invalid
		}
		if stream.Kind == "video" {
			span := new(big.Rat).Sub(maximum, first)
			if span.Cmp(big.NewRat(2, 1)) < 0 {
				return media, errors.New("视频实际画面时长必须在2至30秒之间")
			}
			media.VideoDuration, _ = span.Float64()
		}
		seconds, _ := maximum.Float64()
		media.Duration = math.Max(media.Duration, seconds)
	}
	return media, nil
}

// This is a bounded atom-envelope check, not a second media decoder. Unsupported
// edits cannot silently turn a decoded timeline into an uncertain presentation.
func influencerVideoMOVEdits(ctx context.Context, file *os.File, size, start, end int64, depth int, count *int) error {
	invalid := errors.New("视频编辑时间线不受支持")
	if depth > 8 || end > size {
		return invalid
	}
	for start < end {
		if err := ctx.Err(); err != nil {
			return err
		}
		*count++
		if *count > 4096 || end-start < 8 {
			return invalid
		}
		var header [16]byte
		if _, err := file.ReadAt(header[:8], start); err != nil {
			return invalid
		}
		length := int64(binary.BigEndian.Uint32(header[:4]))
		headerSize := int64(8)
		if length == 1 {
			if end-start < 16 {
				return invalid
			}
			if _, err := file.ReadAt(header[8:], start+8); err != nil {
				return invalid
			}
			n := binary.BigEndian.Uint64(header[8:])
			if n > uint64(end-start) {
				return invalid
			}
			length, headerSize = int64(n), 16
		} else if length == 0 {
			length = end - start
		}
		if length < headerSize || length > end-start {
			return invalid
		}
		body := start + headerSize
		switch string(header[4:8]) {
		case "moov", "trak", "edts", "mdia", "minf", "dinf":
			if err := influencerVideoMOVEdits(ctx, file, size, body, start+length, depth+1, count); err != nil {
				return err
			}
		case "dref":
			// Only one self-contained URL reference is supported. The size is
			// fixed, so aliases, URLs and extra external entries fail before decode.
			if length-headerSize != 20 {
				return invalid
			}
			var data [20]byte
			if _, err := file.ReadAt(data[:], body); err != nil {
				return invalid
			}
			if binary.BigEndian.Uint32(data[:4]) != 0 || binary.BigEndian.Uint32(data[4:8]) != 1 || binary.BigEndian.Uint32(data[8:12]) != 12 || string(data[12:16]) != "url " || binary.BigEndian.Uint32(data[16:20]) != 1 {
				return invalid
			}
		case "elst":
			if length-headerSize != 20 && length-headerSize != 28 {
				return invalid
			}
			data := make([]byte, length-headerSize)
			if _, err := file.ReadAt(data, body); err != nil {
				return invalid
			}
			if data[1] != 0 || data[2] != 0 || data[3] != 0 || binary.BigEndian.Uint32(data[4:8]) != 1 {
				return invalid
			}
			var mediaTime int64
			if data[0] == 0 && len(data) == 20 {
				mediaTime = int64(int32(binary.BigEndian.Uint32(data[12:16])))
			} else if data[0] == 1 && len(data) == 28 {
				mediaTime = int64(binary.BigEndian.Uint64(data[16:24]))
			} else {
				return invalid
			}
			if mediaTime < 0 || mediaTime > 1000000000 || binary.BigEndian.Uint32(data[len(data)-4:]) != 65536 {
				return invalid
			}
		case "moof", "mvex":
			return invalid
		}
		start += length
	}
	return nil
}
