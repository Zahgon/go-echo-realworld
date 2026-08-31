package v1_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/DoWithLogic/go-echo-realworld/config"
	v1 "github.com/DoWithLogic/go-echo-realworld/internal/users/delivery/http/v1"
	"github.com/DoWithLogic/go-echo-realworld/internal/users/dtos"
	mocks "github.com/DoWithLogic/go-echo-realworld/internal/users/mock"
	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
	"github.com/DoWithLogic/go-echo-realworld/pkg/middleware"
	"github.com/DoWithLogic/go-echo-realworld/pkg/otel/zerolog"
)

const authKey = "DoWithLogic!@#"

func testConfig() config.Config {
	cfg := config.Config{}
	cfg.Authentication.Key = authKey

	return cfg
}

func newTestServer(t *testing.T) (http.Handler, *mocks.MockUsecase) {
	t.Helper()

	uc := mocks.NewMockUsecase(gomock.NewController(t))
	handlers := v1.NewHandlers(uc, zerolog.NewZeroLog(context.Background(), io.Discard))

	router := httpx.NewRouter(false)
	router.Route("/api/v1", func(version chi.Router) {
		v1.MapUserRoute(version, handlers, testConfig())
	})

	return router, uc
}

func bearer(t *testing.T, userID int64, email string) string {
	t.Helper()

	token, err := middleware.GenerateJWT(middleware.CustomClaims{UserID: userID, Email: email}, authKey)
	require.NoError(t, err)

	return "Bearer " + token
}

type call struct {
	method  string
	target  string
	body    string
	headers map[string]string
}

func do(t *testing.T, handler http.Handler, c call) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if c.body != "" {
		body = strings.NewReader(c.body)
	}

	req := httptest.NewRequest(c.method, c.target, body)
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestRoutingFallbacks(t *testing.T) {
	handler, _ := newTestServer(t)

	tests := []struct {
		name        string
		call        call
		status      int
		body        string
		allow       string
		contentType string
	}{
		{
			name:        "unknown path under the api prefix",
			call:        call{method: http.MethodGet, target: "/api/v1/unknown"},
			status:      http.StatusNotFound,
			body:        "{\"message\":\"Not Found\"}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "root path",
			call:        call{method: http.MethodGet, target: "/"},
			status:      http.StatusNotFound,
			body:        "{\"message\":\"Not Found\"}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "trailing slash is not redirected",
			call:        call{method: http.MethodPost, target: "/api/v1/users/"},
			status:      http.StatusNotFound,
			body:        "{\"message\":\"Not Found\"}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "profiles without a username",
			call:        call{method: http.MethodGet, target: "/api/v1/profiles"},
			status:      http.StatusNotFound,
			body:        "{\"message\":\"Not Found\"}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "extra segment after follow",
			call:        call{method: http.MethodPost, target: "/api/v1/profiles/jane/follow/extra"},
			status:      http.StatusNotFound,
			body:        "{\"message\":\"Not Found\"}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "pretty query indents the not found body",
			call:        call{method: http.MethodGet, target: "/api/v1/unknown?pretty=1"},
			status:      http.StatusNotFound,
			body:        "{\n  \"message\": \"Not Found\"\n}\n",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "wrong method on users",
			call:        call{method: http.MethodGet, target: "/api/v1/users"},
			status:      http.StatusMethodNotAllowed,
			body:        "{\"message\":\"Method Not Allowed\"}\n",
			allow:       "OPTIONS, POST",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "wrong method on user",
			call:        call{method: http.MethodPost, target: "/api/v1/user"},
			status:      http.StatusMethodNotAllowed,
			body:        "{\"message\":\"Method Not Allowed\"}\n",
			allow:       "OPTIONS, GET, PUT",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:        "wrong method on follow",
			call:        call{method: http.MethodGet, target: "/api/v1/profiles/jane/follow"},
			status:      http.StatusMethodNotAllowed,
			body:        "{\"message\":\"Method Not Allowed\"}\n",
			allow:       "OPTIONS, POST",
			contentType: "application/json; charset=UTF-8",
		},
		{
			name:   "head on a disallowed method has no body",
			call:   call{method: http.MethodHead, target: "/api/v1/users"},
			status: http.StatusMethodNotAllowed,
			body:   "",
			allow:  "OPTIONS, POST",
		},
		{
			name:   "options reports the allowed methods",
			call:   call{method: http.MethodOptions, target: "/api/v1/users"},
			status: http.StatusNoContent,
			body:   "",
			allow:  "OPTIONS, POST",
		},
		{
			name:   "options on user reports both verbs",
			call:   call{method: http.MethodOptions, target: "/api/v1/user"},
			status: http.StatusNoContent,
			body:   "",
			allow:  "OPTIONS, GET, PUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, handler, tt.call)

			require.Equal(t, tt.status, rec.Code)
			require.Equal(t, tt.body, rec.Body.String())
			require.Equal(t, tt.allow, rec.Header().Get("Allow"))
			require.Equal(t, tt.contentType, rec.Header().Get("Content-Type"))
		})
	}
}

func TestAuthorizationFailures(t *testing.T) {
	handler, _ := newTestServer(t)

	tests := []struct {
		name   string
		header string
		body   string
	}{
		{
			name:   "missing header",
			header: "",
			body:   "{\"status\":401,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"authorization can't be nil\",\"message\":\"Unauthorized\"}]}\n",
		},
		{
			name:   "single word header",
			header: "Bearer",
			body:   "{\"status\":401,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"invalid authorization value\",\"message\":\"Unauthorized\"}]}\n",
		},
		{
			name:   "wrong scheme",
			header: "Basic abcdef",
			body:   "{\"status\":401,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"auth should be bearer\",\"message\":\"Unauthorized\"}]}\n",
		},
		{
			name:   "malformed token",
			header: "Bearer notatoken",
			body:   "{\"status\":401,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"token contains an invalid number of segments\",\"message\":\"Unauthorized\"}]}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{}
			if tt.header != "" {
				headers["Authorization"] = tt.header
			}

			rec := do(t, handler, call{method: http.MethodGet, target: "/api/v1/user", headers: headers})

			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.Equal(t, tt.body, rec.Body.String())
			require.Equal(t, "application/json; charset=UTF-8", rec.Header().Get("Content-Type"))
		})
	}
}

func TestCreateUserBindFailuresKeepTheirErrorText(t *testing.T) {
	handler, _ := newTestServer(t)

	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
		details     []string
	}{
		{
			name:        "wrong json type",
			contentType: "application/json",
			body:        `{"user":{"username":123}}`,
			status:      http.StatusBadRequest,
			// encoding/json names the struct in this message differently across Go
			// releases (the innermost field type up to 1.24, the outermost from 1.25),
			// so that single token is left free while everything Echo contributed is pinned.
			details: []string{
				"code=400, message=Unmarshal type error: expected=string, got=number, field=user.username, offset=23, internal=json: cannot unmarshal number into Go struct field ",
				".user.username of type string",
			},
		},
		{
			name:        "truncated json",
			contentType: "application/json",
			body:        `{"user":{"username":"john"`,
			status:      http.StatusBadRequest,
			details:     []string{"code=400, message=unexpected EOF, internal=unexpected EOF"},
		},
		{
			name:        "invalid literal",
			contentType: "application/json",
			body:        `{n}`,
			status:      http.StatusBadRequest,
			details:     []string{"code=400, message=Syntax error: offset=2, error=invalid character 'n' looking for beginning of object key string, internal=invalid character 'n' looking for beginning of object key string"},
		},
		{
			name:        "unsupported media type",
			contentType: "text/plain",
			body:        "username=john",
			status:      http.StatusBadRequest,
			details:     []string{"code=415, message=Unsupported Media Type"},
		},
		{
			name:        "empty body skips binding and fails validation",
			contentType: "application/json",
			body:        "",
			status:      http.StatusBadRequest,
			details:     []string{"email: cannot be blank; password: cannot be blank; username: cannot be blank."},
		},
		{
			name:        "form body binds then fails validation",
			contentType: "application/x-www-form-urlencoded",
			body:        "username=john",
			status:      http.StatusBadRequest,
			details:     []string{"email: cannot be blank; password: cannot be blank; username: cannot be blank."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, handler, call{
				method:  http.MethodPost,
				target:  "/api/v1/users",
				body:    tt.body,
				headers: map[string]string{"Content-Type": tt.contentType},
			})

			require.Equal(t, tt.status, rec.Code)
			for _, detail := range tt.details {
				require.Contains(t, rec.Body.String(), detail)
			}
		})
	}
}

func TestCreateUserSuccessAlwaysReportsOK(t *testing.T) {
	handler, uc := newTestServer(t)

	request := dtos.UserData{Data: dtos.User{UserName: "john", Email: "john@realworld.io", Password: "secret"}}
	created := dtos.UserData{Data: dtos.User{UserName: "john", Email: "john@realworld.io"}}

	uc.EXPECT().Create(gomock.Any(), request).Return(created, http.StatusCreated, nil)

	rec := do(t, handler, call{
		method:  http.MethodPost,
		target:  "/api/v1/users",
		body:    `{"user":{"username":"john","email":"john@realworld.io","password":"secret"}}`,
		headers: map[string]string{"Content-Type": "application/json"},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"user\":{\"username\":\"john\",\"email\":\"john@realworld.io\",\"token\":\"\",\"image\":\"\",\"bio\":\"\"}}\n", rec.Body.String())
}

func TestCreateUserFailurePropagatesUsecaseStatus(t *testing.T) {
	handler, uc := newTestServer(t)

	uc.EXPECT().Create(gomock.Any(), gomock.Any()).Return(dtos.UserData{}, http.StatusConflict, errors.New("email already exist"))

	rec := do(t, handler, call{
		method:  http.MethodPost,
		target:  "/api/v1/users",
		body:    `{"user":{"username":"john","email":"john@realworld.io","password":"secret"}}`,
		headers: map[string]string{"Content-Type": "application/json"},
	})

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, "{\"status\":409,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"email already exist\",\"message\":\"Conflict\"}]}\n", rec.Body.String())
}

func TestUserDetailReceivesTheIdentityFromTheToken(t *testing.T) {
	handler, uc := newTestServer(t)

	detail := dtos.UserData{Data: dtos.User{UserName: "john", Email: "john@realworld.io"}}
	uc.EXPECT().Detail(gomock.Any(), int64(7)).Return(detail, http.StatusOK, nil)

	rec := do(t, handler, call{
		method:  http.MethodGet,
		target:  "/api/v1/user",
		headers: map[string]string{"Authorization": bearer(t, 7, "john@realworld.io")},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"user\":{\"username\":\"john\",\"email\":\"john@realworld.io\",\"token\":\"\",\"image\":\"\",\"bio\":\"\"}}\n", rec.Body.String())
}

func TestUpdateUserPassesTheClaimsThrough(t *testing.T) {
	handler, uc := newTestServer(t)

	request := dtos.UserData{Data: dtos.User{UserName: "johnny"}}
	claims := middleware.CustomClaims{UserID: 7, Email: "john@realworld.io"}
	updated := dtos.UserData{Data: dtos.User{UserName: "johnny", Email: "john@realworld.io"}}

	uc.EXPECT().
		Update(gomock.Any(), request, gomock.AssignableToTypeOf(claims)).
		DoAndReturn(func(_ context.Context, _ dtos.UserData, got middleware.CustomClaims) (dtos.UserData, int, error) {
			require.Equal(t, claims.UserID, got.UserID)
			require.Equal(t, claims.Email, got.Email)

			return updated, http.StatusOK, nil
		})

	rec := do(t, handler, call{
		method: http.MethodPut,
		target: "/api/v1/user",
		body:   `{"user":{"username":"johnny"}}`,
		headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": bearer(t, 7, "john@realworld.io"),
		},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"user\":{\"username\":\"johnny\",\"email\":\"john@realworld.io\",\"token\":\"\",\"image\":\"\",\"bio\":\"\"}}\n", rec.Body.String())
}

func TestProfileDetailBindsTheUsernamePathParameter(t *testing.T) {
	handler, uc := newTestServer(t)

	profile := dtos.ProfileData{Profile: dtos.Profile{UserName: "jane"}}
	uc.EXPECT().
		ProfileDetail(gomock.Any(), dtos.ProfileRequest{UserName: "jane"}).
		Return(profile, http.StatusOK, nil)

	rec := do(t, handler, call{method: http.MethodGet, target: "/api/v1/profiles/jane"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"profile\":{\"username\":\"jane\",\"bio\":\"\",\"image\":\"\",\"following\":false}}\n", rec.Body.String())
}

func TestProfileDetailCarriesTheOptionalIdentity(t *testing.T) {
	handler, uc := newTestServer(t)

	profile := dtos.ProfileData{Profile: dtos.Profile{UserName: "jane", Following: true}}
	uc.EXPECT().
		ProfileDetail(gomock.Any(), dtos.ProfileRequest{UserID: 7, UserName: "jane"}).
		Return(profile, http.StatusOK, nil)

	rec := do(t, handler, call{
		method:  http.MethodGet,
		target:  "/api/v1/profiles/jane",
		headers: map[string]string{"Authorization": bearer(t, 7, "john@realworld.io")},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"profile\":{\"username\":\"jane\",\"bio\":\"\",\"image\":\"\",\"following\":true}}\n", rec.Body.String())
}

func TestProfileFollowUserCombinesIdentityAndPathParameter(t *testing.T) {
	handler, uc := newTestServer(t)

	profile := dtos.ProfileData{Profile: dtos.Profile{UserName: "jane", Following: true}}
	uc.EXPECT().
		FollowUser(gomock.Any(), dtos.ProfileRequest{UserID: 7, Email: "john@realworld.io", UserName: "jane"}).
		Return(profile, http.StatusOK, nil)

	rec := do(t, handler, call{
		method:  http.MethodPost,
		target:  "/api/v1/profiles/jane/follow",
		headers: map[string]string{"Authorization": bearer(t, 7, "john@realworld.io")},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "{\"profile\":{\"username\":\"jane\",\"bio\":\"\",\"image\":\"\",\"following\":true}}\n", rec.Body.String())
}

func TestLoginReportsTheUsecaseStatus(t *testing.T) {
	handler, uc := newTestServer(t)

	uc.EXPECT().
		Login(gomock.Any(), gomock.Any()).
		Return(dtos.UserData{}, http.StatusUnauthorized, errors.New("invalid password"))

	rec := do(t, handler, call{
		method:  http.MethodPost,
		target:  "/api/v1/users/login",
		body:    `{"user":{"email":"john@realworld.io","password":"wrong"}}`,
		headers: map[string]string{"Content-Type": "application/json"},
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "{\"status\":401,\"message\":{\"en\":\"Failed\",\"id\":\"Gagal\"},\"errors\":[{\"details\":\"invalid password\",\"message\":\"Unauthorized\"}]}\n", rec.Body.String())
}
