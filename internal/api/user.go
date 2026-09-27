package api

import (
	"encoding/json"
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

// resolveActorID resolves the actor user ID performing a relationship action.
// It checks for a temporary client-provided JSON body payload (pre-auth workaround)
// and falls back to the authenticated user ID in request context.
func resolveActorID(r *http.Request) (int64, error) {
	if r.Body != nil && r.ContentLength > 0 {
		var input struct {
			UserID int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err == nil && input.UserID > 0 {
			return input.UserID, nil
		}
	}
	return UserIDFromContext(r.Context())
}

// Follow handles PUT /v1/users/{userID}/follow.
func (h *UserHandler) Follow(w http.ResponseWriter, r *http.Request) {
	targetID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	actorID, err := resolveActorID(r)
	if err != nil {
		badRequest(w, r, err)
		return
	}

	if err := h.service.FollowUser(r.Context(), actorID, targetID); err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Unfollow handles PUT /v1/users/{userID}/unfollow.
func (h *UserHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	targetID, err := readIDParam(r, "userID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	actorID, err := resolveActorID(r)
	if err != nil {
		badRequest(w, r, err)
		return
	}

	if err := h.service.UnfollowUser(r.Context(), actorID, targetID); err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
