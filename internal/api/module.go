package api

import (
	"net/http"

	"github.com/MattiSig/snaelda/internal/auth"
)

type Module interface {
	Name() string
}

func mountPlaceholderModule(mux *http.ServeMux, module Module) {
	name := module.Name()
	mux.HandleFunc("GET /api/"+name, notImplemented(name))
}

func mountAuthenticatedPlaceholderModule(mux *http.ServeMux, authHandler *auth.Handler, module Module) {
	name := module.Name()
	mux.Handle("GET /api/"+name, authHandler.RequireUser(notImplemented(name)))
}

func notImplemented(moduleName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented, "not_implemented", moduleName+" module is scaffolded but not implemented")
	}
}
