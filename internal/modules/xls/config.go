package xls

import "context"

// Config tunes the xls module; a zero field takes its default.
type Config struct {
	// MaxFileSize is the largest workbook in bytes the module opens; a larger
	// one is refused. Default 50 MiB.
	MaxFileSize int64
	// MaxReadRows caps the rows returned, or matches collected, by one call.
	// Default 200.
	MaxReadRows uint
}

func (c *Config) initDefaults() {
	if c.MaxFileSize == 0 {
		c.MaxFileSize = 50 << 20
	}
	if c.MaxReadRows == 0 {
		c.MaxReadRows = 200
	}
}

type configKey struct{}

func cfg(ctx context.Context) Config {
	c, _ := ctx.Value(configKey{}).(Config)
	c.initDefaults()
	return c
}
