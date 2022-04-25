package httpapp

import (
	"net/http"
	"sync"

	"github.com/costal/go-misc-tools/middler"
)

var currentURM *uriRestMethods

// this struct must correspond with restMethods in order to reflect values upon it
type uriRestMethods struct {
	uri        string
	methodsSet bool
	Post       methods
	Get        methods
	Put        methods
	Patch      methods
	Delete     methods
	methods    []*methods
}

type methods struct {
	methodName string
	method     http.HandlerFunc
	middleware []middler.Middleware
}

type httpMethods struct {
	methods      []*uriRestMethods
	mapToMethods map[string]*uriRestMethods
}

func (h *httpMethods) URL(url string, method ...*methods) *uriRestMethods {
	urm, ok := h.mapToMethods[url]
	if !ok {
		urm = &uriRestMethods{uri: url}
		urm.methods = []*methods{&urm.Post, &urm.Get, &urm.Put, &urm.Patch, &urm.Delete}
		h.mapToMethods[url] = urm
		h.methods = append(h.methods, urm)
	}
	currentURM = urm
	return urm.Method(method...)
}

func (urm *uriRestMethods) Method(method ...*methods) *uriRestMethods {
	// Point for optimization
	for len(method) != 0 {
		for _, val := range currentURM.methods {
			if val.methodName == method[0].methodName {
				val.method = method[0].method
				val.middleware = method[0].middleware
				method = method[1:]
			} else {
				val.middleware = nil
			}
		}
	}
	return urm
}

func (urm *uriRestMethods) Middleware(middleware ...middler.Middleware) *uriRestMethods {
	var funcs []func(wg sync.WaitGroup)
	// Point for optimization
	for _, val := range urm.methods {
		if urm.methodsSet {
			if val.methodName != "" {
				funcs = append(funcs, func(wg sync.WaitGroup) {
					defer wg.Done()
					val.middleware = append(val.middleware, middleware...)
				})
			} else {
				val.middleware = nil
			}
		} else {
			funcs = append(funcs, func(wg sync.WaitGroup) {
				defer wg.Done()
				val.middleware = append(val.middleware, middleware...)
			})
		}
	}

	var wg sync.WaitGroup
	for _, val := range funcs {
		wg.Add(1)
		go val(wg)
	}
	wg.Wait()
	return urm
}

func createHTTPMethod(name string, handler http.HandlerFunc) *methods {
	if handler == nil {
		panic("The handler function must not be nil!")
	}
	var m *methods
	switch name {
	case "POST":
		m = &currentURM.Post
	case "GET":
		m = &currentURM.Get
	case "PUT":
		m = &currentURM.Put
	case "PATCH":
		m = &currentURM.Patch
	case "DELETE":
		m = &currentURM.Delete
	default:
		panic("Wrong http method")
	}
	m.methodName = name
	m.method = handler
	return m
}

type httpMethodRoute struct{}

func GetHTTPMethods() httpMethodRoute {
	return httpMethodRoute{}
}

func (h httpMethodRoute) Post(handler http.HandlerFunc) *methods {
	return createHTTPMethod("POST", handler)
}

func (h httpMethodRoute) Get(handler http.HandlerFunc) *methods {
	return createHTTPMethod("GET", handler)
}

func (h httpMethodRoute) Put(handler http.HandlerFunc) *methods {
	return createHTTPMethod("PUT", handler)
}

func (h httpMethodRoute) Patch(handler http.HandlerFunc) *methods {
	return createHTTPMethod("PATCH", handler)
}

func (h httpMethodRoute) Delete(handler http.HandlerFunc) *methods {
	return createHTTPMethod("DELETE", handler)
}

func (m *methods) Middleware(middleware ...middler.Middleware) *methods {
	m.middleware = middleware
	return m
}
