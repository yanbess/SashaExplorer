package winshell

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"unsafe"
)

// Flags of IShellItemImageFactory::GetImage.
const (
	siigbfResizeToFit   = 0x00
	siigbfIconOnly      = 0x04
	siigbfThumbnailOnly = 0x08
)

// MaxImageSize limits the size that can be asked for.
const MaxImageSize = 1024

// Image is an encoded picture.
type Image struct {
	Data        []byte
	ContentType string
	// Fallback is an icon given in place of a thumbnail Windows could not make.
	// That can pass, as when a file is still being synced, so it is not kept.
	Fallback bool
}

// Thumbnail returns the picture Windows shows for a file: a preview for images,
// videos, and documents, or the icon when there is none.
func (p *Pool) Thumbnail(path string, size int) (Image, error) {
	image, err := p.shellImage(path, size, siigbfThumbnailOnly)
	if err == nil {
		return image, nil
	}
	image, err = p.shellImage(path, size, siigbfIconOnly)
	image.Fallback = true
	return image, err
}

// Icon returns the icon of a file or folder.
func (p *Pool) Icon(path string, size int) (Image, error) {
	return p.shellImage(path, size, siigbfIconOnly)
}

func (p *Pool) shellImage(path string, size int, flags uintptr) (Image, error) {
	size = min(max(size, 16), MaxImageSize)
	var (
		pixels *image.RGBA
		opaque bool
		err    error
	)
	p.Do(func() { pixels, opaque, err = shellBitmap(path, size, flags) })
	if err != nil {
		return Image{}, err
	}
	var out bytes.Buffer
	// A photo is smaller and faster as JPEG. An icon needs its transparency.
	if opaque && flags != siigbfIconOnly {
		err = jpeg.Encode(&out, pixels, &jpeg.Options{Quality: 85})
		return Image{Data: out.Bytes(), ContentType: "image/jpeg"}, err
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	err = encoder.Encode(&out, pixels)
	return Image{Data: out.Bytes(), ContentType: "image/png"}, err
}

func shellBitmap(path string, size int, flags uintptr) (*image.RGBA, bool, error) {
	var factory comObject
	hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(utf16Ptr(path))), 0,
		uintptr(unsafe.Pointer(&iidShellItemImageFactory)), uintptr(unsafe.Pointer(&factory)),
	)
	if err := failed(hr); err != nil {
		return nil, false, err
	}
	defer factory.release()

	// SIZE is 8 bytes and goes by value, so it fits in one register.
	sizeArg := uintptr(uint32(size)) | uintptr(uint32(size))<<32
	var bitmap uintptr
	if err := failed(factory.call(3, sizeArg, flags, uintptr(unsafe.Pointer(&bitmap)))); err != nil {
		return nil, false, err
	}
	defer procDeleteObject.Call(bitmap)
	return bitmapPixels(bitmap)
}

type bitmapInfo struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// bitmapPixels copies a bitmap into an image. It also reports whether the
// picture has no transparency.
func bitmapPixels(bitmap uintptr) (*image.RGBA, bool, error) {
	var info bitmapInfo
	if r, _, _ := procGetObjectW.Call(bitmap, unsafe.Sizeof(info), uintptr(unsafe.Pointer(&info))); r == 0 {
		return nil, false, errors.New("read thumbnail size")
	}
	width, height := int(info.Width), int(info.Height)
	if width <= 0 || height <= 0 || width > 4*MaxImageSize || height > 4*MaxImageSize {
		return nil, false, errors.New("thumbnail has an unexpected size")
	}

	header := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(width),
		Height:   -int32(height), // Negative: rows from top to bottom.
		Planes:   1,
		BitCount: 32,
	}
	// BITMAPINFO has room for one color after the header.
	var infoBuffer [unsafe.Sizeof(bitmapInfoHeader{}) + 4]byte
	*(*bitmapInfoHeader)(unsafe.Pointer(&infoBuffer[0])) = header

	pixels := make([]byte, width*height*4)
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	lines, _, _ := procGetDIBits.Call(
		dc, bitmap, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&infoBuffer[0])), 0,
	)
	if lines == 0 {
		return nil, false, errors.New("read thumbnail pixels")
	}

	// Windows gives blue, green, red, alpha with the color already multiplied
	// by alpha, which is also how image.RGBA stores it. A bitmap without
	// transparency often has alpha 0 everywhere, which means opaque.
	hasAlpha := false
	opaque := true
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 0 {
			hasAlpha = true
		}
		if pixels[i] != 0xff {
			opaque = false
		}
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
		if !hasAlpha {
			pixels[i+3] = 0xff
		}
	}
	return &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}, opaque || !hasAlpha, nil
}
