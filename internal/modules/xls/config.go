package xls

import "context"

type Config struct {
	MaxFileSize  int64
	MaxUnzipSize int64
	MaxReadRows  uint
}

func (c *Config) initDefaults() {
	if c.MaxFileSize == 0 {
		c.MaxFileSize = 50 << 20
	}
	if c.MaxUnzipSize == 0 {
		c.MaxUnzipSize = 1 << 30
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
