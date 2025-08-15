package app

import (
	"encoding/json"
	"log"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/godev90/validator"
	"github.com/godev90/validator/faults"
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

var validSession map[SessionType]reflect.Type

type SessionType uint
type Session interface {
	Identifier() string
	Type() SessionType
}
type Context struct {
	writer  http.ResponseWriter
	request *http.Request
	locale  faults.LanguageTag
	Params  map[string]string
	session Session

	httpStatus int
}

func RegisterSessionType(session Session) {
	if session == nil {
		panic(faults.ErrCannotBeNull)
	}

	if validSession == nil {
		validSession = make(map[SessionType]reflect.Type)
	}

	modelType := reflect.TypeOf(session)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	existing, exists := validSession[session.Type()]
	if exists {
		if existing != modelType {
			panic(faults.ErrConflict)
		}
	}

	validSession[session.Type()] = modelType
}

func (c *Context) Session() Session {
	return c.session
}

func (c *Context) SetSession(session Session) {
	c.session = session
}

func (c *Context) FetchSession(dst any) error {
	if c.session == nil {
		return faults.ErrUnauthorized
	}

	if dst == nil {
		return faults.ErrCannotBeNull
	}

	expectedType, ok := validSession[c.session.Type()]
	if !ok {
		return faults.ErrUnauthorized
	}

	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Ptr || dstVal.IsNil() {
		return faults.ErrTypeMismatch
	}

	dstElem := dstVal.Elem()
	srcVal := reflect.ValueOf(c.session)

	if srcVal.Type().Kind() == reflect.Ptr && srcVal.Type().Elem() != expectedType {
		return faults.ErrTypeMismatch
	}
	if srcVal.Type().Kind() != reflect.Ptr && srcVal.Type() != expectedType {
		return faults.ErrTypeMismatch
	}

	// Pastikan src bisa ditransfer ke dst
	if !srcVal.Type().AssignableTo(dstElem.Type()) {
		return faults.ErrTypeMismatch
	}

	dstElem.Set(srcVal)
	return nil
}

func (c *Context) UseLocale(l faults.LanguageTag) {
	c.locale = l
}

func (c *Context) Locale() faults.LanguageTag {
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
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusUnauthorized, map[string]any{
			"code": http.StatusUnauthorized,
			"data": ers.LocalizedError(c.locale),
		})
	} else if er, ok := err.(faults.Error); ok {
		c.JSON(http.StatusUnauthorized, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})
	} else {
		c.JSON(http.StatusUnauthorized, map[string]any{
			"code": http.StatusUnauthorized,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) BadInput(err error) error {
	c.httpStatus = http.StatusBadRequest
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusBadRequest, map[string]any{
			"code": http.StatusBadRequest,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusBadRequest, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusBadRequest, map[string]any{
			"code": http.StatusBadRequest,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) NotFound(err error) error {
	c.httpStatus = http.StatusNotFound
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusNotFound, map[string]any{
			"code": http.StatusNotFound,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusNotFound, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusNotFound, map[string]any{
			"code": http.StatusNotFound,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) Forbidden(err error) error {
	c.httpStatus = http.StatusForbidden
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusForbidden, map[string]any{
			"code": http.StatusForbidden,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusForbidden, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusForbidden, map[string]any{
			"code": http.StatusForbidden,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) TooManyRequest(err error) error {
	c.httpStatus = http.StatusTooManyRequests
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusTooManyRequests, map[string]any{
			"code": http.StatusTooManyRequests,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusTooManyRequests, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusTooManyRequests, map[string]any{
			"code": http.StatusTooManyRequests,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) Conflict(err error) error {
	c.httpStatus = http.StatusConflict
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusConflict, map[string]any{
			"code": http.StatusConflict,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusConflict, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusConflict, map[string]any{
			"code": http.StatusConflict,
			"data": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) NotAllowed(err error) error {
	c.httpStatus = http.StatusMethodNotAllowed
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusMethodNotAllowed, map[string]any{
			"code": http.StatusMethodNotAllowed,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusMethodNotAllowed, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(http.StatusMethodNotAllowed, map[string]any{
			"code": http.StatusMethodNotAllowed,
			"error": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) BadGateway(err error) error {
	c.httpStatus = http.StatusBadGateway
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusBadGateway, map[string]any{
			"code": http.StatusBadGateway,
			"data": ers.LocalizedError(c.locale),
		})
	} else if ers, ok := err.(faults.Error); ok {
		c.JSON(http.StatusBadGateway, map[string]any{
			"code": ers.Code(),
			"data": map[string]any{
				"description": ers.LocalizedError(c.locale),
			},
		})
	} else {
		c.JSON(c.httpStatus, map[string]any{
			"code": http.StatusBadGateway,
			"error": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) Unavailable(err error) error {
	c.httpStatus = http.StatusServiceUnavailable
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusServiceUnavailable, map[string]any{
			"code": http.StatusServiceUnavailable,
			"data": ers.LocalizedError(c.locale),
		})
	} else if er, ok := err.(faults.Error); ok {
		c.JSON(http.StatusServiceUnavailable, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	} else {
		c.JSON(c.httpStatus, map[string]any{
			"code": http.StatusServiceUnavailable,
			"error": map[string]any{
				"description": err.Error(),
			},
		})
	}

	return err
}

func (c *Context) ServerError(err error) error {
	c.httpStatus = http.StatusInternalServerError
	if ers, ok := err.(faults.Errors); ok {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"code": http.StatusInternalServerError,
			"data": ers.LocalizedError(c.locale),
		})
	} else if er, ok := err.(faults.Error); ok {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"code": er.Code(),
			"data": map[string]any{
				"description": er.LocalizedError(c.locale),
			}})

		return err
	} else {
		c.JSON(c.httpStatus, map[string]any{
			"code": http.StatusInternalServerError,
			"error": map[string]any{
				"description": err.Error(),
			},
		})
	}

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
