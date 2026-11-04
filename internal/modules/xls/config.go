package xls

import "context"

type Config struct {
	MaxFileSize int64 // bytes per workbook; default 50 MiB
	MaxReadRows uint  // rows returned or matches collected per call; default 200
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
