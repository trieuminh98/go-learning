package api

import (
	"net/http"
	"strconv"
	"strings"

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

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if parsed, err := strconv.Atoi(pageStr); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	if sizeStr := r.URL.Query().Get("page_size"); sizeStr != "" {
		if parsed, err := strconv.Atoi(sizeStr); err == nil && parsed > 0 {
			pageSize = parsed
		}
	} else if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	limit := pageSize

	var filter logic.FeedFilter
	if tag := strings.TrimSpace(r.URL.Query().Get("tag")); tag != "" {
		filter.Tag = &tag
	}
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		filter.Search = &search
	}

	posts, err := h.service.GetFeed(r.Context(), actorID, filter, limit, offset)
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
