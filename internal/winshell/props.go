package winshell

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"sashaexplorer/internal/media"
)

var procPropVariantClear = ole32.NewProc("PropVariantClear")

var (
	iidShellItem2    = windows.GUID{Data1: 0x7e9fb0d3, Data2: 0x919f, Data3: 0x4307, Data4: [8]byte{0xab, 0x2e, 0x9b, 0x18, 0x60, 0x31, 0x0c, 0x93}}
	iidPropertyStore = windows.GUID{Data1: 0x886d8eeb, Data2: 0x8cf2, Data3: 0x4446, Data4: [8]byte{0x8d, 0x02, 0xcd, 0xba, 0x1d, 0xbd, 0xcf, 0x99}}
)

// propertyKey is PROPERTYKEY: a property set and an id in it.
type propertyKey struct {
	Set windows.GUID
	ID  uint32
}

func propSet(data1 uint32, data2, data3 uint16, data4 ...byte) windows.GUID {
	guid := windows.GUID{Data1: data1, Data2: data2, Data3: data3}
	copy(guid.Data4[:], data4)
	return guid
}

// Property sets of propkey.h.
var (
	setMedia = propSet(0x64440490, 0x4c8b, 0x11d1, 0x8b, 0x70, 0x08, 0x00, 0x36, 0xb1, 0x1a, 0x03)
	setVideo = propSet(0x64440491, 0x4c8b, 0x11d1, 0x8b, 0x70, 0x08, 0x00, 0x36, 0xb1, 0x1a, 0x03)
	setImage = propSet(0x6444048f, 0x4c8b, 0x11d1, 0x8b, 0x70, 0x08, 0x00, 0x36, 0xb1, 0x1a, 0x03)
	setMusic = propSet(0x56a3372e, 0xce9c, 0x11d2, 0x9f, 0x0e, 0x00, 0x60, 0x97, 0xc6, 0x86, 0xf6)
	setTitle = propSet(0xf29f85e0, 0x4ff9, 0x1068, 0xab, 0x91, 0x08, 0x00, 0x2b, 0x27, 0xb3, 0xd9)
	setPhoto = propSet(0x14b81da1, 0x0135, 0x4d31, 0x96, 0xd9, 0x6c, 0xbf, 0xc9, 0x67, 0x1a, 0x99)
	setLens  = propSet(0xe6ddcaf7, 0x29c5, 0x4f0a, 0x9a, 0x68, 0xd1, 0x94, 0x12, 0xec, 0x70, 0x90)
)

var (
	keyDuration      = propertyKey{setMedia, 3} // 100 ns units
	keyAudioBitrate  = propertyKey{setMedia, 4} // bits per second
	keyAudioRate     = propertyKey{setMedia, 5} // Hz
	keyAudioBits     = propertyKey{setMedia, 6} // bits per sample
	keyAudioChannels = propertyKey{setMedia, 7} //
	keyAudioFormat   = propertyKey{setMedia, 2} // a subtype GUID as text
	keyAudioCompress = propertyKey{setMedia, 10}
	keyVideoWidth    = propertyKey{setVideo, 3}
	keyVideoHeight   = propertyKey{setVideo, 4}
	keyVideoFPS      = propertyKey{setVideo, 6} // frames per 1000 seconds
	keyVideoBitrate  = propertyKey{setVideo, 8}
	keyVideoBits     = propertyKey{setVideo, 9}
	keyVideoCodec    = propertyKey{setVideo, 10} // a subtype GUID as text
	keyTotalBitrate  = propertyKey{setVideo, 43}
	keyFourCC        = propertyKey{setVideo, 44}
	keyImageWidth    = propertyKey{setImage, 3}
	keyImageHeight   = propertyKey{setImage, 4}
	keyImageDPI      = propertyKey{setImage, 5}
	keyImageBits     = propertyKey{setImage, 7}
	keyArtist        = propertyKey{setMusic, 2}
	keyAlbum         = propertyKey{setMusic, 4}
	keyYear          = propertyKey{setMusic, 5}
	keyTrack         = propertyKey{setMusic, 7}
	keyGenre         = propertyKey{setMusic, 11}
	keyAlbumArtist   = propertyKey{setMusic, 13}
	keyTitle         = propertyKey{setTitle, 2}
	keyCameraMaker   = propertyKey{setPhoto, 271}
	keyCameraModel   = propertyKey{setPhoto, 272}
	keyExposure      = propertyKey{setPhoto, 33434}
	keyFNumber       = propertyKey{setPhoto, 33437}
	keyISO           = propertyKey{setPhoto, 34855}
	keyTaken         = propertyKey{setPhoto, 36867}
	keyFocalLength   = propertyKey{setPhoto, 37386}
	keyFlash         = propertyKey{setPhoto, 37385}
	keyLensModel     = propertyKey{setLens, 100}
)

// propVariant is PROPVARIANT on 64-bit Windows: a type and 16 bytes of value.
type propVariant struct {
	Type uint16
	_    [3]uint16
	Val  [2]uint64
}

const (
	vtI2       = 2
	vtI4       = 3
	vtR4       = 4
	vtR8       = 5
	vtBool     = 11
	vtUI1      = 17
	vtUI2      = 18
	vtUI4      = 19
	vtI8       = 20
	vtUI8      = 21
	vtInt      = 22
	vtUInt     = 23
	vtLPWSTR   = 31
	vtFiletime = 64
	vtVector   = 0x1000

	gpsBestEffort = 0x40
)

type propertyStore struct{ comObject }

func (s propertyStore) value(key propertyKey) (any, bool) {
	var value propVariant
	if failed(s.call(5, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value)))) != nil {
		return nil, false
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&value)))
	low := value.Val[0]
	switch value.Type {
	case vtI2:
		return int64(int16(low)), true
	case vtI4, vtInt:
		return int64(int32(low)), true
	case vtUI1:
		return int64(uint8(low)), true
	case vtUI2:
		return int64(uint16(low)), true
	case vtUI4, vtUInt:
		return int64(uint32(low)), true
	case vtI8, vtUI8:
		return int64(low), true
	case vtR4:
		return float64(*(*float32)(unsafe.Pointer(&value.Val[0]))), true
	case vtR8:
		return *(*float64)(unsafe.Pointer(&value.Val[0])), true
	case vtBool:
		return int16(low) != 0, true
	case vtLPWSTR:
		return windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&value.Val[0]))), true
	case vtFiletime:
		filetime := windows.Filetime{LowDateTime: uint32(low), HighDateTime: uint32(low >> 32)}
		return time.Unix(0, filetime.Nanoseconds()), true
	case vtVector | vtLPWSTR:
		count := int(uint32(low))
		items := unsafe.Slice(*(***uint16)(unsafe.Pointer(&value.Val[1])), count)
		var texts []string
		for _, item := range items {
			if text := windows.UTF16PtrToString(item); text != "" {
				texts = append(texts, text)
			}
		}
		return strings.Join(texts, "; "), len(texts) > 0
	}
	return nil, false
}

func (s propertyStore) int(key propertyKey) int64 {
	value, _ := s.value(key)
	switch v := value.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}

func (s propertyStore) float(key propertyKey) float64 {
	value, _ := s.value(key)
	switch v := value.(type) {
	case int64:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

func (s propertyStore) text(key propertyKey) string {
	value, _ := s.value(key)
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case int64:
		if v != 0 {
			return strconv.FormatInt(v, 10)
		}
	}
	return ""
}

// MediaInfo reads what Windows knows about a video, a song, or a picture from
// the property handlers of the system, the same that fill Explorer's columns.
func (p *Pool) MediaInfo(path string) (media.Info, error) {
	var (
		info media.Info
		err  error
	)
	p.Do(func() { info, err = mediaInfo(path) })
	return info, err
}

func mediaInfo(path string) (media.Info, error) {
	var item comObject
	hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(utf16Ptr(path))), 0,
		uintptr(unsafe.Pointer(&iidShellItem2)), uintptr(unsafe.Pointer(&item)),
	)
	if err := failed(hr); err != nil {
		return media.Info{}, err
	}
	defer item.release()
	var store propertyStore
	// IShellItem2::GetPropertyStore
	if err := failed(item.call(8, gpsBestEffort, uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store.comObject)))); err != nil {
		return media.Info{}, err
	}
	defer store.release()

	info := media.Info{Source: "windows", Tags: map[string]string{}}
	info.Duration = float64(store.int(keyDuration)) / 1e7
	info.Bitrate = store.int(keyTotalBitrate)

	if width := store.int(keyVideoWidth); width > 0 {
		video := media.Video{
			Codec:    videoCodec(store.text(keyVideoCodec), uint32(store.int(keyFourCC))),
			Width:    int(width),
			Height:   int(store.int(keyVideoHeight)),
			FPS:      float64(store.int(keyVideoFPS)) / 1000,
			Bitrate:  store.int(keyVideoBitrate),
			BitDepth: int(store.int(keyVideoBits)),
		}
		// Windows gives bits per pixel here, such as 24, not bits per channel.
		if video.BitDepth >= 24 {
			video.BitDepth /= 3
		}
		info.Video = append(info.Video, video)
	}
	if channels := store.int(keyAudioChannels); channels > 0 {
		info.Audio = append(info.Audio, media.Audio{
			Codec:      audioCodec(store.text(keyAudioFormat) + store.text(keyAudioCompress)),
			Channels:   int(channels),
			SampleRate: int(store.int(keyAudioRate)),
			Bitrate:    store.int(keyAudioBitrate),
			BitDepth:   int(store.int(keyAudioBits)),
		})
	}
	for name, key := range map[string]propertyKey{
		"title": keyTitle, "artist": keyArtist, "album": keyAlbum, "album_artist": keyAlbumArtist,
		"genre": keyGenre, "year": keyYear, "track": keyTrack,
	} {
		if text := store.text(key); text != "" {
			info.Tags[name] = text
		}
	}

	if width := store.int(keyImageWidth); width > 0 && len(info.Video) == 0 {
		photo := &media.Photo{
			Width:       int(width),
			Height:      int(store.int(keyImageHeight)),
			BitDepth:    int(store.int(keyImageBits)),
			DPI:         store.float(keyImageDPI),
			Exposure:    store.float(keyExposure),
			FNumber:     store.float(keyFNumber),
			ISO:         int(store.int(keyISO)),
			FocalLength: store.float(keyFocalLength),
			Lens:        store.text(keyLensModel),
			// Bit 0 of the EXIF flash value says whether it fired.
			Flash: store.int(keyFlash)&1 == 1,
		}
		maker, model := store.text(keyCameraMaker), store.text(keyCameraModel)
		// Many cameras repeat the maker in the model, such as "Canon" and
		// "Canon EOS R5".
		if maker != "" && !strings.HasPrefix(strings.ToLower(model), strings.ToLower(maker)) {
			photo.Camera = strings.TrimSpace(maker + " " + model)
		} else {
			photo.Camera = model
		}
		if taken, ok := store.value(keyTaken); ok {
			if when, ok := taken.(time.Time); ok && when.Year() > 1900 {
				photo.Taken = when.UnixMilli()
			}
		}
		info.Photo = photo
	}
	return info, nil
}

// videoCodec names a Media Foundation subtype such as
// "{34363248-0000-0010-8000-00AA00389B71}", whose first part is a FourCC.
func videoCodec(subtype string, fourCC uint32) string {
	if fourCC == 0 {
		fourCC = subtypeCode(subtype)
	}
	if fourCC == 0 {
		return ""
	}
	code := strings.ToUpper(string([]byte{byte(fourCC), byte(fourCC >> 8), byte(fourCC >> 16), byte(fourCC >> 24)}))
	switch code {
	case "H264", "AVC1", "X264":
		return "H.264 (AVC)"
	case "HEVC", "HVC1", "HEV1", "H265":
		return "H.265 (HEVC)"
	case "AV01":
		return "AV1"
	case "VP90":
		return "VP9"
	case "VP80":
		return "VP8"
	case "MP4V", "XVID", "DIVX", "DX50", "FMP4":
		return "MPEG-4 Part 2"
	case "MPG2", "MP2V":
		return "MPEG-2"
	case "WMV3":
		return "Windows Media Video 9"
	case "WVC1":
		return "VC-1"
	case "MJPG":
		return "Motion JPEG"
	}
	for _, r := range code {
		if r < ' ' || r > '~' {
			return fmt.Sprintf("0x%08X", fourCC)
		}
	}
	return strings.TrimSpace(code)
}

// audioCodec names an audio subtype, whose first part is a WAVE format tag.
func audioCodec(subtype string) string {
	switch subtypeCode(subtype) {
	case 0:
		return ""
	case 0x0001:
		return "PCM"
	case 0x0003:
		return "PCM (float)"
	case 0x0055:
		return "MP3"
	case 0x0050:
		return "MP2"
	case 0x0161:
		return "Windows Media Audio"
	case 0x0162:
		return "Windows Media Audio Pro"
	case 0x0163:
		return "Windows Media Audio Lossless"
	case 0x1600, 0x1610, 0xFF:
		return "AAC"
	case 0x2000:
		return "Dolby Digital (AC-3)"
	case 0x2001:
		return "DTS"
	case 0xF1AC:
		return "FLAC"
	case 0x704F:
		return "Opus"
	case 0x6C61:
		return "Apple Lossless"
	case 0xA106:
		return "AAC"
	}
	return ""
}

func subtypeCode(subtype string) uint32 {
	text := strings.Trim(subtype, "{}")
	if len(text) < 8 {
		return 0
	}
	code, err := strconv.ParseUint(text[:8], 16, 32)
	if err != nil {
		return 0
	}
	return uint32(code)
}
