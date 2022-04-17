// Package middler manages and caches middleware
package middler

import (
	"context"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
)

type contextKey string

const contextBranch = contextKey("currentBranch")

type middlewoman struct {
	name                   string
	next                   string
	handlerFunc            func(http.Handler) http.Handler
	selfInducingHandler    http.Handler
	selfInducedNextHandler http.Handler
}

type middleman struct {
	sub           []middlewoman
	branch        int
	currentBranch *int
	branchNum     *int
	funcStore     *map[string]*middlewoman
}

func New(handlers ...func(http.Handler) http.Handler) middleman {
	m := newMiddleman(len(handlers), nil, nil, nil)
	for i := 0; i < len(handlers); i++ {
		m.sub[i] = newMiddlewoman(handlers[i])
		m.selfInduce(i)
		if i > 0 {
			m.serve(i-1, &m.sub[i])
		}
	}
	return m
}

func newMiddlewoman(handler func(http.Handler) http.Handler) middlewoman {
	return middlewoman{
		name:        runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name(),
		handlerFunc: handler,
	}
}

func newMiddleman(
	subLength int,
	currentBranch *int,
	branchNum *int,
	funcStore *map[string]*middlewoman) middleman {
	if currentBranch == nil {
		currentBranch = new(int)
	}
	if branchNum == nil {
		branchNum = new(int)
	}
	if funcStore == nil {
		funcmap := make(map[string]*middlewoman)
		funcStore = &funcmap
	}
	return middleman{
		sub:           make([]middlewoman, subLength),
		currentBranch: currentBranch,
		branchNum:     branchNum,
		funcStore:     funcStore,
	}
}

func (m *middleman) store(mw *middlewoman) {
	(*m.funcStore)[mw.name+"("+strconv.Itoa((*m.currentBranch))+")"] = mw
}

func (m *middleman) storeAccess(key string, branch int) (val *middlewoman, ok bool) {
	m.setStoreContext(branch)
	val, ok = (*m.funcStore)[key+"("+strconv.Itoa((*m.currentBranch))+")"]
	return
}

// Allows for variability in closure, decoupling from captured middleman in storeAccess
func (m *middleman) setStoreContext(branch int) {
	*m.currentBranch = branch
}

func (m *middleman) selfInduce(i int) {
	m.setStoreContext(m.branch)
	currentBranch := *m.currentBranch
	m.sub[i].selfInducingHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i == 0 {
			ctx := context.WithValue(r.Context(), contextBranch, currentBranch)
			r = r.WithContext(ctx)

		}
		branch := r.Context().Value(contextBranch).(int)
		if val, ok := m.storeAccess(m.sub[i].name, branch); ok {
			if val.selfInducedNextHandler != nil {
				val.selfInducedNextHandler.ServeHTTP(w, r)
			}
		}
	})
}

func (m *middleman) serve(i int, next *middlewoman) http.Handler {
	m.setStoreContext(m.branch)
	if _, ok := m.storeAccess(next.name, m.branch); !ok && m.branch != 0 {
		m.store(next)
	}
	m.sub[i].next = next.name
	m.sub[i].selfInducedNextHandler = m.sub[i].handlerFunc(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			branch := r.Context().Value(contextBranch).(int)
			if val, ok := m.storeAccess(next.name, branch); ok {
				if val.selfInducingHandler != nil {
					val.selfInducingHandler.ServeHTTP(w, r)
				}
			}
		}))
	return m.sub[i].selfInducedNextHandler
}

func (m *middleman) mainHandler() http.Handler {
	return m.sub[0].selfInducingHandler
}

func (m middleman) fork() middleman {
	nm := newMiddleman(len(m.sub), m.currentBranch, m.branchNum, m.funcStore)
	nm.branch = (*m.branchNum) + 1
	(*m.branchNum)++
	copy(nm.sub, m.sub)
	nm.setStoreContext(nm.branch)
	for i := 0; i < len(nm.sub)-1; i++ {
		nm.sub[i].next = nm.sub[i+1].name
		nm.store(&nm.sub[i])
	}
	nm.store(&nm.sub[len(nm.sub)-1])
	nm.selfInduce(0)
	return nm
}

func (m middleman) Add(handler func(http.Handler) http.Handler) middleman {
	nm := m.fork()
	mw := newMiddlewoman(handler)
	nm.serve(len(nm.sub)-1, &mw)
	nm.sub = append(nm.sub, mw)
	nm.selfInduce(len(nm.sub) - 1)
	return nm
}

func (m middleman) Then(next http.Handler) http.Handler {
	nm := m.fork()
	nm.sub[len(nm.sub)-1].selfInducedNextHandler = next
	return nm.mainHandler()
}

func (m middleman) ThenFunc(next func(http.ResponseWriter, *http.Request)) http.Handler {
	nm := m.fork()
	var handler http.Handler = http.HandlerFunc(next)
	nm.sub[len(nm.sub)-1].selfInducedNextHandler = handler
	return nm.mainHandler()
}

