package app

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/godev90/validator"
	"github.com/godev90/validator/errors"
)

type HandlerFunc func(*Context) error

type MiddlewareFunc func(HandlerFunc) HandlerFunc

type routeEntry struct {
	handler    HandlerFunc
	middleware []MiddlewareFunc
}

type Router struct {
	prefix     string
	routes     map[string]map[string]routeEntry
	middleware []MiddlewareFunc
}

type App struct {
	router *Router
	mw     []MiddlewareFunc
}

func New() *App {
	r := &Router{
		routes: make(map[string]map[string]routeEntry),
	}
	return &App{router: r}
}

func (app *App) Route() *Router {
	return app.router
}

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := &Context{writer: w, request: r}
	method := r.Method
	path := r.URL.Path

	start := time.Now()

	var entry routeEntry
	var found bool
	for route, e := range app.router.routes[method] {
		if params, ok := matchRoute(route, path); ok {
			ctx.Params = params
			entry = e
			found = true
			break
		}
	}

	if !found {
		http.NotFound(w, r)
		return
	}

	final := entry.handler

	for i := len(entry.middleware) - 1; i >= 0; i-- {
		final = entry.middleware[i](final)
	}
	// Apply global app middleware
	for i := len(app.mw) - 1; i >= 0; i-- {
		final = app.mw[i](final)
	}

	var message = "success"
	if err := final(ctx); err != nil {
		message = err.Error()
	}

	stop := time.Now()
	log.Printf("%s [%d] %s %s (%s) %d milliseconds", ctx.Request().Method,
		ctx.httpStatus,
		ctx.Request().URL.Path,
		ctx.Request().RemoteAddr,
		message, stop.Sub(start).Milliseconds())
}

func (app *App) Use(mw ...MiddlewareFunc) {
	app.mw = append(app.mw, mw...)
}

func (r *Router) Group(prefix string, mws ...MiddlewareFunc) *Router {
	return &Router{
		prefix:     r.prefix + prefix,
		routes:     r.routes,
		middleware: append([]MiddlewareFunc{}, append(r.middleware, mws...)...),
	}
}

func (r *Router) handle(method, path string, h HandlerFunc, mws ...MiddlewareFunc) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]routeEntry)
	}
	// Simpan route dengan middleware chain (router group + route)
	allMiddleware := append([]MiddlewareFunc{}, r.middleware...)
	allMiddleware = append(allMiddleware, mws...)
	r.routes[method][path] = routeEntry{
		handler:    h,
		middleware: allMiddleware,
	}
}

func (r *Router) Use(mws ...MiddlewareFunc) {
	r.middleware = append(r.middleware, mws...)
}

func (r *Router) GET(path string, h HandlerFunc, mws ...MiddlewareFunc) {
	r.handle("GET", r.prefix+path, h, mws...)
}
func (r *Router) POST(path string, h HandlerFunc, mws ...MiddlewareFunc) {
	r.handle("POST", r.prefix+path, h, mws...)
}

func matchRoute(pattern, path string) (map[string]string, bool) {
	parts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	if len(parts) != len(pathParts) {
		return nil, false
	}
	params := make(map[string]string)
	for i := range parts {
		if strings.HasPrefix(parts[i], ":") {
			params[parts[i][1:]] = pathParts[i]
		} else if parts[i] != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

type Session interface {
	Get(key string) any
	Set(key string, value any)
	Delete(key string)
}

type Context struct {
	writer  http.ResponseWriter
	request *http.Request
	locale  errors.LanguageTag
	Params  map[string]string
	Session Session

	httpStatus int
}

func (c *Context) UseLocale(l errors.LanguageTag) {
	c.locale = l
}

func (c *Context) Locale() errors.LanguageTag {
	return c.locale
}

func (c *Context) JSON(code int, data any) error {
	c.writer.Header().Set("Content-Type", "application/json")
	c.writer.WriteHeader(code)
	return json.NewEncoder(c.writer).Encode(data)
}

func (c *Context) Request() *http.Request {
	return c.request
}

func (c *Context) Writer() http.ResponseWriter {
	return c.writer
}

func (c *Context) Success(data any) error {
	c.httpStatus = http.StatusOK

	c.JSON(http.StatusOK, map[string]any{
		"code": http.StatusOK,
		"data": data,
	})

	return nil
}

func (c *Context) Unauthorized(err error) error {
	c.httpStatus = http.StatusUnauthorized
	if er, ok := err.(*errors.Error); ok {
		c.JSON(http.StatusUnauthorized, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	}

	c.JSON(http.StatusUnauthorized, map[string]any{
		"code": http.StatusUnauthorized,
		"data": map[string]any{
			"description": fmt.Sprintf("unauthorized error: %s", err.Error()),
		},
	})

	return err
}

func (c *Context) BadInput(err error) error {
	c.httpStatus = http.StatusBadRequest
	if ers, ok := err.(errors.Errors); ok {
		c.JSON(http.StatusBadRequest, map[string]any{
			"code": http.StatusBadRequest,
			"data": ers.LocalizedError(c.locale),
		})
	} else {
		c.JSON(http.StatusBadRequest, map[string]any{
			"code": http.StatusBadRequest,
			"data": map[string]any{
				"description": fmt.Sprintf("bad input: %s", err.Error()),
			},
		})
	}

	return err
}

func (c *Context) NotAllowed(err error) error {
	c.httpStatus = http.StatusMethodNotAllowed
	if er, ok := err.(*errors.Error); ok {
		c.JSON(http.StatusMethodNotAllowed, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	}

	c.JSON(http.StatusMethodNotAllowed, map[string]any{
		"code": fmt.Sprintf("%d", http.StatusMethodNotAllowed),
		"error": map[string]any{
			"description": fmt.Sprintf("not allowed: %s", err.Error()),
		},
	})

	return err
}

func (c *Context) BadGateway(err error) error {
	c.httpStatus = http.StatusBadGateway
	if er, ok := err.(*errors.Error); ok {
		c.JSON(http.StatusBadGateway, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	}

	c.JSON(c.httpStatus, map[string]any{
		"code": fmt.Sprintf("%d", http.StatusBadGateway),
		"error": map[string]any{
			"description": fmt.Sprintf("bad gateway: %s", err.Error()),
		},
	})

	return err
}

func (c *Context) ServerError(err error) error {
	c.httpStatus = http.StatusInternalServerError
	if er, ok := err.(*errors.Error); ok {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	}

	c.JSON(c.httpStatus, map[string]any{
		"code": fmt.Sprintf("%d", http.StatusInternalServerError),
		"error": map[string]any{
			"description": fmt.Sprintf("server error: %s", err.Error()),
		},
	})

	return err
}

func (c *Context) Param(key string) string {
	return c.Params[key]
}

func (c *Context) Query(key string) string {
	return c.request.URL.Query().Get(key)
}

func (c *Context) Bind(dest any) error {

	defer c.request.Body.Close()
	if err := json.NewDecoder(c.request.Body).Decode(dest); err != nil {
		return err
	}

	if validate, ok := dest.(validator.Validator); ok {
		return validate.Validate()
	}

	return validator.ValidateStruct(dest)
}

func (c *Context) BindForm(dest any) error {
	if err := c.request.ParseForm(); err != nil {
		return err
	}
	return bindFormValues(c.request.Form, dest)
}

func (c *Context) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return c.request.FormFile(key)
}

func bindFormValues(values map[string][]string, dest any) error {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		structField := t.Field(i)
		formKey := structField.Tag.Get("form")
		if formKey == "" {
			continue
		}
		if val, ok := values[formKey]; ok && len(val) > 0 {
			switch field.Kind() {
			case reflect.String:
				field.SetString(val[0])
			case reflect.Int, reflect.Int64:
				i, _ := strconv.ParseInt(val[0], 10, 64)
				field.SetInt(i)
			case reflect.Float64:
				f, _ := strconv.ParseFloat(val[0], 64)
				field.SetFloat(f)
			case reflect.Bool:
				b, _ := strconv.ParseBool(val[0])
				field.SetBool(b)
			case reflect.Ptr:
				ptr := reflect.New(field.Type().Elem())
				switch field.Type().Elem().Kind() {
				case reflect.String:
					ptr.Elem().SetString(val[0])
				case reflect.Int, reflect.Int64:
					i, _ := strconv.ParseInt(val[0], 10, 64)
					ptr.Elem().SetInt(i)
				case reflect.Float64:
					f, _ := strconv.ParseFloat(val[0], 64)
					ptr.Elem().SetFloat(f)
				case reflect.Bool:
					b, _ := strconv.ParseBool(val[0])
					ptr.Elem().SetBool(b)
				}
				field.Set(ptr)
			}
		}
	}

	if validate, ok := dest.(validator.Validator); ok {
		return validate.Validate()
	}

	return validator.ValidateStruct(dest)
}
