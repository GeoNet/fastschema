package fastschema

import (
	"strings"
	"testing"

	"github.com/fastschema/fastschema/fs"
	"github.com/stretchr/testify/assert"
)

func newGuardTestConfig(t *testing.T) *fs.Config {
	t.Helper()
	t.Setenv("APP_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_API_BASE_NAME", "")
	t.Setenv("APP_DASH_BASE_NAME", "")
	t.Setenv("STORAGE", "")

	return &fs.Config{HideResourcesInfo: true, Dir: t.TempDir()}
}

// TestDashBaseNameMustDifferFromAPIBaseName: an equal name mounts the dash SPA
// fallback over the API namespace, swallowing every unmatched GET.
func TestDashBaseNameMustDifferFromAPIBaseName(t *testing.T) {
	config := newGuardTestConfig(t)
	config.APIBaseName = "api"
	config.DashBaseName = "api"

	_, err := New(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "APP_DASH_BASE_NAME")
}

func TestDashBaseNameDefaultsDoNotCollide(t *testing.T) {
	app, err := New(newGuardTestConfig(t))
	assert.NoError(t, err)
	assert.Equal(t, "api", app.Config().APIBaseName)
	assert.Equal(t, "dash", app.Config().DashBaseName)
}

// TestRootPublicPathWarns: publishing a disk at the root path stays allowed, it
// is a valid way to serve files on a dedicated domain. It only earns a warning.
func TestRootPublicPathWarns(t *testing.T) {
	config := newGuardTestConfig(t)
	config.StorageConfig = &fs.StorageConfig{
		DefaultDisk: "public",
		Disks: []*fs.DiskConfig{{
			Name:       "public",
			Driver:     "local",
			PublicPath: "/",
			Root:       t.TempDir(),
		}},
	}

	app, err := New(config)
	assert.NoError(t, err)

	warned := false
	for _, message := range app.startupMessages {
		if strings.Contains(message, "published at the root path") {
			warned = true
		}
	}
	assert.True(t, warned, "expected a startup warning for a disk published at the root path")
}

func TestNonRootPublicPathDoesNotWarn(t *testing.T) {
	config := newGuardTestConfig(t)
	config.StorageConfig = &fs.StorageConfig{
		DefaultDisk: "public",
		Disks: []*fs.DiskConfig{{
			Name:       "public",
			Driver:     "local",
			PublicPath: "/files",
			Root:       t.TempDir(),
		}},
	}

	app, err := New(config)
	assert.NoError(t, err)

	for _, message := range app.startupMessages {
		assert.NotContains(t, message, "published at the root path")
	}
}
