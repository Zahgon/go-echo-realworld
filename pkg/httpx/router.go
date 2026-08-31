package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter builds a router with the fallbacks every route relies on. They have
// to be registered before any route so that mounted sub-routers inherit them.
func NewRouter(debug bool) *chi.Mux {
	router := chi.NewRouter()
	router.Use(WithDebug(debug))
	router.NotFound(NotFoundHandler)
	router.MethodNotAllowed(MethodNotAllowedHandler)

	return router
}

// allowMethodOrder is the order methods appear in a generated Allow header.
// OPTIONS is always reported first and is therefore not part of the list.
var allowMethodOrder = []string{
	http.MethodConnect,
	http.MethodDelete,
	http.MethodGet,
	http.MethodHead,
	http.MethodPatch,
	http.MethodPost,
	http.MethodPut,
	http.MethodTrace,
}

// AllowHeader builds the Allow header for the routes registered at the
// request's path. Every routable path also answers OPTIONS.
func AllowHeader(r *http.Request) string {
	allow := http.MethodOptions

	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return allow
	}
	for _, method := range allowMethodOrder {
		if rctx.Routes.Match(chi.NewRouteContext(), method, r.URL.Path) {
			allow += ", " + method
		}
	}
	return allow
}

// OptionsHandler answers a preflight-style OPTIONS request with the methods
// available at that path.
func OptionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", AllowHeader(r))
	NoContent(w, http.StatusNoContent)
}

// NotFoundHandler renders the 404 JSON envelope.
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	ErrorHandler(w, r, ErrNotFound)
}

// MethodNotAllowedHandler renders the 405 JSON envelope together with the
// Allow header for the requested path.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", AllowHeader(r))
	ErrorHandler(w, r, ErrMethodNotAllowed)
}
