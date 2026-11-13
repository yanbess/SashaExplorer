package media

import "testing"

const sample = `{
 "streams": [
  {"codec_type": "video", "codec_name": "hevc", "profile": "Main 10", "width": 3840, "height": 2160,
   "avg_frame_rate": "24000/1001", "pix_fmt": "yuv420p10le", "color_transfer": "smpte2084",
   "display_aspect_ratio": "16:9", "field_order": "progressive", "tags": {"BPS-eng": "40000000"}},
  {"codec_type": "audio", "codec_name": "eac3", "channels": 6, "channel_layout": "5.1(side)",
   "sample_rate": "48000", "bit_rate": "640000", "tags": {"language": "eng", "title": "Surround"}},
  {"codec_type": "subtitle", "codec_name": "subrip", "tags": {"language": "rus"}},
  {"codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}}
 ],
 "format": {"format_long_name": "Matroska / WebM", "duration": "5400.5", "bit_rate": "41000000",
  "tags": {"TITLE": "A film", "DATE": "2024-05-01T00:00:00Z"}}
}`

func TestParseProbe(t *testing.T) {
	info, err := ParseProbe([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "Matroska / WebM" || info.Duration != 5400.5 || info.Bitrate != 41000000 {
		t.Errorf("format %+v", info)
	}
	if len(info.Video) != 1 {
		t.Fatalf("cover art counted as video: %+v", info.Video)
	}
	video := info.Video[0]
	if video.Codec != "H.265 (HEVC)" || video.BitDepth != 10 || video.HDR == "" || video.Bitrate != 40000000 ||
		video.FPS < 23.97 || video.FPS > 23.98 || video.Scan != "Progressive" {
		t.Errorf("video %+v", video)
	}
	audio := info.Audio[0]
	if audio.Codec != "Dolby Digital Plus (E-AC-3)" || audio.Channels != 6 || audio.Language != "eng" || audio.Title != "Surround" {
		t.Errorf("audio %+v", audio)
	}
	if len(info.Subtitles) != 1 || info.Subtitles[0].Language != "rus" {
		t.Errorf("subtitles %+v", info.Subtitles)
	}
	if info.Tags["title"] != "A film" || info.Tags["year"] != "2024-05-01" {
		t.Errorf("tags %+v", info.Tags)
	}
}

func TestCodecName(t *testing.T) {
	for codec, want := range map[string]string{"h264": "H.264 (AVC)", "pcm_s24le": "PCM (s24le)", "xyz": "XYZ"} {
		if got := CodecName(codec); got != want {
			t.Errorf("%s: %s", codec, got)
		}
	}
}
