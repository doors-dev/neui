package media

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"neui.dev/api"
)

const (
	fallbackImageSide = 2000
	fallbackByteCap   = 4 << 20
	retryArea         = 0.75
	MaxDecodePixels   = 50_000_000
)

func Shape(data []byte, mimeType api.ImageMime, model api.Model) (api.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return api.Image{}, err
	}
	byteCap := model.ImageBytes
	if byteCap <= 0 {
		byteCap = fallbackByteCap
	}
	side := model.ImageSide
	if side <= 0 {
		side = fallbackImageSide
	}
	w, h := cfg.Width, cfg.Height
	if w <= 0 || h <= 0 || int64(w)*int64(h) > MaxDecodePixels {
		return api.Image{}, fmt.Errorf("image is %dx%d, over the %d pixels that can be decoded", w, h, MaxDecodePixels)
	}
	fits := w <= side && h <= side
	if model.ImagePixels > 0 {
		fits = fits && w*h <= model.ImagePixels
	}
	if fits && len(data) <= byteCap {
		return api.Image{Data: data, Mime: mimeType}, nil
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return api.Image{}, err
	}
	w, h = fitSidesToArea(w, h, model.ImagePixels)
	w, h = fitSidesToLongest(w, h, side)
	w, h = max(w, 1), max(h, 1)
	w, h = clampSidesToArea(w, h, model.ImagePixels)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var buf bytes.Buffer
	if mimeType != api.ImageJPEG {
		if err := png.Encode(&buf, dst); err != nil {
			return api.Image{}, err
		}
		if buf.Len() <= byteCap {
			return api.Image{Data: buf.Bytes(), Mime: api.ImagePNG}, nil
		}
	}
	size := 0
	for round := range 2 {
		if round > 0 {
			w, h = scaleSidesByArea(w, h, retryArea)
			shrunk := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
			draw.BiLinear.Scale(shrunk, shrunk.Bounds(), src, src.Bounds(), draw.Src, nil)
			dst = shrunk
		}
		for _, quality := range [3]int{85, 70, 55} {
			buf.Reset()
			if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
				return api.Image{}, err
			}
			if buf.Len() <= byteCap {
				return api.Image{Data: buf.Bytes(), Mime: api.ImageJPEG}, nil
			}
			size = buf.Len()
		}
	}
	return api.Image{}, fmt.Errorf(
		"image is still %d bytes at %dx%d and the lowest quality tried, over the %d bytes this model takes in one image",
		size, dst.Bounds().Dx(), dst.Bounds().Dy(), byteCap,
	)
}

func scaleSidesByArea(w, h int, areaShare float64) (int, int) {
	sideFactor := math.Sqrt(areaShare)
	scaledWidth := int(float64(w) * sideFactor)
	scaledHeight := int(float64(h) * sideFactor)
	return scaledWidth, scaledHeight
}

func fitSidesToArea(w, h, pixels int) (int, int) {
	if pixels <= 0 {
		return w, h
	}
	area := w * h
	if area <= pixels {
		return w, h
	}
	areaShare := float64(pixels) / float64(area)
	return scaleSidesByArea(w, h, areaShare)
}

func fitSidesToLongest(w, h, longest int) (int, int) {
	side := max(w, h)
	if side <= longest {
		return w, h
	}
	fittedWidth := w * longest / side
	fittedHeight := h * longest / side
	return fittedWidth, fittedHeight
}

func clampSidesToArea(w, h, pixels int) (int, int) {
	if pixels <= 0 {
		return w, h
	}
	widthAtFullHeight := max(pixels/h, 1)
	clampedWidth := min(w, widthAtFullHeight)
	heightAtClampedWidth := max(pixels/clampedWidth, 1)
	clampedHeight := min(h, heightAtClampedWidth)
	return clampedWidth, clampedHeight
}
