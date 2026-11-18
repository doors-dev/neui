// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
