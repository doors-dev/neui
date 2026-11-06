package media

import (
	"bytes"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"

	"neui.dev/api"
)

func noise(t *testing.T, w, h int) *image.RGBA {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.UintN(256))
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xff
	}
	return img
}

func noisePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, noise(t, w, h)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func noiseJPEG(t *testing.T, w, h, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, noise(t, w, h), &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func flatPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func dims(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestShapeScalesByArea(t *testing.T) {
	pixels := 1568 * 1568
	img, err := Shape(flatPNG(t, 4000, 3000), api.ImagePNG, api.Model{ImagePixels: pixels})
	if err != nil {
		t.Fatal(err)
	}
	w, h := dims(t, img.Data)
	if w*h > pixels {
		t.Fatalf("%dx%d is %d pixels, over %d", w, h, w*h, pixels)
	}
	if w*h < pixels/2 {
		t.Fatalf("%dx%d is %d pixels, needlessly far under %d", w, h, w*h, pixels)
	}
	if float64(w)/float64(h) < 1.3 || float64(w)/float64(h) > 1.37 {
		t.Fatalf("%dx%d lost the 4:3 aspect", w, h)
	}
}

func TestShapeKeepsFittingImage(t *testing.T) {
	data := flatPNG(t, 1000, 1000)
	img, err := Shape(data, api.ImagePNG, api.Model{ImagePixels: 1568 * 1568, ImageBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Data, data) || img.Mime != api.ImagePNG {
		t.Fatalf("re-encoded a fitting image: %s, %d bytes", img.Mime, len(img.Data))
	}
}

func TestShapeCapsJPEGBytes(t *testing.T) {
	data := noiseJPEG(t, 2000, 2000, 95)
	byteCap := len(data) / 4
	img, err := Shape(data, api.ImageJPEG, api.Model{ImagePixels: 1568 * 1568, ImageBytes: byteCap})
	if err != nil {
		t.Fatal(err)
	}
	if img.Mime != api.ImageJPEG {
		t.Fatalf("mime = %s", img.Mime)
	}
	if len(img.Data) > byteCap {
		t.Fatalf("%d bytes over the %d cap", len(img.Data), byteCap)
	}
	w, h := dims(t, img.Data)
	if w*h > 1568*1568 {
		t.Fatalf("%dx%d is over %d pixels", w, h, 1568*1568)
	}
}

func TestShapeCascadesQuality(t *testing.T) {
	data := noiseJPEG(t, 1000, 1000, 95)
	at85 := len(noiseJPEG(t, 1000, 1000, 85))
	at55 := len(noiseJPEG(t, 1000, 1000, 55))
	if at55 >= at85 {
		t.Fatalf("quality 55 (%d bytes) did not beat 85 (%d bytes)", at55, at85)
	}
	byteCap := at55 + (at85-at55)/2
	img, err := Shape(data, api.ImageJPEG, api.Model{ImagePixels: 1568 * 1568, ImageBytes: byteCap})
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Data) > byteCap {
		t.Fatalf("%d bytes over the %d cap, quality never dropped", len(img.Data), byteCap)
	}
	if w, h := dims(t, img.Data); w != 1000 || h != 1000 {
		t.Fatalf("%dx%d: downscaled although quality alone was enough", w, h)
	}
}

func TestShapeBoundsBothDimensions(t *testing.T) {
	const pixels = 2458624
	model := api.Model{ImagePixels: pixels}
	for _, size := range [][2]int{{20000, 100}, {200000, 1}, {1, 20000}} {
		img, err := Shape(flatPNG(t, size[0], size[1]), api.ImagePNG, model)
		if err != nil {
			t.Fatalf("%dx%d: %v", size[0], size[1], err)
		}
		w, h := dims(t, img.Data)
		if w > fallbackImageSide || h > fallbackImageSide {
			t.Fatalf("%dx%d came back as %dx%d: neither dimension is bounded", size[0], size[1], w, h)
		}
		if w*h > pixels {
			t.Fatalf("%dx%d came back as %dx%d, %d pixels, over the %d budget", size[0], size[1], w, h, w*h, pixels)
		}
	}
}

func TestShapeClampStaysUnderThePixelBudget(t *testing.T) {
	const pixels = 2458624
	img, err := Shape(flatPNG(t, 3000000, 1), api.ImagePNG, api.Model{ImagePixels: pixels})
	if err != nil {
		t.Fatal(err)
	}
	w, h := dims(t, img.Data)
	if w*h > pixels {
		t.Fatalf("%dx%d is %d pixels, over the %d budget: the one-pixel clamp pushed it past the limit", w, h, w*h, pixels)
	}
	if h < 1 {
		t.Fatalf("%dx%d: the short side has to stay at least one pixel", w, h)
	}
}

func TestShapeCapsTheLongestSide(t *testing.T) {
	model := api.Model{ImageSide: 1568}
	for _, size := range [][2]int{{4000, 1000}, {1000, 4000}} {
		img, err := Shape(flatPNG(t, size[0], size[1]), api.ImagePNG, model)
		if err != nil {
			t.Fatal(err)
		}
		w, h := dims(t, img.Data)
		if max(w, h) != 1568 || min(w, h) != 1568*1000/4000 {
			t.Fatalf("%dx%d came back as %dx%d, want the long side at 1568", size[0], size[1], w, h)
		}
	}
	data := flatPNG(t, 2500, 1000)
	img, err := Shape(data, api.ImagePNG, api.Model{ImageSide: 2576})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Data, data) {
		t.Fatal("re-encoded a 2500x1000 image that fits a 2576 side")
	}
}

func TestShapeFallsBackWithoutModelLimits(t *testing.T) {
	img, err := Shape(flatPNG(t, 4000, 100), api.ImagePNG, api.Model{})
	if err != nil {
		t.Fatal(err)
	}
	w, h := dims(t, img.Data)
	if want := 100 * fallbackImageSide / 4000; w != fallbackImageSide || h != want {
		t.Fatalf("%dx%d, want %dx%d", w, h, fallbackImageSide, want)
	}
	big := noisePNG(t, 1600, 1600)
	if len(big) <= fallbackByteCap {
		t.Fatalf("%d bytes is not over the %d fallback cap", len(big), fallbackByteCap)
	}
	img, err = Shape(big, api.ImagePNG, api.Model{})
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Data) > fallbackByteCap {
		t.Fatalf("%d bytes over the %d fallback cap", len(img.Data), fallbackByteCap)
	}
	if img.Mime != api.ImageJPEG {
		t.Fatalf("mime = %s, want JPEG after the PNG stayed over the cap", img.Mime)
	}
	if w, h := dims(t, img.Data); w != 1600 || h != 1600 {
		t.Fatalf("%dx%d: downscaled although the dims fit", w, h)
	}
}

func TestShapeErrorsWhenCapUnreachable(t *testing.T) {
	_, err := Shape(noiseJPEG(t, 1000, 1000, 95), api.ImageJPEG, api.Model{ImageBytes: 500})
	if err == nil {
		t.Fatal("an image that cannot meet the cap was returned anyway")
	}
	if !strings.Contains(err.Error(), "500 bytes") {
		t.Fatalf("err = %v", err)
	}
}

func TestShapeHoldsImageBytesOnRawBytes(t *testing.T) {
	data := noiseJPEG(t, 400, 400, 95)
	img, err := Shape(data, api.ImageJPEG, api.Model{ImageBytes: len(data)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.Data, data) {
		t.Fatalf("a %d byte image at exactly ImageBytes was re-encoded to %d bytes", len(data), len(img.Data))
	}
	img, err = Shape(data, api.ImageJPEG, api.Model{ImageBytes: len(data) - 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Data) > len(data)-1 {
		t.Fatalf("a %d byte image under a %d byte cap came back at %d bytes", len(data), len(data)-1, len(img.Data))
	}
}

func TestShapeRejectsOversizedDeclaredDimensions(t *testing.T) {
	ihdr := []byte("IHDR\x00\x01\x86\xa0\x00\x01\x86\xa0\x08\x06\x00\x00\x00")
	var data bytes.Buffer
	data.WriteString("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d")
	data.Write(ihdr)
	sum := crc32.ChecksumIEEE(ihdr)
	data.Write([]byte{byte(sum >> 24), byte(sum >> 16), byte(sum >> 8), byte(sum)})
	_, err := Shape(data.Bytes(), api.ImagePNG, api.Model{})
	if err == nil || !strings.Contains(err.Error(), "100000x100000") {
		t.Fatalf("Shape on a 100000x100000 header = %v, want a pixel cap error", err)
	}
}

func TestShapeFlattensTransparencyOntoWhiteForJPEG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1000, 1000))
	r := rand.New(rand.NewPCG(1, 2))
	for y := range 200 {
		for x := range 200 {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(r.UintN(256)), uint8(r.UintN(256)), uint8(r.UintN(256)), 0xff
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	byteCap := buf.Len() / 2
	for _, model := range []api.Model{{ImageBytes: byteCap}, {ImageBytes: byteCap / 3}} {
		shaped, err := Shape(buf.Bytes(), api.ImagePNG, model)
		if err != nil {
			t.Fatal(err)
		}
		if shaped.Mime != api.ImageJPEG {
			t.Fatalf("mime = %s", shaped.Mime)
		}
		out, err := jpeg.Decode(bytes.NewReader(shaped.Data))
		if err != nil {
			t.Fatal(err)
		}
		b := out.Bounds()
		if cr, cg, cb, _ := out.At(b.Dx()*4/5, b.Dy()*4/5).RGBA(); cr < 0xf000 || cg < 0xf000 || cb < 0xf000 {
			t.Fatalf("cap %d: a transparent pixel came out as %x %x %x, want white", model.ImageBytes, cr, cg, cb)
		}
	}
}

func TestShapeZeroSizeImageHasNoPixels(t *testing.T) {
	data := []byte("GIF89a\x00\x00\x00\x00\x00\x00\x00;")
	_, err := Shape(data, api.ImageGIF, api.Model{})
	if err == nil || !strings.Contains(err.Error(), "no pixels (0x0)") {
		t.Fatalf("err = %v", err)
	}
}
