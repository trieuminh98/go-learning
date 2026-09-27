package api

import (
	"net/http"
	"strconv"

	"go-learning/internal/logic"
)

// FeedHandler handles HTTP feed requests and maps them to logic.FeedService.
type FeedHandler struct {
	service *logic.FeedService
}

// NewFeedHandler creates a new FeedHandler.
func NewFeedHandler(service *logic.FeedService) *FeedHandler {
	return &FeedHandler{service: service}
}

// GetFeed handles GET /v1/feed.
func (h *FeedHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	actorID, err := resolveActorID(r)
	if err != nil {
		badRequest(w, r, err)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	posts, err := h.service.GetFeed(r.Context(), actorID, limit)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	resp := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, toPostResponse(p))
	}

	_ = jsonResponse(w, http.StatusOK, resp)
}
