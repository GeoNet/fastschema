package restfulresolver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/logger"
	fserrors "github.com/fastschema/fastschema/pkg/errors"
	"github.com/fastschema/fastschema/pkg/restfulresolver"
	"github.com/fastschema/fastschema/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewares(t *testing.T) {
	mockLogger := logger.CreateMockLogger(true)
	server := restfulresolver.New(restfulresolver.Config{
		Logger: mockLogger,
	})
	server.Use(
		restfulresolver.MiddlewareCookie,
		restfulresolver.MiddlewareRequestID,
		restfulresolver.CreateMiddlewareRequestLog([]*fs.StaticFs{}),
		restfulresolver.MiddlewareCors,
		restfulresolver.MiddlewareRecover,
	)
	server.Get("/test", func(c *restfulresolver.Context) error {
		return errors.New("test error")
	})
	server.Get("/panic", func(c *restfulresolver.Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := server.Test(req)
	assert.NoError(t, err)
	defer closeResponse(t, resp)
	assert.Equal(t, 500, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("X-Request-Id"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Auth-Token, Range", resp.Header.Get("Access-Control-Allow-Headers"))
	assert.Equal(t, 1, len(mockLogger.Messages))
	assert.Contains(t, mockLogger.Last().String(), "Request completed")

	req2 := httptest.NewRequest("OPTIONS", "/not-found", nil)
	resp, err = server.Test(req2)
	assert.NoError(t, err)
	defer closeResponse(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	req3 := httptest.NewRequest("GET", "/panic", nil)
	resp, err = server.Test(req3)
	assert.NoError(t, err)
	defer closeResponse(t, resp)
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, `{"error":"test panic"}`, utils.Must(utils.ReadCloserToString(resp.Body)))
}

// requestLogs returns the log contexts of every "Request completed" entry.
func requestLogs(mockLogger *logger.MockLogger) []logger.LogContext {
	logContexts := []logger.LogContext{}
	for _, message := range mockLogger.Messages {
		if len(message.Params) < 2 {
			continue
		}

		msg, ok := message.Params[0].(string)
		if !ok || msg != "Request completed" {
			continue
		}

		if logContext, ok := message.Params[1].(logger.LogContext); ok {
			logContexts = append(logContexts, logContext)
		}
	}

	return logContexts
}

// newRequestLogTestResolver builds a resolver shaped like the real app: an API
// group, a RootDir static and a RootFS static with an SPA fallback file.
func newRequestLogTestResolver(t *testing.T, staticBasePath string) (*restfulresolver.RestfulResolver, *logger.MockLogger) {
	t.Helper()

	filesDir := t.TempDir()
	utils.WriteFile(filesDir+"/anh.png", "an image")

	dashDir := t.TempDir()
	assert.NoError(t, utils.MkDirs(dashDir+"/dash"))
	utils.WriteFile(dashDir+"/dash/index.html", "<html>spa</html>")

	resourceManager := fs.NewResourcesManager()
	resourceManager.Group("api").
		Add(fs.NewResource("ping", func(c fs.Context, _ any) (string, error) {
			return "pong", nil
		})).
		Add(fs.NewResource("boom", func(c fs.Context, _ any) (any, error) {
			return nil, fserrors.BadRequest("boom")
		}))

	mockLogger := logger.CreateMockLogger(true)
	resolver := restfulresolver.NewRestfulResolver(&restfulresolver.ResolverConfig{
		ResourceManager: resourceManager,
		Logger:          mockLogger,
		StaticFSs: []*fs.StaticFs{{
			BasePath: staticBasePath,
			RootDir:  filesDir,
		}, {
			BasePath:     "/dash",
			RootFS:       http.Dir(dashDir),
			FSPrefix:     "dash",
			NotFoundFile: "dash/index.html",
		}},
	})

	return resolver, mockLogger
}

// TestMiddlewareRequestLogStatus locks the logged status to the status actually
// sent to the client. Fiber writes the status of a returned error in its error
// handler, which runs after this middleware has unwound, so the response object
// cannot be trusted on the error path.
func TestMiddlewareRequestLogStatus(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"handler success", "/api/ping", 200},
		{"resolver writes the error response itself", "/api/boom", 400},
		{"unmatched route returns a fiber error", "/khong-ton-tai", 404},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resolver, mockLogger := newRequestLogTestResolver(t, "/files")

			resp, err := resolver.Server().App.Test(httptest.NewRequest("GET", tt.path, nil))
			assert.NoError(t, err)
			defer closeResponse(t, resp)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			logs := requestLogs(mockLogger)
			require.Len(t, logs, 1)
			assert.Equal(t, tt.wantStatus, logs[0]["status"])
		})
	}
}

// TestMiddlewareRequestLogStatusFromError covers the status sources that the app
// routes do not exercise: a fiber error carrying its own code, an errors.Error
// without a status, and a plain error.
func TestMiddlewareRequestLogStatusFromError(t *testing.T) {
	for _, tt := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"fiber error", fiber.ErrTeapot, 418},
		{"errors.Error with status", fserrors.Unauthorized("nope"), 401},
		{"errors.Error without status", fserrors.New("no status"), 500},
		{"plain error", errors.New("plain"), 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := logger.CreateMockLogger(true)
			server := restfulresolver.New(restfulresolver.Config{Logger: mockLogger})
			server.Use(restfulresolver.CreateMiddlewareRequestLog(nil))
			server.Get("/fail", func(c *restfulresolver.Context) error {
				return tt.err
			})

			resp, err := server.Test(httptest.NewRequest("GET", "/fail", nil))
			assert.NoError(t, err)
			defer closeResponse(t, resp)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			logs := requestLogs(mockLogger)
			require.Len(t, logs, 1)
			assert.Equal(t, tt.wantStatus, logs[0]["status"])
		})
	}
}

// TestMiddlewareRequestLogSkipsServedStatics asserts the skip rule keys off the
// route that actually served the request, not off a path prefix. A path that
// merely shares a prefix with a static mount, or a miss under a static mount,
// must still be logged: those are exactly the probes worth seeing.
func TestMiddlewareRequestLogSkipsServedStatics(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		wantStatus int
		wantLogged bool
	}{
		{"api route", "/api/ping", 200, true},
		{"file served by static", "/files/anh.png", 200, false},
		{"spa deep link served by static", "/dash/content/x", 200, false},
		{"missing file under static", "/files/thieu.png", 404, true},
		{"path sharing a prefix with a static", "/filesystem", 404, true},
		{"unmatched route", "/khong-ton-tai", 404, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resolver, mockLogger := newRequestLogTestResolver(t, "/files")

			resp, err := resolver.Server().App.Test(httptest.NewRequest("GET", tt.path, nil))
			assert.NoError(t, err)
			defer closeResponse(t, resp)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			logs := requestLogs(mockLogger)
			if !tt.wantLogged {
				assert.Empty(t, logs)
				return
			}

			require.Len(t, logs, 1)
			assert.Equal(t, tt.wantStatus, logs[0]["status"])
			assert.Equal(t, tt.path, logs[0]["path"])
		})
	}
}

// TestMiddlewareRequestLogRootStaticKeepsLogging guards the configuration where a
// disk is published at the root path: it must not silence the log of every route.
func TestMiddlewareRequestLogRootStaticKeepsLogging(t *testing.T) {
	resolver, mockLogger := newRequestLogTestResolver(t, "/")

	resp, err := resolver.Server().App.Test(httptest.NewRequest("GET", "/api/ping", nil))
	assert.NoError(t, err)
	defer closeResponse(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	logs := requestLogs(mockLogger)
	require.Len(t, logs, 1)
	assert.Equal(t, 200, logs[0]["status"])
}

// TestMiddlewareRequestLogRoutePathIsStaticBasePath pins the fiber behaviour the
// skip rule depends on: after Next, Route().Path of a static mount is its mount
// path, for both the RootDir and the RootFS shapes. A fiber upgrade that changes
// this must fail here rather than silently drop or add logs.
func TestMiddlewareRequestLogRoutePathIsStaticBasePath(t *testing.T) {
	routePaths := map[string]string{}
	app := fiber.New(fiber.Config{CaseSensitive: true})
	app.Use(func(c *fiber.Ctx) error {
		// c.Path() views the request buffer, which fiber reuses for the next
		// request. Clone it before keeping it past the handler.
		path := strings.Clone(c.Path())
		err := c.Next()
		routePaths[path] = strings.Clone(c.Route().Path)
		return err
	})

	filesDir := t.TempDir()
	utils.WriteFile(filesDir+"/anh.png", "an image")
	dashDir := t.TempDir()
	assert.NoError(t, utils.MkDirs(dashDir+"/dash"))
	utils.WriteFile(dashDir+"/dash/index.html", "<html>spa</html>")

	app.Static("/files", filesDir)
	app.Use("/dash", filesystem.New(filesystem.Config{
		Root:         http.Dir(dashDir),
		PathPrefix:   "dash",
		NotFoundFile: "dash/index.html",
	}))

	for _, path := range []string{"/files/anh.png", "/files/thieu.png", "/dash/content/x"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		assert.NoError(t, err)
		closeResponse(t, resp)
	}

	assert.Equal(t, "/files", routePaths["/files/anh.png"])
	assert.Equal(t, "/files", routePaths["/files/thieu.png"])
	assert.Equal(t, "/dash", routePaths["/dash/content/x"])
}
