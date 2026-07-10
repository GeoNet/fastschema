package restfulresolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathIsUnderMount(t *testing.T) {
	for _, tt := range []struct {
		path     string
		basePath string
		want     bool
	}{
		{"/dash", "/dash", true},
		{"/dash/content/x", "/dash", true},
		{"/dashboard", "/dash", false},
		{"/", "/dash", false},
		{"", "/dash", false},
		// A mount may be written with a trailing slash; it names the same prefix.
		{"/files/photo.png", "/files/", true},
		{"/files", "/files/", true},
		{"/filesystem", "/files/", false},
		// A disk published at the root serves everything but the root itself,
		// which keeps falling through to the API and the SPA.
		{"/anything", "/", true},
		{"/", "/", false},
	} {
		t.Run(tt.basePath+" "+tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, pathIsUnderMount(tt.path, tt.basePath))
		})
	}
}
