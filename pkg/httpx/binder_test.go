package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
)

type payload struct {
	UserName string `param:"username" json:"username"`
	Page     int    `query:"page"`
	Active   bool   `query:"active"`
	Ratio    float64
	Token    string `header:"X-Token"`
}

// bindThrough registers pattern on a chi router so that the request carries a
// real chi route context, then binds the request into a fresh payload.
func bindThrough(t *testing.T, pattern, method, target, body, contentType string) (payload, error) {
	t.Helper()

	var (
		bound payload
		err   error
	)

	router := chi.NewRouter()
	router.MethodFunc(method, pattern, func(w http.ResponseWriter, r *http.Request) {
		err = httpx.Bind(&bound, r)
	})

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	var req *http.Request
	if reader == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, reader)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	router.ServeHTTP(httptest.NewRecorder(), req)

	return bound, err
}

func TestBindReadsPathParameters(t *testing.T) {
	bound, err := bindThrough(t, "/profiles/{username}", http.MethodGet, "/profiles/jane", "", "")

	require.NoError(t, err)
	require.Equal(t, "jane", bound.UserName)
}

func TestBindReadsQueryParametersOnlyForBodylessMethods(t *testing.T) {
	tests := []struct {
		method   string
		wantPage int
	}{
		{method: http.MethodGet, wantPage: 3},
		{method: http.MethodDelete, wantPage: 3},
		{method: http.MethodHead, wantPage: 3},
		{method: http.MethodPost, wantPage: 0},
		{method: http.MethodPut, wantPage: 0},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			bound, err := bindThrough(t, "/items", tt.method, "/items?page=3&active=true", "", "")

			require.NoError(t, err)
			require.Equal(t, tt.wantPage, bound.Page)
		})
	}
}

func TestBindSkipsTheBodyWhenThereIsNothingToRead(t *testing.T) {
	bound, err := bindThrough(t, "/items", http.MethodPost, "/items", "", "application/json")

	require.NoError(t, err)
	require.Zero(t, bound)
}

func TestBindRejectsUnknownContentTypes(t *testing.T) {
	_, err := bindThrough(t, "/items", http.MethodPost, "/items", "username=jane", "text/plain")

	require.EqualError(t, err, "code=415, message=Unsupported Media Type")
}

func TestBindDecodesFormBodies(t *testing.T) {
	type form struct {
		UserName string `form:"username"`
	}

	var bound form

	router := chi.NewRouter()
	router.Post("/items", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, httpx.Bind(&bound, r))
	})

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("username=jane"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "jane", bound.UserName)
}

func TestBindReportsJSONErrorsWithEchosWording(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "type mismatch",
			body: `{"username":123}`,
			want: "code=400, message=Unmarshal type error: expected=string, got=number, field=username, offset=15, internal=json: cannot unmarshal number into Go struct field payload.username of type string",
		},
		{
			name: "syntax error",
			body: `{n}`,
			want: "code=400, message=Syntax error: offset=2, error=invalid character 'n' looking for beginning of object key string, internal=invalid character 'n' looking for beginning of object key string",
		},
		{
			name: "truncated document",
			body: `{"username":"jane"`,
			want: "code=400, message=unexpected EOF, internal=unexpected EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bindThrough(t, "/items", http.MethodPost, "/items", tt.body, "application/json")

			require.EqualError(t, err, tt.want)
		})
	}
}

func TestBindHeadersIsOptInOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set("X-Token", "abc123")

	var implicit payload
	require.NoError(t, httpx.Bind(&implicit, req))
	require.Empty(t, implicit.Token)

	var explicit payload
	require.NoError(t, httpx.BindHeaders(req, &explicit))
	require.Equal(t, "abc123", explicit.Token)
}

func TestJSONHonoursTheEchoResponseContract(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		debug       bool
		wantBody    string
		wantCharset string
	}{
		{
			name:        "compact by default",
			target:      "/items",
			wantBody:    "{\"username\":\"jane\",\"Page\":0,\"Active\":false,\"Ratio\":0,\"Token\":\"\"}\n",
			wantCharset: "application/json; charset=UTF-8",
		},
		{
			name:        "indented when pretty is requested",
			target:      "/items?pretty",
			wantBody:    "{\n  \"username\": \"jane\",\n  \"Page\": 0,\n  \"Active\": false,\n  \"Ratio\": 0,\n  \"Token\": \"\"\n}\n",
			wantCharset: "application/json; charset=UTF-8",
		},
		{
			name:        "indented in debug mode",
			target:      "/items",
			debug:       true,
			wantBody:    "{\n  \"username\": \"jane\",\n  \"Page\": 0,\n  \"Active\": false,\n  \"Ratio\": 0,\n  \"Token\": \"\"\n}\n",
			wantCharset: "application/json; charset=UTF-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Use(httpx.WithDebug(tt.debug))
			router.Get("/items", func(w http.ResponseWriter, r *http.Request) {
				httpx.JSON(w, r, http.StatusTeapot, payload{UserName: "jane"})
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			require.Equal(t, http.StatusTeapot, rec.Code)
			require.Equal(t, tt.wantBody, rec.Body.String())
			require.Equal(t, tt.wantCharset, rec.Header().Get("Content-Type"))
		})
	}
}

func TestErrorHandlerMirrorsEchosDefaults(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		err      error
		debug    bool
		status   int
		wantBody string
	}{
		{
			name:     "http error with a string message",
			method:   http.MethodGet,
			err:      httpx.ErrNotFound,
			status:   http.StatusNotFound,
			wantBody: "{\"message\":\"Not Found\"}\n",
		},
		{
			name:     "plain error becomes an internal server error",
			method:   http.MethodGet,
			err:      errBoom{},
			status:   http.StatusInternalServerError,
			wantBody: "{\"message\":\"Internal Server Error\"}\n",
		},
		{
			name:     "debug mode exposes the underlying error",
			method:   http.MethodGet,
			err:      httpx.NewHTTPError(http.StatusBadRequest, "bad input"),
			debug:    true,
			status:   http.StatusBadRequest,
			wantBody: "{\n  \"error\": \"code=400, message=bad input\",\n  \"message\": \"bad input\"\n}\n",
		},
		{
			name:     "head requests never carry a body",
			method:   http.MethodHead,
			err:      httpx.ErrNotFound,
			status:   http.StatusNotFound,
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Use(httpx.WithDebug(tt.debug))
			router.MethodFunc(tt.method, "/items", func(w http.ResponseWriter, r *http.Request) {
				httpx.ErrorHandler(w, r, tt.err)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, "/items", nil))

			require.Equal(t, tt.status, rec.Code)
			require.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestAllowHeaderListsEveryRegisteredMethodInOrder(t *testing.T) {
	noop := func(w http.ResponseWriter, r *http.Request) {}

	t.Run("only the methods that are registered", func(t *testing.T) {
		router := httpx.NewRouter(false)
		router.Route("/api", func(r chi.Router) {
			r.Get("/thing", noop)
			r.Put("/thing", noop)
			r.Delete("/thing", noop)
		})

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/thing", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Equal(t, "OPTIONS, DELETE, GET, PUT", rec.Header().Get("Allow"))
	})

	// Echo's ordering is only observable on a path that answers every verb.
	t.Run("the complete ordering", func(t *testing.T) {
		router := httpx.NewRouter(false)
		router.Route("/api", func(r chi.Router) {
			r.Post("/thing", noop)
			r.Trace("/thing", noop)
			r.Get("/thing", noop)
			r.Connect("/thing", noop)
			r.Put("/thing", noop)
			r.Head("/thing", noop)
			r.Delete("/thing", noop)
			r.Patch("/thing", noop)
		})

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/thing", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		require.Equal(t,
			"OPTIONS, CONNECT, DELETE, GET, HEAD, PATCH, POST, PUT, TRACE",
			rec.Header().Get("Allow"))
	})
}
