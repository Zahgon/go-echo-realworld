package httpx

import (
	"fmt"
	"net/http"
)

// HTTPError represents an error that occurred while handling a request.
//
// Its Error() string is part of the public API surface: handlers put it into
// the "details" field of the JSON error envelope, so the formatting is fixed.
type HTTPError struct {
	Code     int         `json:"-"`
	Message  interface{} `json:"message"`
	Internal error       `json:"-"`
}

// NewHTTPError creates a new HTTPError instance. When no message is supplied
// the canonical status text for code is used.
func NewHTTPError(code int, message ...interface{}) *HTTPError {
	he := &HTTPError{Code: code, Message: http.StatusText(code)}
	if len(message) > 0 {
		he.Message = message[0]
	}
	return he
}

// Error makes it compatible with the `error` interface.
func (he *HTTPError) Error() string {
	if he.Internal == nil {
		return fmt.Sprintf("code=%d, message=%v", he.Code, he.Message)
	}
	return fmt.Sprintf("code=%d, message=%v, internal=%v", he.Code, he.Message, he.Internal)
}

// SetInternal sets the error wrapped by the HTTPError.
func (he *HTTPError) SetInternal(err error) *HTTPError {
	he.Internal = err
	return he
}

// Unwrap satisfies the errors.Unwrap interface.
func (he *HTTPError) Unwrap() error {
	return he.Internal
}

// Errors returned when a request cannot be routed or its body cannot be read.
var (
	ErrUnsupportedMediaType = NewHTTPError(http.StatusUnsupportedMediaType)
	ErrNotFound             = NewHTTPError(http.StatusNotFound)
	ErrMethodNotAllowed     = NewHTTPError(http.StatusMethodNotAllowed)
)

// ErrorHandler writes err to the response as a JSON envelope.
//
// A *HTTPError is rendered with its own status code and message; anything else
// becomes a 500. When the error carries an internal *HTTPError, that inner
// error wins. HEAD requests get the status code and no body.
func ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	he, ok := err.(*HTTPError)
	if ok {
		if he.Internal != nil {
			if herr, ok := he.Internal.(*HTTPError); ok {
				he = herr
			}
		}
	} else {
		he = &HTTPError{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError),
		}
	}

	code := he.Code
	message := he.Message
	switch m := he.Message.(type) {
	case string:
		if IsDebug(r) {
			message = map[string]interface{}{"message": m, "error": err.Error()}
		} else {
			message = map[string]interface{}{"message": m}
		}
	case error:
		message = map[string]interface{}{"message": m.Error()}
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(code)
		return
	}
	JSON(w, r, code, message)
}
