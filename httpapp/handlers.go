package httpapp

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/costal/go-misc-tools/middler"
)

type HTTPHandlers struct {
	AuthenticationFunc func()
	DynamicMiddleware  []middler.Middleware
	standardMiddleware []middler.Middleware
	mux                *http.ServeMux
	httpMethods
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
				app.serverError(w, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *Application) restRestrict(m *restMethods) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var next http.Handler
		v := reflect.ValueOf(m)
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsNil() {
				switch i {
				case 0:
					w.Header().Add("Allow", http.MethodPost)
					if r.Method == http.MethodPost {
						next = m.Post
					}
				case 1:
					w.Header().Add("Allow", http.MethodGet)
					if r.Method == http.MethodGet {
						next = m.Get
					}
				case 2:
					w.Header().Add("Allow", http.MethodPut)
					if r.Method == http.MethodPut {
						next = m.Put
					}
				case 3:
					w.Header().Add("Allow", http.MethodPatch)
					if r.Method == http.MethodPatch {
						next = m.Patch
					}
				case 4:
					w.Header().Add("Allow", http.MethodDelete)
					if r.Method == http.MethodDelete {
						next = m.Delete
					}
				}
			}
		}
		if next == nil {
			app.clientError(w, http.StatusMethodNotAllowed)
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
			if rm.middleware != nil {
				for _, m := range rm.middleware {
					dynamicMiddleware = dynamicMiddleware.Add(m)
				}
				*method = dynamicMiddleware.ThenFunc(rm.method)
			} else {
				*method = rm.method
			}
		}
	}

	// Point for optimization
	for _, urm := range app.urmMethods {
		rest := &restMethods{}
		rvPtr := reflect.ValueOf(rest)
		urv := reflect.ValueOf(urm)
		for i := 0; i < rvPtr.Elem().NumField(); i++ {
			m := rvPtr.Elem().Field(i).Addr().Interface().(*http.Handler)
			rm := urv.Field(i + 2).Interface().(methods)
			commitMethod(m, &rm)
		}
		app.mux.Handle(urm.uri, app.restRestrict(rest))
	}

	return standardMiddleware.Then(app.mux)
}
