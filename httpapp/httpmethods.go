package httpapp

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"github.com/costal/go-misc-tools/middler"
)

var currentURM *uriRestMethods

// this struct must correspond with restMethods in order to reflect values upon it
type uriRestMethods struct {
	uri                string
	methodsSet         bool
	Post               methods
	Get                methods
	Put                methods
	Patch              methods
	Delete             methods
	methods            []*methods
	standardMiddleware []middler.Middleware
	isMux              bool
	*httpMethods
}

type methods struct {
	methodName string
	method     http.HandlerFunc
	middleware []middler.Middleware
}

type httpMethods struct {
	urmMethods []*uriRestMethods
	urmMap     map[string]*uriRestMethods
}

type containerMethod func(...middler.Middleware) interface{}

func hasMux(route string, muxOnly bool) bool {
	isMux := regexp.MustCompile("^[/][[:word:]]+([/][[:word:]]+)*[/]$")
	isRoute := regexp.MustCompile("[/][[:word:]]+([/][[:word:]]+)*$")
	if muxOnly {
		return isMux.MatchString(route)
	}
	return isMux.MatchString(route) || isRoute.MatchString(route)
}

func flattenContainers(cms []containerMethod) (ms []*methods) {
	ms = make([]*methods, len(cms))
	var wg sync.WaitGroup
	// Point for optimization
	for i, val := range cms {
		wg.Add(1)
		go func(i int, cm containerMethod) {
			defer wg.Done()
			if m, ok := cm(nil).(*methods); ok {
				ms[i] = m
			} else {
				m := cm(nil).(containerMethod)
				for mi, ok := m(nil).(containerMethod); ok; {
					mi, ok = mi(nil).(containerMethod)
					if !ok {
						ms[i] = mi(nil).(*methods)
					}
				}
			}
		}(i, val)
	}
	wg.Wait()
	return
}

func getURIRestMethods(h *httpMethods, url string) *uriRestMethods {
	urm, ok := h.urmMap[url]
	if !ok {
		urm = &uriRestMethods{uri: url, isMux: hasMux(url, true)}
		urm.methods = []*methods{&urm.Post, &urm.Get, &urm.Put, &urm.Patch, &urm.Delete}
		h.urmMap[url] = urm
		h.urmMethods = append(h.urmMethods, urm)
		urm.httpMethods = h
	}
	return urm
}

func (h *httpMethods) URL(url string, method ...containerMethod) *uriRestMethods {
	currentURM = getURIRestMethods(h, url)
	return currentURM.Method(flattenContainers(method)...)
}

func (urm *uriRestMethods) URL(url string, method ...containerMethod) *uriRestMethods {
	currentURM = getURIRestMethods(urm.httpMethods, url)
	return currentURM.Method(flattenContainers(method)...)
}

func (urm *uriRestMethods) Method(method ...*methods) *uriRestMethods {
	// Point for optimization
	for len(method) != 0 {
		for _, val := range currentURM.methods {
			// Proceed based on currentURM methods in order to erase
			// unnecessary middlewares - hence, even if len(method) = 0
			// continue
			if len(method) != 0 && val.methodName == method[0].methodName {
				val.method = method[0].method
				val.middleware = method[0].middleware
				method = method[1:]
				if !urm.methodsSet {
					urm.methodsSet = true
				}
			}
		}
	}
	return urm
}

func (c containerMethod) Middleware(middleware ...middler.Middleware) containerMethod {
	return func(m1 ...middler.Middleware) containerMethod {
		return func(m2 ...middler.Middleware) interface{} {
			m2 = append(m1, m2...)
			return c(m2...)
		}
	}(middleware...)
}

func (urm *uriRestMethods) Middleware(middleware ...middler.Middleware) *uriRestMethods {
	// Point for optimization
	var wg sync.WaitGroup

	if urm.isMux {
		urm.standardMiddleware = middleware
	} else {
		for _, val := range urm.methods {
			// Add to specific methods or add to all of them
			fmt.Println(urm.uri, 1, val)
			if val.methodName != "" || !urm.methodsSet {
				wg.Add(1)
				go func(m *methods) {
					defer wg.Done()
					m.middleware = append(m.middleware, middleware...)
				}(val)
			} else {
				val.middleware = nil
			}
		}

		wg.Wait()
	}

	return urm
}

type httpMethodRoute struct{}

func HTTPMethods() httpMethodRoute {
	return httpMethodRoute{}
}

func filterOutNilMiddleware(middlewares []middler.Middleware) []middler.Middleware {
	// Point for optimization: in-place was not chosen for prototype because
	// optimization will likely require a secondary array
	nm := make([]middler.Middleware, 0)
	for _, val := range middlewares {
		if val != nil {
			nm = append(nm, val)
		}
	}
	return nm
}

func createHTTPMethod(name string, handler http.HandlerFunc, middleware ...middler.Middleware) *methods {
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
	m.middleware = filterOutNilMiddleware(middleware)
	return m
}

func httpMethodContainerGenerator(method string, handler http.HandlerFunc) containerMethod {
	return func(m ...middler.Middleware) interface{} {
		return func(middleware ...middler.Middleware) *methods {
			return createHTTPMethod(method, handler, middleware...)
		}(m...)
	}
}

func (h httpMethodRoute) Post(handler http.HandlerFunc) containerMethod {
	return httpMethodContainerGenerator("POST", handler)
}

func (h httpMethodRoute) Get(handler http.HandlerFunc) containerMethod {
	return httpMethodContainerGenerator("GET", handler)
}

func (h httpMethodRoute) Put(handler http.HandlerFunc) containerMethod {
	return httpMethodContainerGenerator("PUT", handler)
}

func (h httpMethodRoute) Patch(handler http.HandlerFunc) containerMethod {
	return httpMethodContainerGenerator("PATCH", handler)
}

func (h httpMethodRoute) Delete(handler http.HandlerFunc) containerMethod {
	return httpMethodContainerGenerator("DELETE", handler)
}

func (m *methods) Middleware(middleware ...middler.Middleware) *methods {
	m.middleware = middleware
	return m
}
