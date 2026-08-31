package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"

	"github.com/DoWithLogic/go-echo-realworld/config"
	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
	"github.com/DoWithLogic/go-echo-realworld/pkg/utils/response"
)

// CustomClaims represents the custom claims you want to include in the JWT payload.
type CustomClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.StandardClaims
}

type identityCtxKey struct{}

// Identity returns the claims the auth middleware attached to the request, or
// nil when the request was not authenticated.
func Identity(ctx context.Context) *CustomClaims {
	claims, _ := ctx.Value(identityCtxKey{}).(*CustomClaims)
	return claims
}

func withIdentity(r *http.Request, claims *CustomClaims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), identityCtxKey{}, claims))
}

func GenerateJWT(data CustomClaims, secretKey string) (string, error) {
	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, data)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func OptionalAuthJWT(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				auth, err := extractBearerToken(r)
				if err != nil {
					httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))
					return
				}

				token, err := jwt.ParseWithClaims(*auth, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
					return []byte(cfg.Authentication.Key), nil
				})

				if err != nil {
					httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))
					return
				}

				if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
					next.ServeHTTP(w, withIdentity(r, claims))

					return
				}

				httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func AuthorizeJWT(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, err := extractBearerToken(r)
			if err != nil {
				httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))
				return
			}

			token, err := jwt.ParseWithClaims(*auth, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.Authentication.Key), nil
			})

			if err != nil {
				httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))
				return
			}

			if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
				next.ServeHTTP(w, withIdentity(r, claims))

				return
			}

			httpx.JSON(w, r, http.StatusUnauthorized, response.NewResponseError(http.StatusUnauthorized, response.MsgFailed, err.Error()))
		})
	}
}

func extractBearerToken(r *http.Request) (*string, error) {
	authData := r.Header.Get("Authorization")
	if authData == "" {
		return nil, errors.New("authorization can't be nil")
	}
	parts := strings.Split(authData, " ")
	if len(parts) < 2 {
		return nil, errors.New("invalid authorization value")
	}
	if parts[0] != "Bearer" {
		return nil, errors.New("auth should be bearer")
	}

	return &parts[1], nil
}
