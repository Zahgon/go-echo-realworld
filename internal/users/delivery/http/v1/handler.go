package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/DoWithLogic/go-echo-realworld/config"
	"github.com/DoWithLogic/go-echo-realworld/internal/users/dtos"
	usecases "github.com/DoWithLogic/go-echo-realworld/internal/users/usecase"
	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
	"github.com/DoWithLogic/go-echo-realworld/pkg/middleware"
	"github.com/DoWithLogic/go-echo-realworld/pkg/otel/zerolog"
	"github.com/DoWithLogic/go-echo-realworld/pkg/utils/response"
)

type (
	Handlers interface {
		Login(w http.ResponseWriter, r *http.Request)
		CreateUser(w http.ResponseWriter, r *http.Request)
		UserDetail(w http.ResponseWriter, r *http.Request)
		UpdateUser(w http.ResponseWriter, r *http.Request)

		ProfileDetail(w http.ResponseWriter, r *http.Request)
		ProfileFollowUser(w http.ResponseWriter, r *http.Request)
	}

	handlers struct {
		uc  usecases.Usecase
		log *zerolog.Logger
		cfg config.Config
	}
)

func NewHandlers(uc usecases.Usecase, log *zerolog.Logger) Handlers {
	return &handlers{uc: uc, log: log}
}

func (h *handlers) Login(w http.ResponseWriter, r *http.Request) {
	var (
		request dtos.UserData
	)

	if err := httpx.Bind(&request, r); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	if err := request.ValidateLogin(); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	authData, httpCode, err := h.uc.Login(r.Context(), request)
	if err != nil {
		httpx.JSON(w, r, httpCode, response.NewResponseError(httpCode, response.MsgFailed, err.Error()))
		return
	}

	httpx.JSON(w, r, httpCode, authData)
}

func (h *handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var (
		ctx, cancel = context.WithTimeout(r.Context(), time.Duration(30*time.Second))
		request     dtos.UserData
	)
	defer cancel()

	if err := httpx.Bind(&request, r); err != nil {
		h.log.Z().Err(err).Msg("users.handlers.CreateUser.Bind")

		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(
			http.StatusBadRequest,
			response.MsgFailed,
			err.Error()),
		)

		return
	}

	if err := request.ValidateCreate(); err != nil {
		h.log.Z().Err(err).Msg("users.handlers.CreateUser.Validate")

		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(
			http.StatusBadRequest,
			response.MsgFailed,
			err.Error()),
		)

		return
	}

	data, httpCode, err := h.uc.Create(ctx, request)
	if err != nil {
		httpx.JSON(w, r, httpCode, response.NewResponseError(
			httpCode,
			response.MsgFailed,
			err.Error()),
		)

		return
	}

	httpx.JSON(w, r, http.StatusOK, data)
}

func (h *handlers) UserDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(30*time.Second))
	defer cancel()

	userID := middleware.Identity(r.Context()).UserID

	data, code, err := h.uc.Detail(ctx, userID)
	if err != nil {
		httpx.JSON(w, r, code, response.NewResponseError(code, response.MsgFailed, err.Error()))
		return
	}

	httpx.JSON(w, r, code, data)
}

func (h *handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(30*time.Second))
	defer cancel()

	var request dtos.UserData
	if err := httpx.Bind(&request, r); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	identity := middleware.Identity(r.Context())

	data, code, err := h.uc.Update(ctx, request, *identity)
	if err != nil {
		httpx.JSON(w, r, code, response.NewResponseError(code, response.MsgFailed, err.Error()))
		return
	}

	httpx.JSON(w, r, code, data)
}

func (h *handlers) ProfileDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(30*time.Second))
	defer cancel()

	var request dtos.ProfileRequest
	if identity := middleware.Identity(r.Context()); identity != nil {
		request.UserID = identity.UserID
	}

	if err := httpx.Bind(&request, r); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	if err := request.Validate(); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	profileData, code, err := h.uc.ProfileDetail(ctx, request)
	if err != nil {
		httpx.JSON(w, r, code, response.NewResponseError(code, response.MsgFailed, err.Error()))
		return
	}

	httpx.JSON(w, r, code, profileData)
}

func (h *handlers) ProfileFollowUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(30*time.Second))
	defer cancel()

	var request = dtos.ProfileRequest{
		UserID: middleware.Identity(r.Context()).UserID,
		Email:  middleware.Identity(r.Context()).Email,
	}

	if err := httpx.Bind(&request, r); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	if err := request.Validate(); err != nil {
		httpx.JSON(w, r, http.StatusBadRequest, response.NewResponseError(http.StatusBadRequest, response.MsgFailed, err.Error()))
		return
	}

	profileData, code, err := h.uc.FollowUser(ctx, request)
	if err != nil {
		httpx.JSON(w, r, code, response.NewResponseError(code, response.MsgFailed, err.Error()))
		return
	}

	httpx.JSON(w, r, code, profileData)
}
