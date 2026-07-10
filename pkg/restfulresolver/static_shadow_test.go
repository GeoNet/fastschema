package restfulresolver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/logger"
	"github.com/fastschema/fastschema/pkg/restfulresolver"
	"github.com/fastschema/fastschema/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// newShadowTestResolver mounts a disk at publicPath, an SPA at /dash, and an API
// handler at /api/config/brand. A file exists on disk at the very path the API
// handler answers, so a static mount that wins the match is directly observable.
func newShadowTestResolver(t *testing.T, publicPath string) *restfulresolver.RestfulResolver {
	t.Helper()

	publicDir := t.TempDir()
	assert.NoError(t, utils.MkDirs(publicDir+"/api/config"))
	utils.WriteFile(publicDir+"/api/config/brand", `{"iam":"a real file on disk"}`)

	dashDir := t.TempDir()
	assert.NoError(t, utils.MkDirs(dashDir+"/dash"))
	utils.WriteFile(dashDir+"/dash/index.html", "<html>spa</html>")

	resourceManager := fs.NewResourcesManager()
	resourceManager.Group("api").Group("config").
		Add(fs.NewResource("brand", func(c fs.Context, _ any) (fs.Map, error) {
			return fs.Map{"iam": "the api handler"}, nil
		}))

	return restfulresolver.NewRestfulResolver(&restfulresolver.ResolverConfig{
		ResourceManager: resourceManager,
		Logger:          logger.CreateMockLogger(true),
		StaticFSs: []*fs.StaticFs{{
			BasePath: publicPath,
			RootDir:  publicDir,
		}, {
			BasePath:     "/dash",
			RootFS:       http.Dir(dashDir),
			FSPrefix:     "dash",
			NotFoundFile: "dash/index.html",
		}},
	})
}

func getBody(t *testing.T, resolver *restfulresolver.RestfulResolver, path string) (int, string) {
	t.Helper()

	resp, err := resolver.Server().App.Test(httptest.NewRequest("GET", path, nil))
	assert.NoError(t, err)
	defer closeResponse(t, resp)

	return resp.StatusCode, utils.Must(utils.ReadCloserToString(resp.Body))
}

// TestStaticDoesNotShadowAPI pins the registration order: API routes are matched
// before any static mount. A disk published at the root path can otherwise serve
// an uploaded file in place of an API response, since the upload root and the API
// namespace then share the same URL space.
func TestStaticDoesNotShadowAPI(t *testing.T) {
	for _, publicPath := range []string{"/", "/files"} {
		t.Run("public path "+publicPath, func(t *testing.T) {
			resolver := newShadowTestResolver(t, publicPath)

			status, body := getBody(t, resolver, "/api/config/brand")
			assert.Equal(t, 200, status)
			assert.Equal(t, `{"data":{"iam":"the api handler"}}`, body)
		})
	}
}

// TestStaticMountBoundary keeps the SPA mount from swallowing sibling paths that
// merely share its prefix. Fiber matches a mount prefix without a path-segment
// boundary, so /dashboard would otherwise be served the dash SPA with status 200.
func TestStaticMountBoundary(t *testing.T) {
	resolver := newShadowTestResolver(t, "/files")

	for _, tt := range []struct {
		path       string
		wantStatus int
	}{
		{"/dash", 200},
		{"/dash/content/x", 200},
		// The first-run setup page is a deep link into the SPA.
		{"/dash/setup/", 200},
		{"/dashboard", 404},
		{"/files/nope", 404},
		{"/api/missing", 404},
		{"/filesystem", 404},
	} {
		t.Run(tt.path, func(t *testing.T) {
			status, _ := getBody(t, resolver, tt.path)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// TestStaticIsRegisteredLast documents the invariant the deleted status-reset
// middleware used to paper over: a static mount that misses leaves a 404 status
// on the context before calling Next. Nothing may be registered after the statics,
// or it would inherit that status. Registering statics last makes fiber's own
// not-found handler the only thing downstream.
func TestStaticIsRegisteredLast(t *testing.T) {
	resolver := newShadowTestResolver(t, "/files")

	// The API answers 200 even though a static mount sits in the same URL space
	// and would have set 404 had it been consulted first.
	status, _ := getBody(t, resolver, "/api/config/brand")
	assert.Equal(t, 200, status)

	// Stack is indexed by method, GET first, and holds the routes of that method
	// in registration order.
	routes := []string{}
	for _, route := range resolver.Server().App.Stack()[0] {
		routes = append(routes, route.Path)
	}

	lastAPI, firstStatic := -1, len(routes)
	for i, path := range routes {
		if path == "/api/config/brand" {
			lastAPI = i
		}
		if (path == "/files" || path == "/dash") && i < firstStatic {
			firstStatic = i
		}
	}

	assert.NotEqual(t, -1, lastAPI)
	assert.Less(t, lastAPI, firstStatic, "api routes must be registered before static mounts")
}
