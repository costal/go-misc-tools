// Package httpapp contains data structures for developing apps test
package httpapp

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
)

type Application struct {
	name      string
	errorLog  *log.Logger
	infoLog   *log.Logger
	endpoints interface{}
	HTTPHandlers
}

func DefaultApp(name string, endpoints interface{}) *Application {
	app := &Application{
		name:      name,
		errorLog:  log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
		infoLog:   log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		endpoints: endpoints,
		HTTPHandlers: HTTPHandlers{
			mux: http.NewServeMux(),
		},
	}
	app.standardMiddleware = []func(http.Handler) http.Handler{
		app.recoverPanic, app.logRequest,
	}
	app.DynamicMiddleware = []func(http.Handler) http.Handler{}
	return app
}

func (app *Application) Name() string {
	return app.name
}

func (app *Application) ErrorLog() *log.Logger {
	return app.errorLog
}

func (app *Application) InfoLog() *log.Logger {
	return app.infoLog
}

func (app *Application) AddStandardMiddleware(sm ...func(http.Handler) http.Handler) {
	if sm != nil {
		app.standardMiddleware = append(app.standardMiddleware, sm...)
	}
}

func (app *Application) serverError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.errorLog.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *Application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}
