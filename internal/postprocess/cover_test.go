package postprocess

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
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
	out, mime, _, err := ResizeJPEG(src, 800, 90)
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
	out, _, _, err := ResizeJPEG(src, 800, 90)
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
	out, _, _, err := ResizeJPEG(src, 800, 90)
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
	out, mime, _, err := ResizeJPEG(src, 800, 90)
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
	out, mime, _, err := ResizeJPEG(src, 800, 90)
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
	if _, _, _, err := ResizeJPEG(src, 0, 90); err == nil {
		t.Error("expected error for maxPx=0")
	}
	if _, _, _, err := ResizeJPEG(src, -10, 90); err == nil {
		t.Error("expected error for negative maxPx")
	}
}

func TestResizeJPEG_GarbageInput(t *testing.T) {
	if _, _, _, err := ResizeJPEG([]byte("not an image"), 800, 90); err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestResizeJPEG_ChangedFlag(t *testing.T) {
	bigSrc := makeJPEG(t, 1500, 1000)
	_, _, changed, err := ResizeJPEG(bigSrc, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true for oversized JPEG")
	}

	smallSrc := makeJPEG(t, 600, 400)
	_, _, changed, err = ResizeJPEG(smallSrc, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected changed=false for small JPEG")
	}

	pngSrc := makePNG(t, 600, 400)
	_, _, changed, err = ResizeJPEG(pngSrc, 800, 90)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true for PNG (re-encoded to JPEG)")
	}
}

// writeTaggedMP3 builds a minimal ID3v2.3-tagged file whose TXXX frames are
// encoded the way yt-dlp writes them: encoding 0x01, a little-endian BOM,
// UTF-16LE text. We hand-assemble the bytes rather than use bogem, because
// bogem's writer is the thing under test — letting it produce the fixture
// would hide the very defect we are checking for.
func writeTaggedMP3(t *testing.T, path string, cover []byte, udt map[string]string) {
	t.Helper()

	utf16le := func(s string) []byte {
		out := []byte{0xff, 0xfe}
		for _, r := range s {
			out = append(out, byte(r), byte(r>>8))
		}
		return out
	}
	frame := func(id string, body []byte) []byte {
		out := []byte(id)
		n := len(body)
		out = append(out, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		out = append(out, 0, 0)
		return append(out, body...)
	}

	var frames []byte
	for desc, val := range udt {
		body := []byte{0x01}
		body = append(body, utf16le(desc)...)
		body = append(body, 0x00, 0x00) // UTF-16 terminator
		body = append(body, utf16le(val)...)
		frames = append(frames, frame("TXXX", body)...)
	}
	apic := []byte{0x00}
	apic = append(apic, []byte("image/jpeg")...)
	apic = append(apic, 0x00, 0x03, 0x00) // type=front cover, empty description
	apic = append(apic, cover...)
	frames = append(frames, frame("APIC", apic)...)

	sz := len(frames)
	hdr := append([]byte("ID3"), 0x03, 0x00, 0x00)
	hdr = append(hdr, byte(sz>>21)&0x7f, byte(sz>>14)&0x7f, byte(sz>>7)&0x7f, byte(sz)&0x7f)

	// A byte of "audio" so the file is not just a tag.
	out := append(append(hdr, frames...), 0xff, 0xfb, 0x00, 0x00)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// bogem/id3v2 v2.1.4 corrupts UTF-16 TXXX frames whenever it rewrites a tag,
// misaligning them by one NUL byte and truncating the last character. We only
// ever want the cover resized, so the two frames yt-dlp fills with the
// YouTube blurb are dropped instead of being written back mangled.
func TestResizeCoverArtInMP3_DropsFramesBogemWouldCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.mp3")
	writeTaggedMP3(t, path, makeJPEG(t, 1280, 720), map[string]string{
		"description": "Кирилица, за да е UTF-16",
		"synopsis":    "също кирилица",
	})

	if err := ResizeCoverArtInMP3(path, 800); err != nil {
		t.Fatalf("resize: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer tag.Close()

	for _, f := range tag.GetFrames(tag.CommonID("User defined text information frame")) {
		if u, ok := f.(id3v2.UserDefinedTextFrame); ok {
			if u.Description == "description" || u.Description == "synopsis" {
				t.Errorf("frame %q survived; bogem will have corrupted it", u.Description)
			}
		}
	}

	// The point of the function still has to work.
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pics) != 1 {
		t.Fatalf("got %d APIC frames, want 1", len(pics))
	}
	w, h := decodeBounds(t, pics[0].(id3v2.PictureFrame).Picture)
	if w > 800 || h > 800 {
		t.Errorf("cover is %dx%d, want both sides <= 800", w, h)
	}
}

// Only the two known-bad descriptors go. yt-dlp also writes ASCII-encoded
// TXXX frames, which bogem round-trips correctly and which must survive.
func TestResizeCoverArtInMP3_KeepsOtherUserTextFrames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.mp3")
	writeTaggedMP3(t, path, makeJPEG(t, 1280, 720), map[string]string{
		"description": "drop me",
		"purl":        "https://www.youtube.com/watch?v=abc",
	})

	if err := ResizeCoverArtInMP3(path, 800); err != nil {
		t.Fatalf("resize: %v", err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer tag.Close()

	var got []string
	for _, f := range tag.GetFrames(tag.CommonID("User defined text information frame")) {
		if u, ok := f.(id3v2.UserDefinedTextFrame); ok {
			got = append(got, u.Description)
		}
	}
	if len(got) != 1 || got[0] != "purl" {
		t.Errorf("remaining TXXX descriptors = %v, want exactly [purl]", got)
	}
}
