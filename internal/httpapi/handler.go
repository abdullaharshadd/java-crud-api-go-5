package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// maxBodyBytes caps request bodies at 1 MiB.
const maxBodyBytes = 1 << 20

// textPlainUTF8 is the Content-Type Spring used for ResponseEntity<String>.
const textPlainUTF8 = "text/plain;charset=UTF-8"

// Handler serves the User REST endpoints. It replaces the Spring
// UserController; the UserService is injected through NewHandler instead of
// @Autowired field injection.
type Handler struct {
	svc service.UserService
	log zerolog.Logger
}

// NewHandler returns a Handler backed by svc that logs through logger.
func NewHandler(svc service.UserService, logger zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: logger}
}

// Routes returns the HTTP handler with every User endpoint registered,
// wrapped in trailing-slash stripping (Spring Boot 2.7 trailing-slash
// matching). Unknown paths get Spring Boot's default 404 JSON.
//
// MIGRATION_NOTE: cmd/server/router.go must mount this handler (e.g.
// mux.Handle("/", h.Routes())) — it currently only serves /healthz.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/save_user_data", h.saveUser)
	mux.HandleFunc("/get_user_data", h.fetchUserList)
	mux.HandleFunc("/get_user_data/{id}", h.fetchUserByID)
	mux.HandleFunc("/delete_user_data/{id}", h.deleteUser)
	mux.HandleFunc("/update_user_data/{id}", h.updateUser)
	mux.HandleFunc("/get_user_name/name/{name}", h.getUserByName)
	mux.Handle("/", NotFoundHandler())
	return stripTrailingSlash(mux)
}

// stripTrailingSlash removes a single trailing slash so "/x/" matches "/x".
func stripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if len(p) > 1 && strings.HasSuffix(p, "/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimSuffix(p, "/")
			if r2.URL.RawPath != "" {
				r2.URL.RawPath = strings.TrimSuffix(r2.URL.RawPath, "/")
			}
			r = r2
		}
		next.ServeHTTP(w, r)
	})
}

// allowMethod checks the request method against the route's method. GET
// routes also accept HEAD. OPTIONS is answered with 200 and Allow. It
// returns true when the handler should continue.
func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	allowed := []string{method}
	if method == http.MethodGet {
		allowed = append(allowed, http.MethodHead)
	}
	allowed = append(allowed, http.MethodOptions)

	switch {
	case r.Method == method:
		return true
	case method == http.MethodGet && r.Method == http.MethodHead:
		return true
	case r.Method == http.MethodOptions:
		w.Header().Set("Allow", strings.Join(allowed, ","))
		w.WriteHeader(http.StatusOK)
		return false
	default:
		writeError(w, r, &MethodNotAllowedError{Allowed: allowed})
		return false
	}
}

// parseID converts the {id} path variable to a 32-bit int, like Spring's
// String->int conversion. Failures map to 400.
func parseID(r *http.Request) (int32, error) {
	raw := r.PathValue("id")
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid id %q: %v", ErrBadRequest, raw, err)
	}
	return int32(n), nil
}

// decodeUser reads a JSON User body. Non-JSON content types give 415; a
// missing, null or malformed body gives 400 (HttpMessageNotReadable).
// Unknown fields are tolerated, like Jackson in Spring Boot's defaults.
func decodeUser(w http.ResponseWriter, r *http.Request) (model.User, error) {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return model.User{}, ErrUnsupportedMediaType
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var u *model.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		return model.User{}, fmt.Errorf("%w: decode body: %v", ErrBadRequest, err)
	}
	if u == nil {
		return model.User{}, fmt.Errorf("%w: null body", ErrBadRequest)
	}
	return *u, nil
}

// writeText writes a text/plain;charset=UTF-8 body with status 200.
func (h *Handler) writeText(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", textPlainUTF8)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(body)); err != nil {
		h.log.Debug().Err(err).Msg("failed to write response")
	}
}

// saveUser handles POST /save_user_data: validates (@Valid) and persists the
// user, answering with a plain-text confirmation.
func (h *Handler) saveUser(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	u, err := decodeUser(w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := u.Validate(); err != nil {
		writeError(w, r, fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}
	h.log.Info().Msg("inside the saveUser of UserController ")
	if _, err := h.svc.SaveUser(r.Context(), u); err != nil {
		writeError(w, r, err)
		return
	}
	h.writeText(w, "User data saved successfully!")
}

// fetchUserList handles GET /get_user_data: every user as a JSON array.
func (h *Handler) fetchUserList(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	h.log.Info().Msg("inside the fetchUserList of UserController ")
	users, err := h.svc.FetchUserList(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	if users == nil {
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

// fetchUserByID handles GET /get_user_data/{id}. A missing user yields the
// 404 ErrorMessage body via writeError.
func (h *Handler) fetchUserByID(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := h.svc.FetchUserByID(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// deleteUser handles DELETE /delete_user_data/{id}. Deleting a missing id
// surfaces as 500 (EmptyResultDataAccessException in Spring Data 2.x).
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteUser(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	h.writeText(w, "user data deleted Successfully")
}

// updateUser handles PUT /update_user_data/{id}. No bean validation is
// applied (the source had no @Valid). The request body is echoed back.
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	id, err := parseID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	u, err := decodeUser(w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.svc.UpdateUser(r.Context(), id, u); err != nil {
		writeError(w, r, err)
		return
	}
	// MIGRATION_NOTE: Java passed the same User instance to the service,
	// which set the path id on it before saving; the echoed body therefore
	// carried the path id. Go passes by value, so the id is applied here.
	u.ID = id
	writeJSON(w, http.StatusOK, u)
}

// getUserByName handles GET /get_user_name/name/{name}. When no user matches
// the response is 200 with an empty body (Java returned null); multiple
// matches surface as 500.
func (h *Handler) getUserByName(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	u, ok, err := h.svc.GetUserByName(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
