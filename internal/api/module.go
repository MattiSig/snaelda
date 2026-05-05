package api

import "net/http"

type Module interface {
	Name() string
}

func mountPlaceholderModule(mux *http.ServeMux, module Module) {
	name := module.Name()
	mux.HandleFunc("GET /api/"+name, notImplemented(name))
}

func notImplemented(moduleName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotImplemented, "not_implemented", moduleName+" module is scaffolded but not implemented")
	}
}
