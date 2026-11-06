package xls

import "context"

// Config tunes the xls module; a zero field takes its default.
type Config struct {
	// MaxFileSize is the largest workbook file on disk, in bytes, the module
	// opens; a larger one is refused. Default 50 MiB.
	MaxFileSize int64
	// MaxUnzipSize caps the uncompressed contents of a workbook in bytes; a
	// workbook that expands past it, or holds a part compressed over 100x, is
	// refused. Default 1 GiB.
	MaxUnzipSize int64
	// MaxReadRows caps the rows returned, or matches collected, by one call;
	// it also caps the rows profile reads to guess column types and the
	// errors check reports. Default 200.
	MaxReadRows uint
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
