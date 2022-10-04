// Package httpapp contains data structures for developing apps test
package httpapp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/costal/go-misc-tools/middler"
)

type Auth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	GrantType    string
	Code         string
	Realm        string
	Authorize    func(context.Context, ...interface{}) (interface{}, error)
}

type Application struct {
	name      string
	domain    string
	errorLog  *log.Logger
	infoLog   *log.Logger
	endpoints interface{}
	HTTPHandlers
	Auth
}

func DefaultApp(name, domain string) *Application {
	app := &Application{
		name:     name,
		domain:   domain,
		errorLog: log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
		infoLog:  log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		HTTPHandlers: HTTPHandlers{
			mux:     http.NewServeMux(),
			muxTree: make(map[string]*http.ServeMux),
			httpMethods: httpMethods{
				urmMap: make(map[string]*uriRestMethods),
			},
		},
	}
	app.standardMiddleware = []middler.Middleware{
		app.recoverPanic, app.logRequest,
	}
	app.DynamicMiddleware = []middler.Middleware{}
	return app
}

func (app *Application) Name() string {
	return app.name
}

func (app *Application) Domain() string {
	return app.domain
}

func (app *Application) ErrorLog() *log.Logger {
	return app.errorLog
}

func (app *Application) InfoLog() *log.Logger {
	return app.infoLog
}

func (app *Application) AddStandardMiddleware(sm ...middler.Middleware) {
	if sm != nil {
		app.standardMiddleware = append(app.standardMiddleware, sm...)
	}
}

// ERROR functions

func (app *Application) ServerError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *Application) ClientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *Application) NotFound(w http.ResponseWriter) {
	app.ClientError(w, http.StatusNotFound)
}
