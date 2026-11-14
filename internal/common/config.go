package common

import (
	"context"
	"os"
	"time"

	"neui.dev/api"
)

type ConfigKey struct{}

type FilesConfigKey struct{}

type IslandConfigKey struct{}

type ModulesKey struct{}

type SystemKey struct{}

type IslandFSKey struct{}

type IslandDirKey struct{}

type IslandFilesConfigKey struct{}

type IslandScopeKey struct{}

type ProviderKey struct{}

func AppProvider(ctx context.Context) api.Provider {
	p, _ := ctx.Value(ProviderKey{}).(api.Provider)
	return p
}

func AppModules(ctx context.Context) []ModuleFactory {
	m, _ := ctx.Value(ModulesKey{}).([]ModuleFactory)
	return m
}

func AppSystem(ctx context.Context) []string {
	s, _ := ctx.Value(SystemKey{}).([]string)
	return s
}

func ConfigDefaults(c api.ThreadConfig) api.ThreadConfig {
	if c.CompactionTrigger == 0 {
		c.CompactionTrigger = 0.8
	}
	if c.CompactionAllowed == 0 {
		c.CompactionAllowed = 0.2
	}
	if c.CompactionPreserve == 0 {
		c.CompactionPreserve = 0.1
	}
	if c.ReplayItemShare <= 0 {
		c.ReplayItemShare = 0.1
	}
	c.ReplayItemShare = min(c.ReplayItemShare, 0.25)
	if c.ThreadMemoryLimit == 0 {
		c.ThreadMemoryLimit = 18 << 20
	}
	if c.ToolCallLimit == 0 {
		c.ToolCallLimit = 32
	}
	return c
}

func FilesConfigDefaults(c api.FilesConfig) api.FilesConfig {
	if c.TempDir == "" {
		c.TempDir = os.TempDir()
	}
	if c.MaxFiles == 0 {
		c.MaxFiles = 10000
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = 1 << 30
	}
	if c.ReadLimit == 0 {
		c.ReadLimit = 50 << 10
	}
	if c.EditLimit == 0 {
		c.EditLimit = 10 << 20
	}
	if c.ImageFileLimit == 0 {
		c.ImageFileLimit = 20 << 20
	}
	if c.DocumentLimit == 0 {
		c.DocumentLimit = 32 << 20
	}
	if c.UnzipLimit == 0 {
		c.UnzipLimit = 512 << 20
	}
	if c.UnzipFiles == 0 {
		c.UnzipFiles = 10000
	}
	return c
}

func IslandConfigDefaults(c api.IslandConfig) api.IslandConfig {
	if c.MaxItems <= 0 {
		c.MaxItems = 512
	}
	if c.MaxOps <= 0 {
		c.MaxOps = 512
	}
	if c.TurnTime <= 0 {
		c.TurnTime = 30 * time.Second
	}
	if c.Memory <= 0 {
		c.Memory = 16 << 20
	}
	c.Memory = max(c.Memory, 4<<20)
	if c.MaxCallDepth <= 0 {
		c.MaxCallDepth = 1000
	}
	if c.MaxQueue <= 0 {
		c.MaxQueue = 16
	}
	if c.FnTime <= 0 {
		c.FnTime = 5 * time.Second
	}
	if c.FnMemory <= 0 {
		c.FnMemory = 4 << 20
	}
	c.FnMemory = max(c.FnMemory, 4<<20)
	return c
}

func AppConfig(ctx context.Context) api.ThreadConfig {
	c, _ := ctx.Value(ConfigKey{}).(api.ThreadConfig)
	return ConfigDefaults(c)
}

func AppFilesConfig(ctx context.Context) api.FilesConfig {
	c, _ := ctx.Value(FilesConfigKey{}).(api.FilesConfig)
	return FilesConfigDefaults(c)
}

func AppIslandConfig(ctx context.Context) api.IslandConfig {
	c, _ := ctx.Value(IslandConfigKey{}).(api.IslandConfig)
	return IslandConfigDefaults(c)
}
