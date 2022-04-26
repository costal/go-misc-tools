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
	urmMethods []*uriRestMethods
	urmMap     map[string]*uriRestMethods
}

type containerMethod func(...middler.Middleware) interface{}

func flattenContainers(cms []containerMethod) (ms []*methods) {
	ms = make([]*methods, len(cms))
	var wg sync.WaitGroup
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

func (h *httpMethods) URL(url string, method ...containerMethod) *uriRestMethods {
	urm, ok := h.urmMap[url]
	if !ok {
		urm = &uriRestMethods{uri: url}
		urm.methods = []*methods{&urm.Post, &urm.Get, &urm.Put, &urm.Patch, &urm.Delete}
		h.urmMap[url] = urm
		h.urmMethods = append(h.urmMethods, urm)
	}
	currentURM = urm
	return urm.Method(flattenContainers(method)...)
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
			} else {
				val.middleware = nil
			}
		}
	}
	return urm
}

func (c containerMethod) Middleware(middleware ...middler.Middleware) containerMethod {
	return func(m ...middler.Middleware) containerMethod {
		return func(m ...middler.Middleware) interface{} {
			m = append(middleware, m...)
			return c(m...)
		}
	}(middleware...)
}

func (urm *uriRestMethods) Middleware(middleware ...middler.Middleware) *uriRestMethods {
	var funcs []func(wg *sync.WaitGroup)
	// Point for optimization
	for _, val := range urm.methods {
		// Add to specific methods or add to all of them
		if urm.methodsSet {
			if val.methodName != "" {
				funcs = append(funcs, func(wg *sync.WaitGroup) {
					defer wg.Done()
					val.middleware = append(val.middleware, middleware...)
				})
			} else {
				val.middleware = nil
			}
		} else {
			funcs = append(funcs, func(wg *sync.WaitGroup) {
				defer wg.Done()
				val.middleware = append(val.middleware, middleware...)
			})
		}
	}

	var wg sync.WaitGroup
	for _, val := range funcs {
		wg.Add(1)
		go val(&wg)
	}
	wg.Wait()
	return urm
}

type httpMethodRoute struct{}

func GetHTTPMethods() httpMethodRoute {
	return httpMethodRoute{}
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

func httpMethodContainerGenerator(method string, handler http.HandlerFunc) containerMethod {
	return func(m ...middler.Middleware) interface{} {
		return func(...middler.Middleware) *methods {
			return createHTTPMethod(method, handler)
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
