package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

// CreateUserRequest is the DTO for incoming user creation payload.
// Demonstrates: External snake_case JSON -> DTO struct tags -> Idiomatic Go fields.
type CreateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// UserResponse is the DTO for outgoing user JSON serialization.
// Keeps external API JSON contract separated from domain core.User entity.
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

// ListUsersHandler returns all users as JSON array.
func ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := logic.ListUsers()
	resp := make([]UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateUserHandler handles user creation demonstrating:
// 1. Decode JSON from r.Body via DTO
// 2. Translate DTO to pure core.User domain entity
// 3. Delegate to Logic layer (logic.CreateUser)
// 4. Map Logic/Core errors to standard HTTP status codes
// 5. Interface composition: http.ResponseWriter satisfies io.Writer passed to json.NewEncoder
func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input CreateUserRequest

	// Decode request body stream
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "malformed JSON: " + err.Error()})
		return
	}

	// Translate DTO -> Domain Entity
	userEntity := core.User{
		FirstName: input.FirstName,
		LastName:  input.LastName,
	}

	// Call Logic layer
	created, err := logic.CreateUser(userEntity)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrInvalidFirstName),
			errors.Is(err, core.ErrInvalidLastName):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		case errors.Is(err, core.ErrUserAlreadyExists):
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "unexpected server error"})
		}
		return
	}

	// Note on interface composition:
	// http.ResponseWriter satisfies io.Writer required by json.NewEncoder(w).
	writeJSON(w, http.StatusCreated, toUserResponse(created))
}
