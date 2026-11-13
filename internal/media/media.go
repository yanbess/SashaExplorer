// Package media describes the content of video, audio, and picture files for
// the preview panel: streams, codecs, frame rates, and tags. It asks ffprobe
// when it is installed, and the caller falls back to what Windows knows.
package media

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Info is what a media file holds. Zero values are unknown.
type Info struct {
	// Source is "ffprobe" or "windows".
	Source string `json:"source"`
	// Format is the container, such as "QuickTime / MOV" or "Matroska / WebM".
	Format string `json:"format"`
	// Duration is in seconds.
	Duration float64 `json:"duration"`
	// Bitrate is the overall rate in bits per second.
	Bitrate   int64      `json:"bitrate"`
	Video     []Video    `json:"video"`
	Audio     []Audio    `json:"audio"`
	Subtitles []Subtitle `json:"subtitles"`
	// Tags are the title, artist, album and the like, by lower case name.
	Tags  map[string]string `json:"tags"`
	Photo *Photo            `json:"photo"`
}

// Video is a video stream.
type Video struct {
	Codec       string  `json:"codec"`
	Profile     string  `json:"profile"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	Bitrate     int64   `json:"bitrate"`
	PixelFormat string  `json:"pixelFormat"`
	BitDepth    int     `json:"bitDepth"`
	// HDR is "HDR10", "HLG", or "" for standard range.
	HDR      string `json:"hdr"`
	Aspect   string `json:"aspect"`
	Scan     string `json:"scan"`
	Rotation int    `json:"rotation"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// Audio is an audio stream.
type Audio struct {
	Codec      string `json:"codec"`
	Profile    string `json:"profile"`
	Channels   int    `json:"channels"`
	Layout     string `json:"layout"`
	SampleRate int    `json:"sampleRate"`
	Bitrate    int64  `json:"bitrate"`
	BitDepth   int    `json:"bitDepth"`
	Language   string `json:"language"`
	Title      string `json:"title"`
}

// Subtitle is a subtitle stream.
type Subtitle struct {
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
}

// Photo is what a camera wrote into a picture.
type Photo struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	BitDepth     int     `json:"bitDepth"`
	DPI          float64 `json:"dpi"`
	Camera       string  `json:"camera"`
	Lens         string  `json:"lens"`
	Taken        int64   `json:"taken"`
	Exposure     float64 `json:"exposure"`
	FNumber      float64 `json:"fNumber"`
	ISO          int     `json:"iso"`
	FocalLength  float64 `json:"focalLength"`
	Flash        bool    `json:"flash"`
	ColorProfile string  `json:"colorProfile"`
}

var probePath = sync.OnceValue(func() string {
	if exe, err := os.Executable(); err == nil {
		local := filepath.Join(filepath.Dir(exe), "ffprobe.exe")
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	path, _ := exec.LookPath("ffprobe")
	return path
})

// ErrNoProbe means ffprobe is not installed.
var ErrNoProbe = errors.New("ffprobe is not installed")

// HaveProbe reports whether ffprobe is next to the program or on the PATH.
func HaveProbe() bool {
	return probePath() != ""
}

// Probe asks ffprobe about a file.
func Probe(ctx context.Context, path string) (Info, error) {
	exe := probePath()
	if exe == "" {
		return Info{}, ErrNoProbe
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, exe,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", "--", path)
	// ffprobe is a console program: without this a console window flashes.
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := command.Output()
	if err != nil {
		return Info{}, err
	}
	return ParseProbe(output)
}

type probeOutput struct {
	Streams []probeStream `json:"streams"`
	Format  struct {
		LongName string            `json:"format_long_name"`
		Duration string            `json:"duration"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

type probeStream struct {
	Type          string            `json:"codec_type"`
	Codec         string            `json:"codec_name"`
	CodecTag      string            `json:"codec_tag_string"`
	Profile       string            `json:"profile"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	AvgFrameRate  string            `json:"avg_frame_rate"`
	RealFrameRate string            `json:"r_frame_rate"`
	BitRate       string            `json:"bit_rate"`
	PixelFormat   string            `json:"pix_fmt"`
	BitsRaw       string            `json:"bits_per_raw_sample"`
	BitsSample    int               `json:"bits_per_sample"`
	SampleFormat  string            `json:"sample_fmt"`
	Transfer      string            `json:"color_transfer"`
	Aspect        string            `json:"display_aspect_ratio"`
	FieldOrder    string            `json:"field_order"`
	Channels      int               `json:"channels"`
	Layout        string            `json:"channel_layout"`
	SampleRate    string            `json:"sample_rate"`
	Disposition   map[string]int    `json:"disposition"`
	Tags          map[string]string `json:"tags"`
	SideData      []struct {
		Rotation float64 `json:"rotation"`
	} `json:"side_data_list"`
}

// ParseProbe reads the JSON of ffprobe -show_format -show_streams.
func ParseProbe(output []byte) (Info, error) {
	var probe probeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return Info{}, err
	}
	info := Info{
		Source:   "ffprobe",
		Format:   probe.Format.LongName,
		Duration: parseFloat(probe.Format.Duration),
		Bitrate:  parseInt(probe.Format.BitRate),
		Tags:     pickTags(probe.Format.Tags),
	}
	for _, stream := range probe.Streams {
		tags := lowerKeys(stream.Tags)
		switch stream.Type {
		case "video":
			// Cover art of a song is a video stream of one picture.
			if stream.Disposition["attached_pic"] == 1 {
				continue
			}
			video := Video{
				Codec:       CodecName(stream.Codec),
				Profile:     stream.Profile,
				Width:       stream.Width,
				Height:      stream.Height,
				FPS:         frameRate(stream.AvgFrameRate),
				Bitrate:     streamBitrate(stream.BitRate, tags),
				PixelFormat: stream.PixelFormat,
				BitDepth:    videoBitDepth(stream),
				HDR:         hdrName(stream.Transfer),
				Aspect:      stream.Aspect,
				Scan:        scanName(stream.FieldOrder),
				Language:    language(tags),
				Title:       tags["title"],
			}
			if video.FPS == 0 {
				video.FPS = frameRate(stream.RealFrameRate)
			}
			if video.Aspect == "0:1" || video.Aspect == "N/A" {
				video.Aspect = ""
			}
			for _, side := range stream.SideData {
				if side.Rotation != 0 {
					video.Rotation = int(side.Rotation)
				}
			}
			info.Video = append(info.Video, video)
		case "audio":
			info.Audio = append(info.Audio, Audio{
				Codec:      CodecName(stream.Codec),
				Profile:    stream.Profile,
				Channels:   stream.Channels,
				Layout:     stream.Layout,
				SampleRate: int(parseInt(stream.SampleRate)),
				Bitrate:    streamBitrate(stream.BitRate, tags),
				BitDepth:   audioBitDepth(stream),
				Language:   language(tags),
				Title:      tags["title"],
			})
		case "subtitle":
			info.Subtitles = append(info.Subtitles, Subtitle{
				Codec:    CodecName(stream.Codec),
				Language: language(tags),
				Title:    tags["title"],
			})
		}
	}
	return info, nil
}

var codecNames = map[string]string{
	"h264":              "H.264 (AVC)",
	"hevc":              "H.265 (HEVC)",
	"av1":               "AV1",
	"vp9":               "VP9",
	"vp8":               "VP8",
	"mpeg4":             "MPEG-4 Part 2",
	"mpeg2video":        "MPEG-2",
	"mpeg1video":        "MPEG-1",
	"mjpeg":             "Motion JPEG",
	"prores":            "Apple ProRes",
	"dnxhd":             "Avid DNxHD",
	"wmv3":              "Windows Media Video 9",
	"vc1":               "VC-1",
	"theora":            "Theora",
	"gif":               "GIF",
	"aac":               "AAC",
	"mp3":               "MP3",
	"mp2":               "MP2",
	"opus":              "Opus",
	"vorbis":            "Vorbis",
	"flac":              "FLAC",
	"alac":              "Apple Lossless",
	"ac3":               "Dolby Digital (AC-3)",
	"eac3":              "Dolby Digital Plus (E-AC-3)",
	"truehd":            "Dolby TrueHD",
	"dts":               "DTS",
	"wmav2":             "Windows Media Audio",
	"wmapro":            "Windows Media Audio Pro",
	"amr_nb":            "AMR",
	"subrip":            "SubRip (SRT)",
	"ass":               "ASS",
	"mov_text":          "MPEG-4 Timed Text",
	"webvtt":            "WebVTT",
	"hdmv_pgs_subtitle": "PGS",
	"dvd_subtitle":      "VobSub",
}

// CodecName turns an ffprobe codec name into the one people know.
func CodecName(codec string) string {
	if name, ok := codecNames[codec]; ok {
		return name
	}
	if rest, ok := strings.CutPrefix(codec, "pcm_"); ok {
		return "PCM (" + rest + ")"
	}
	return strings.ToUpper(codec)
}

func parseFloat(text string) float64 {
	value, _ := strconv.ParseFloat(strings.TrimSpace(text), 64)
	return value
}

func parseInt(text string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	return value
}

// frameRate reads "30000/1001".
func frameRate(text string) float64 {
	num, den, ok := strings.Cut(text, "/")
	if !ok {
		return parseFloat(text)
	}
	d := parseFloat(den)
	if d == 0 {
		return 0
	}
	return parseFloat(num) / d
}

// streamBitrate takes the rate of a stream, or the one Matroska keeps in its
// tags, such as BPS or BPS-eng.
func streamBitrate(text string, tags map[string]string) int64 {
	if rate := parseInt(text); rate > 0 {
		return rate
	}
	for key, value := range tags {
		if key == "bps" || strings.HasPrefix(key, "bps-") {
			if rate := parseInt(value); rate > 0 {
				return rate
			}
		}
	}
	return 0
}

func videoBitDepth(stream probeStream) int {
	if bits := int(parseInt(stream.BitsRaw)); bits > 0 {
		return bits
	}
	format := stream.PixelFormat
	for _, depth := range []string{"16", "14", "12", "10", "9"} {
		if strings.Contains(format, "p"+depth) {
			n, _ := strconv.Atoi(depth)
			return n
		}
	}
	if format != "" {
		return 8
	}
	return 0
}

func audioBitDepth(stream probeStream) int {
	if bits := int(parseInt(stream.BitsRaw)); bits > 0 {
		return bits
	}
	if stream.BitsSample > 0 {
		return stream.BitsSample
	}
	// Lossy codecs have no bit depth; only exact sample formats tell one.
	if strings.HasPrefix(stream.Codec, "pcm_") || stream.Codec == "flac" || stream.Codec == "alac" {
		switch strings.TrimSuffix(stream.SampleFormat, "p") {
		case "s16":
			return 16
		case "s32":
			return 32
		case "u8":
			return 8
		}
	}
	return 0
}

func hdrName(transfer string) string {
	switch transfer {
	case "smpte2084":
		return "HDR10 (PQ)"
	case "arib-std-b67":
		return "HLG"
	}
	return ""
}

func scanName(fieldOrder string) string {
	switch fieldOrder {
	case "", "unknown":
		return ""
	case "progressive":
		return "Progressive"
	}
	return "Interlaced"
}

func language(tags map[string]string) string {
	if lang := tags["language"]; lang != "und" {
		return lang
	}
	return ""
}

func lowerKeys(tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[strings.ToLower(key)] = value
	}
	return out
}

// shownTags are the tags of a whole file worth showing, with the names files use
// for them.
var shownTags = map[string][]string{
	"title":        {"title"},
	"artist":       {"artist", "author"},
	"album":        {"album"},
	"album_artist": {"album_artist", "album artist"},
	"genre":        {"genre"},
	"year":         {"date", "year", "creation_time"},
	"track":        {"track"},
	"composer":     {"composer"},
	"encoder":      {"encoder", "encoded_by"},
	"comment":      {"comment", "description"},
}

func pickTags(tags map[string]string) map[string]string {
	lower := lowerKeys(tags)
	picked := map[string]string{}
	for name, keys := range shownTags {
		for _, key := range keys {
			if value := strings.TrimSpace(lower[key]); value != "" {
				if name == "year" && len(value) > 10 {
					value = value[:10]
				}
				picked[name] = value
				break
			}
		}
	}
	return picked
}
