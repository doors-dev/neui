package media

import (
	"bytes"
	"errors"
	"fmt"
	"image/gif"
)

var errTruncatedGIF = errors.New("a truncated GIF")

func StillGIF(data []byte) error {
	if _, err := gif.DecodeConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("a malformed GIF: %w", err)
	}
	i, frames := 13, 0
	if data[10]&0x80 != 0 {
		i += 3 << (data[10]&7 + 1)
	}
	for {
		if i >= len(data) {
			return errTruncatedGIF
		}
		switch data[i] {
		case 0x3B:
			return nil
		case 0x21:
			i += 2
		case 0x2C:
			frames++
			if frames > 1 {
				return errors.New("an animated GIF, which providers do not accept")
			}
			if i+10 >= len(data) {
				return errTruncatedGIF
			}
			packed := data[i+9]
			i += 11
			if packed&0x80 != 0 {
				i += 3 << (packed&7 + 1)
			}
		default:
			return fmt.Errorf("a malformed GIF: unknown block 0x%02x", data[i])
		}
		for {
			if i >= len(data) {
				return errTruncatedGIF
			}
			n := int(data[i])
			i += 1 + n
			if n == 0 {
				break
			}
		}
	}
}
