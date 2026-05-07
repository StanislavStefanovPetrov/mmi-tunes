package postprocess

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/bogem/id3v2/v2"
	"golang.org/x/image/draw"
)

// MaxCoverPx is the Audi MMI 3G+ upper bound for embedded album art.
const MaxCoverPx = 800

// ResizeJPEG re-encodes img bytes to a JPEG no larger than maxPx in either
// dimension, preserving aspect ratio. Returns the original bytes (with the
// "image/jpeg" mime type) if the image already fits within maxPx.
//
// Accepts JPEG and PNG inputs; output is always JPEG. quality is 1..100.
func ResizeJPEG(img []byte, maxPx int, quality int) ([]byte, string, error) {
	if maxPx <= 0 {
		return nil, "", fmt.Errorf("maxPx must be positive, got %d", maxPx)
	}

	src, format, err := image.Decode(bytes.NewReader(img))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxPx && h <= maxPx && format == "jpeg" {
		// Already fits and already JPEG — keep as-is.
		return img, "image/jpeg", nil
	}

	// Compute target size preserving aspect ratio.
	tw, th := w, h
	if w > maxPx || h > maxPx {
		if w >= h {
			tw = maxPx
			th = h * maxPx / w
		} else {
			th = maxPx
			tw = w * maxPx / h
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if quality <= 0 || quality > 100 {
		quality = 90
	}
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, "", fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

// ResizeCoverArtInMP3 inspects every APIC (attached picture) frame in path's
// ID3v2 tag and re-encodes any image whose largest dimension exceeds maxPx
// down to ≤ maxPx, in place. Frames smaller than maxPx are left untouched.
//
// This is the post-processor that brings yt-dlp's embedded YouTube
// thumbnails (typically 1280×720) down to Audi MMI's 800×800 limit.
func ResizeCoverArtInMP3(path string, maxPx int) error {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("open id3v2: %w", err)
	}
	defer tag.Close()

	frames := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(frames) == 0 {
		return nil
	}

	changed := false
	newFrames := make([]id3v2.Framer, 0, len(frames))
	for _, f := range frames {
		pic, ok := f.(id3v2.PictureFrame)
		if !ok {
			newFrames = append(newFrames, f)
			continue
		}
		resized, mime, err := ResizeJPEG(pic.Picture, maxPx, 90)
		if err != nil {
			return fmt.Errorf("resize cover: %w", err)
		}
		if &resized[0] == &pic.Picture[0] || bytes.Equal(resized, pic.Picture) {
			newFrames = append(newFrames, f)
			continue
		}
		newFrames = append(newFrames, id3v2.PictureFrame{
			Encoding:    pic.Encoding,
			MimeType:    mime,
			PictureType: pic.PictureType,
			Description: pic.Description,
			Picture:     resized,
		})
		changed = true
	}

	if !changed {
		return nil
	}

	tag.DeleteFrames(tag.CommonID("Attached picture"))
	for _, f := range newFrames {
		tag.AddFrame(tag.CommonID("Attached picture"), f)
	}
	if err := tag.Save(); err != nil {
		return fmt.Errorf("save id3v2: %w", err)
	}
	return nil
}

// readAll is a small convenience used by callers that already have an
// io.Reader rather than bytes.
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
