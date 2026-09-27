package api

import (
	"net/http"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

// UserHandler handles HTTP user requests and maps them to logic.UserService.
type UserHandler struct {
	service *logic.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(service *logic.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// CreateUserRequest is the DTO for incoming user creation payload.
type CreateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// UserResponse is the DTO for outgoing user JSON serialization.
type UserResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func toUserResponse(u core.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

// ListUsers returns all users as a JSON array.
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
