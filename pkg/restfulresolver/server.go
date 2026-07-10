package restfulresolver

import (
	"net/http"

	"github.com/fastschema/fastschema/fs"
	"github.com/fastschema/fastschema/logger"
	"github.com/fastschema/fastschema/pkg/errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/utils"
)

type Server struct {
	*fiber.App

	logger logger.Logger
}

type Config struct {
	AppName            string
	MaxRequestBodySize int
	JSONEncoder        utils.JSONMarshal
	Logger             logger.Logger
}

// ErrorStatus reports the HTTP status an error is served with. It is the single
// source of truth shared by the error handler, which writes the status, and the
// request log, which reports it: fiber writes the status only after every
// middleware has unwound, so the log cannot read it back from the response.
func ErrorStatus(err error) int {
	var appErr *errors.Error
	if errors.As(err, &appErr) && appErr.Status != 0 {
		return appErr.Status
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code
	}

	return fiber.StatusInternalServerError
}

// errorHandler mirrors fiber.DefaultErrorHandler but also honors the status of
// an errors.Error. Without it an errors.Unauthorized returned to fiber would be
// served as 500, because fiber only recognizes its own error type.
func errorHandler(c *fiber.Ctx, err error) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	return c.Status(ErrorStatus(err)).SendString(err.Error())
}

func New(config Config) *Server {
	app := fiber.New(fiber.Config{
		AppName:               config.AppName,
		StrictRouting:         false,
		CaseSensitive:         true,
		EnablePrintRoutes:     false,
		DisableStartupMessage: true,
		JSONEncoder:           config.JSONEncoder,
		BodyLimit:             config.MaxRequestBodySize,
		ErrorHandler:          errorHandler,
		// Prefork:       true,
		// Immutable:     true,
	})

	app.Hooks().OnListen(func(listenData fiber.ListenData) error {
		address := listenData.Host + ":" + listenData.Port
		if config.Logger != nil {
			config.Logger.Info("Server listening on " + address)
		}

		return nil
	})

	app.Hooks().OnShutdown(func() error {
		config.Logger.Info("Server is shutting down")
		return nil
	})

	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // 1
	}))

	return &Server{
		App:    app,
		logger: config.Logger,
	}
}

func (s *Server) Test(req *http.Request, msTimeout ...int) (resp *http.Response, err error) {
	return s.App.Test(req, msTimeout...)
}

func (s *Server) Listen(address string) error {
	return s.App.Listen(address)
}

func (s *Server) Use(handlers ...Handler) {
	middlewares := []any{}
	transformedHandlers := TransformHandlers(nil, handlers, s.logger)
	for _, handler := range transformedHandlers {
		middlewares = append(middlewares, handler)
	}

	s.App.Use(middlewares...)
}

func (s *Server) Group(prefix string, r *fs.Resource, handlers ...Handler) *Router {
	var fiberHandlers []fiber.Handler

	for _, handler := range handlers {
		fiberHandlers = append(fiberHandlers, func(c *fiber.Ctx) error {
			return handler(CreateContext(r, c, s.logger))
		})
	}

	g := s.App.Group(prefix, fiberHandlers...).(*fiber.Group)

	return &Router{
		fiberGroup: g,
		App:        s.App,
		logger:     s.logger,
	}
}

func (s *Server) Static(prefix, root string, configs ...fs.StaticConfig) {
	config := fiber.Static{}

	if len(configs) > 0 {
		config = fiber.Static{
			Index:         configs[0].Index,
			Browse:        configs[0].Browse,
			MaxAge:        configs[0].MaxAge,
			Compress:      configs[0].Compress,
			ByteRange:     configs[0].ByteRange,
			Download:      configs[0].Download,
			CacheDuration: configs[0].CacheDuration,
		}
	}
	s.App.Static(prefix, root, config)
}

func (s *Server) Get(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Get(path, handlers...).Name(name)
}

func (s *Server) Head(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Head(path, handlers...).Name(name)
}

func (s *Server) Post(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Post(path, handlers...).Name(name)
}

func (s *Server) Put(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Put(path, handlers...).Name(name)
}

func (s *Server) Delete(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Delete(path, handlers...).Name(name)
}

func (s *Server) Connect(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Connect(path, handlers...).Name(name)
}

func (s *Server) Options(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Options(path, handlers...).Name(name)
}

func (s *Server) Trace(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Trace(path, handlers...).Name(name)
}

func (s *Server) Patch(path string, handler Handler, resources ...*fs.Resource) {
	name, handlers := GetHandlerInfo(handler, s.logger, resources...)
	s.App.Patch(path, handlers...).Name(name)
}
