package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// MIMEApplicationJSONCharsetUTF8 is the Content-Type written by JSON.
const MIMEApplicationJSONCharsetUTF8 = "application/json; charset=UTF-8"

// defaultIndent is used when the response is pretty printed.
const defaultIndent = "  "

type debugCtxKey struct{}

// WithDebug returns middleware that marks every request as belonging to a
// debug-mode server. Responses of debug servers are pretty printed and error
// payloads carry the underlying error string.
func WithDebug(debug bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !debug {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), debugCtxKey{}, true)))
		})
	}
}

// IsDebug reports whether r is served by a server running in debug mode.
func IsDebug(r *http.Request) bool {
	debug, _ := r.Context().Value(debugCtxKey{}).(bool)
	return debug
}

// JSON writes i as a JSON body with the given status code.
//
// The payload is indented when the server runs in debug mode or when the
// request carries a "pretty" query parameter. Content-Type is only set when
// the handler has not already chosen one.
func JSON(w http.ResponseWriter, r *http.Request, code int, i interface{}) {
	indent := ""
	if _, pretty := r.URL.Query()["pretty"]; IsDebug(r) || pretty {
		indent = defaultIndent
	}
	writeJSON(w, code, i, indent)
}

func writeJSON(w http.ResponseWriter, code int, i interface{}, indent string) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", MIMEApplicationJSONCharsetUTF8)
	}
	w.WriteHeader(code)

	enc := json.NewEncoder(w)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	// The response is already committed at this point, so an encoding failure
	// can no longer be turned into an error status.
	_ = enc.Encode(i)
}

// NoContent writes the status code without a body.
func NoContent(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}

// decodeJSON reads the request body into i, translating decoder failures into
// the HTTPError messages clients already depend on.
func decodeJSON(r *http.Request, i interface{}) error {
	err := json.NewDecoder(r.Body).Decode(i)
	if err == nil {
		return nil
	}
	if ute, ok := err.(*json.UnmarshalTypeError); ok {
		return NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v",
			ute.Type, ute.Value, ute.Field, ute.Offset,
		)).SetInternal(err)
	}
	if se, ok := err.(*json.SyntaxError); ok {
		return NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Syntax error: offset=%v, error=%v", se.Offset, se.Error(),
		)).SetInternal(err)
	}
	return err
}
