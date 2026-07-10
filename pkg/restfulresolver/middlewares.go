package restfulresolver

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/logger"
	"github.com/google/uuid"
)

const HeaderRequestID = "X-Request-Id"

func MiddlewareRequestID(c *Context) error {
	requestID := c.Header(HeaderRequestID)
	if requestID == "" {
		requestIDUUID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		requestID = requestIDUUID.String()
	}

	c.Local(fs.TraceID, requestID)
	c.Header(HeaderRequestID, requestID)

	return c.Next()
}

// responseStatus reports the status that will actually reach the client. On the
// error path the response is not written yet: the error handler runs after every
// middleware has unwound, so the error itself is the only source of truth.
func responseStatus(c *Context, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}

	return ErrorStatus(err)
}

func CreateMiddlewareRequestLog(statics []*fs.StaticFs) func(c *Context) error {
	staticRoutes := make(map[string]bool, len(statics))
	for _, static := range statics {
		staticRoutes[static.BasePath] = true
	}

	return func(c *Context) error {
		start := time.Now()
		err := c.Next()
		status := responseStatus(c, err)

		// Static assets are served on every page view and would drown the log.
		// Skip only what a static mount actually served: c.Route() is read after
		// Next, so it names the route that handled the request rather than a path
		// that merely shares a prefix with a mount. A miss under a mount, or an
		// error, still gets logged: those are the requests worth seeing.
		if err == nil && status < 400 && staticRoutes[c.ctx.Route().Path] {
			return nil
		}

		latency := time.Since(start).Round(time.Microsecond)
		logContext := logger.LogContext{
			"latency": latency.String(),
			"status":  status,
			"method":  c.Method(),
			"path":    c.Path(),
			"ip":      c.IP(),
			"queries": c.ctx.Queries(),
		}

		if err != nil {
			logContext["error"] = err.Error()
		}

		c.Logger().Info("Request completed", logContext)
		return err
	}
}

func MiddlewareCookie(c *Context) error {
	if c.Cookie("UUID") == "" {
		uuidValue, err := uuid.NewV7()
		if err != nil {
			return err
		}
		exp := time.Now().Add(time.Hour * 100 * 365 * 24)
		c.Cookie("UUID", &fs.Cookie{
			Name:     "UUID",
			Value:    uuidValue.String(),
			Expires:  exp,
			HTTPOnly: false,
			SameSite: "lax",
			Secure:   true,
		})
	}
	return c.Next()
}

func MiddlewareCors(c *Context) error {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Auth-Token, Range")

	if c.Method() == "OPTIONS" {
		c.Status(200)
		return nil
	}

	return c.Next()
}

func MiddlewareRecover(c *Context) error {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}

			stack := make([]byte, 4<<10)
			length := runtime.Stack(stack, true)
			msg := fmt.Sprintf("%v %s\n", err, stack[:length])
			c.Logger().Error(msg, logger.LogContext{"recovered": true})
			if err := c.Status(http.StatusBadRequest).JSON(fs.Map{"error": err.Error()}); err != nil {
				c.Logger().Error(err)
			}
		}
	}()

	return c.Next()
}
