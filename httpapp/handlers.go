package httpapp

import (
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/costal/go-misc-tools/middler"
	"github.com/rs/cors"
)

type HTTPHandlers struct {
	CorsOptions        corsOptions
	AuthenticationFunc func()
	OriginValidator    func(string) bool
	DynamicMiddleware  []middler.Middleware
	standardMiddleware []middler.Middleware
	mux                *http.ServeMux
	muxTree            map[string]*http.ServeMux
	httpMethods
}

type corsOptions struct {
	allowedOrigins         []string
	allowOriginFunc        func(string) bool
	AllowOriginRequestFunc func(*http.Request, string) bool
	allowedMethods         []string
	allowedHeaders         []string
	ExposedHeaders         []string
	AllowCredentials       bool
	MaxAge                 int
	corsHandler            middler.Middleware
}

type restMethods struct {
	Post   http.Handler
	Get    http.Handler
	Put    http.Handler
	Patch  http.Handler
	Delete http.Handler
}

func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.AuthenticationFunc()
		next.ServeHTTP(w, r)
	})
}

func (app *Application) Cors(allowedHeaders []string) middler.Middleware {
	if app.CorsOptions.corsHandler == nil {
		allowedHeaders = append(allowedHeaders, "access-control-allow-origin")
		c := cors.New(cors.Options{
			AllowOriginRequestFunc: app.CorsOptions.AllowOriginRequestFunc,
			ExposedHeaders:         app.CorsOptions.ExposedHeaders,
			AllowedHeaders:         allowedHeaders,
			AllowCredentials:       app.CorsOptions.AllowCredentials,
		})
		app.CorsOptions.corsHandler = c.Handler
	}
	return app.CorsOptions.corsHandler
}

func (app *Application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.infoLog.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

func (app *Application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.ServerError(w, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *Application) restRestrict(m *restMethods) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var next http.Handler
		var allowedMethods []string
		v := reflect.ValueOf(m).Elem()
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsNil() {
				switch i {
				case 0:
					allowedMethods = append(allowedMethods, "POST")
					if r.Method == http.MethodPost {
						next = m.Post
					}
				case 1:
					allowedMethods = append(allowedMethods, "GET")
					if r.Method == http.MethodGet {
						next = m.Get
					}
				case 2:
					allowedMethods = append(allowedMethods, "PUT")
					if r.Method == http.MethodPut {
						next = m.Put
					}
				case 3:
					allowedMethods = append(allowedMethods, "PATCH")
					if r.Method == http.MethodPatch {
						next = m.Patch
					}
				case 4:
					allowedMethods = append(allowedMethods, "DELETE")
					if r.Method == http.MethodDelete {
						next = m.Delete
					}
				}
			}
		}
		if next == nil {
			w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
			app.ClientError(w, http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *Application) Router() http.Handler {
	standardMiddleware := middler.New(app.standardMiddleware...)
	dynamicMiddleware := middler.New(app.DynamicMiddleware...)

	commitMethod := func(method *http.Handler, rm *methods) {
		if rm.method != nil {
			if len(rm.middleware) != 0 {
				for _, m := range rm.middleware {
					dynamicMiddleware = dynamicMiddleware.Add(m)
				}
				*method = dynamicMiddleware.Then(rm.method)
			} else {
				*method = rm.method
			}
		}
	}

	// Point for optimization
	for _, urm := range app.urmMethods {
		rest := &restMethods{}
		rvPtr := reflect.ValueOf(rest)
		urvPtr := reflect.ValueOf(urm)
		for i := 0; i < rvPtr.Elem().NumField(); i++ {
			m := rvPtr.Elem().Field(i).Addr().Interface().(*http.Handler)
			rm := urvPtr.Elem().Field(i + 2).Interface().(methods)
			commitMethod(m, &rm)
		}

		app.compileMux(urm.uri, urm.standardMiddleware, app.restRestrict(rest))
	}

	return standardMiddleware.Then(app.mux)
}

func (app *Application) compileMux(route string, middleware []middler.Middleware, handler http.Handler) {
	routes := strings.Split(route, "/")
	if routes[len(routes)-1] == "" {
		routes = routes[:len(routes)-1]
	}
	currentMux := app.mux
	isNotMux := !hasMux(route, true)
	var routeBuilder strings.Builder
	for i, r := range routes {
		r = strings.Replace(r, "", "/", 1)
		routeBuilder.WriteString(r)
		muxString := strings.TrimSpace(path.Join(routeBuilder.String(), " "))
		if !isNotMux || i != len(routes)-1 {
			if muxString == "/" || hasMux(muxString, true) {
				if _, ok := app.muxTree[muxString]; !ok {
					mux := http.NewServeMux()
					app.muxTree[muxString] = mux
					if route == muxString && len(middleware) > 0 {
						handler = http.StripPrefix(
							muxString[:len(muxString)-1],
							middler.New(middleware...).Then(mux),
						)
					} else {
						handler = mux
					}
					currentMux.Handle(muxString, handler)
				}
				currentMux = app.muxTree[muxString]
			}
		} else if isNotMux {
			if len(middleware) > 0 {
				handler = middler.New(middleware...).Then(handler)
			}
			currentMux.Handle(r, handler)
		}
	}
}
