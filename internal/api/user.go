package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

// UserHandler handles HTTP user requests and maps them to logic.UserService.
type UserHandler struct {
	service   *logic.UserService
	validator *validator.Validate
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(service *logic.UserService, validator *validator.Validate) *UserHandler {
	return &UserHandler{
		service:   service,
		validator: validator,
	}
}

// CreateUserRequest is the DTO for incoming user creation payload.
type CreateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// RegisterUserRequest is the DTO for incoming user registration payload.
type RegisterUserRequest struct {
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"required,max=100"`
	Email     string `json:"email" validate:"required,email,max=255"`
	Password  string `json:"password" validate:"required,min=8,max=72"`
}

// CreateTokenRequest is the DTO for incoming user authentication payload.
type CreateTokenRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// TokenResponse is the DTO for outgoing JWT token response.
type TokenResponse struct {
	Token string `json:"token"`
}

// TokenEnvelope defines the standard API success envelope for token endpoints.
type TokenEnvelope struct {
	Data TokenResponse `json:"data"`
}

// UserResponse is the DTO for outgoing user JSON serialization.
type UserResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
	IsActive  bool   `json:"is_active"`
}

func toUserResponse(u core.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		IsActive:  u.IsActive,
	}
}

// ListUsers returns all users as a JSON array.
//
//	@Summary		List users
//	@Description	Retrieve a list of all registered users
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UsersEnvelope
//	@Failure		401	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/users [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := make([]UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	_ = jsonResponse(w, http.StatusOK, resp)
}

// CreateUser handles user creation by delegating to UserService.
//
//	@Summary		Create user
//	@Description	Register a new user
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		CreateUserRequest	true	"User creation payload"
//	@Success		201		{object}	UserEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var input CreateUserRequest

	if err := readJSON(w, r, &input); err != nil {
		badRequest(w, r, err)
		return
	}

	userEntity := core.User{
		FirstName: input.FirstName,
		LastName:  input.LastName,
	}

	created, err := h.service.CreateUser(r.Context(), userEntity)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = jsonResponse(w, http.StatusCreated, toUserResponse(created))
}

// GetUser handles GET /v1/users/{userID}.
//
//	@Summary		Get user
//	@Description	Retrieve profile of a user by their ID
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			userID	path		int	true	"User ID"
//	@Success		200		{object}	UserEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/users/{userID} [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	user, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = jsonResponse(w, http.StatusOK, toUserResponse(user))
}

// Follow handles PUT /v1/users/{userID}/follow.
//
//	@Summary		Follow user
//	@Description	Establish a follow relationship with target user
//	@Tags			users
//	@Security		BearerAuth
//	@Param			userID	path	int	true	"Target User ID to follow"
//	@Success		204		"No Content"
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/users/{userID}/follow [put]
func (h *UserHandler) Follow(w http.ResponseWriter, r *http.Request) {
	targetID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	actorID, err := UserIDFromContext(r.Context())
	if err != nil {
		unauthorized(w, r, "unauthorized")
		return
	}

	if err := h.service.FollowUser(r.Context(), actorID, targetID); err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Unfollow handles PUT /v1/users/{userID}/unfollow.
//
//	@Summary		Unfollow user
//	@Description	Remove an existing follow relationship with target user
//	@Tags			users
//	@Security		BearerAuth
//	@Param			userID	path	int	true	"Target User ID to unfollow"
//	@Success		204		"No Content"
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/users/{userID}/unfollow [put]
func (h *UserHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	targetID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	actorID, err := UserIDFromContext(r.Context())
	if err != nil {
		unauthorized(w, r, "unauthorized")
		return
	}

	if err := h.service.UnfollowUser(r.Context(), actorID, targetID); err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterUser handles POST /v1/authentication/user.
//
//	@Summary		Register user
//	@Description	Register a new user account as inactive, generate an invitation token, and send an activation email
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		RegisterUserRequest	true	"User registration payload"
//	@Success		201		{object}	UserEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/authentication/user [post]
func (h *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var input RegisterUserRequest

	if err := readJSON(w, r, &input); err != nil {
		badRequest(w, r, err)
		return
	}

	if h.validator != nil {
		if err := h.validator.Struct(input); err != nil {
			badRequest(w, r, err)
			return
		}
	}

	created, err := h.service.RegisterUser(r.Context(), logic.RegisterUserInput{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     input.Email,
		Password:  input.Password,
	})
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = jsonResponse(w, http.StatusCreated, toUserResponse(created))
}

// ActivateUser handles PUT /v1/authentication/users/activate/{token}.
//
//	@Summary		Activate user
//	@Description	Activate an inactive user account using a one-time invitation token
//	@Tags			authentication
//	@Param			token	path	string	true	"Plaintext activation token"
//	@Success		204		"No Content"
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/authentication/users/activate/{token} [put]
func (h *UserHandler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		token = r.PathValue("token")
	}
	if token == "" {
		MapDomainErrorToHTTP(w, r, core.ErrInvitationNotFound)
		return
	}

	if err := h.service.ActivateUser(r.Context(), token); err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateToken handles POST /v1/authentication/token.
//
//	@Summary		Create authentication token
//	@Description	Exchange valid user credentials for a signed, short-lived JWT token
//	@Tags			authentication
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		CreateTokenRequest	true	"Authentication credentials"
//	@Success		200		{object}	TokenEnvelope
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/authentication/token [post]
func (h *UserHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	var input CreateTokenRequest

	if err := readJSON(w, r, &input); err != nil {
		badRequest(w, r, err)
		return
	}

	if h.validator != nil {
		if err := h.validator.Struct(input); err != nil {
			badRequest(w, r, err)
			return
		}
	}

	token, err := h.service.AuthenticateUser(r.Context(), input.Email, input.Password)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = jsonResponse(w, http.StatusOK, TokenResponse{Token: token})
}
