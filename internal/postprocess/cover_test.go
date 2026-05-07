package postprocess

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// makeJPEG returns a JPEG-encoded test image of the given size with a
// horizontal red→blue gradient. Big enough to exercise resize logic.
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8(x * 255 / max(1, w-1))
			b := uint8(255 - r)
			img.Set(x, y, color.RGBA{R: r, G: 0, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 200, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func decodeBounds(t *testing.T, b []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode bounds: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestResizeJPEG_Landscape1280x720(t *testing.T) {
	src := makeJPEG(t, 1280, 720)
	out, mime, err := ResizeJPEG(src, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" {
		t.Errorf("want image/jpeg mime, got %s", mime)
	}
	w, h := decodeBounds(t, out)
	if w != 800 || h != 450 {
		t.Errorf("want 800x450, got %dx%d", w, h)
	}
}

func TestResizeJPEG_Portrait720x1280(t *testing.T) {
	src := makeJPEG(t, 720, 1280)
	out, _, err := ResizeJPEG(src, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	w, h := decodeBounds(t, out)
	if w != 450 || h != 800 {
		t.Errorf("want 450x800, got %dx%d", w, h)
	}
}

func TestResizeJPEG_Square2000x2000(t *testing.T) {
	src := makeJPEG(t, 2000, 2000)
	out, _, err := ResizeJPEG(src, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	w, h := decodeBounds(t, out)
	if w != 800 || h != 800 {
		t.Errorf("want 800x800, got %dx%d", w, h)
	}
}

func TestResizeJPEG_AlreadySmallEnough(t *testing.T) {
	src := makeJPEG(t, 600, 400)
	out, mime, err := ResizeJPEG(src, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" {
		t.Errorf("want image/jpeg, got %s", mime)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("expected pass-through (no re-encode) for image already within bounds")
	}
}

func TestResizeJPEG_PNGInputConvertedToJPEG(t *testing.T) {
	src := makePNG(t, 1500, 1000)
	out, mime, err := ResizeJPEG(src, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/jpeg" {
		t.Errorf("want image/jpeg, got %s", mime)
	}
	w, h := decodeBounds(t, out)
	// 1500x1000 → 800x533
	if w != 800 || h != 533 {
		t.Errorf("want 800x533, got %dx%d", w, h)
	}
}

func TestResizeJPEG_InvalidMaxPx(t *testing.T) {
	src := makeJPEG(t, 100, 100)
	if _, _, err := ResizeJPEG(src, 0, 90); err == nil {
		t.Error("expected error for maxPx=0")
	}
	if _, _, err := ResizeJPEG(src, -10, 90); err == nil {
		t.Error("expected error for negative maxPx")
	}
}

func TestResizeJPEG_GarbageInput(t *testing.T) {
	if _, _, err := ResizeJPEG([]byte("not an image"), 800, 90); err == nil {
		t.Error("expected error for garbage input")
	}
}
