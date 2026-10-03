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

// Handler exposes the User REST endpoints. It replaces the Java
// UserController (@RestController) and receives its dependencies through
// NewHandler instead of @Autowired field injection.
type Handler struct {
	svc service.UserService
	log zerolog.Logger
}

// NewHandler returns a Handler that delegates to svc and logs through logger.
func NewHandler(svc service.UserService, logger zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: logger}
}

// Routes returns an http.Handler with every User endpoint registered, a
// Spring Boot style 404 fallback and trailing-slash tolerance.
//
// Routes are registered without a method prefix; each handler checks the
// method itself so that 405 responses carry an Allow header and an empty
// body, as Spring did.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/save_user_data", h.saveUser)
	mux.HandleFunc("/get_user_data", h.fetchUserList)
	mux.HandleFunc("/get_user_data/{id}", h.fetchUserByID)
	mux.HandleFunc("/delete_user_data/{id}", h.deleteUser)
	mux.HandleFunc("/update_user_data/{id}", h.updateUser)
	mux.HandleFunc("/get_user_name/name/{name}", h.getUserNameByName)
	mux.Handle("/", NotFoundHandler())
	return stripTrailingSlash(mux)
}

// stripTrailingSlash mirrors Spring 2.7's trailing-slash matching: a request
// for "/get_user_data/" is served by the "/get_user_data" route.
func stripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if len(p) > 1 && strings.HasSuffix(p, "/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimRight(p, "/")
			if r2.URL.Path == "" {
				r2.URL.Path = "/"
			}
			r2.URL.RawPath = ""
			r = r2
		}
		next.ServeHTTP(w, r)
	})
}

// checkMethod reports whether the request should continue to the handler
// body. GET routes also accept HEAD; OPTIONS answers 200 with Allow; any
// other method answers 405 with Allow and an empty body.
func checkMethod(w http.ResponseWriter, r *http.Request, method string) bool {
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
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		w.WriteHeader(http.StatusOK)
		return false
	default:
		writeError(w, r, &MethodNotAllowedError{Allowed: allowed})
		return false
	}
}

// pathID parses the {id} path variable as a 32-bit signed int, like
// Spring's conversion to Java int. Failures map to 400.
func pathID(r *http.Request) (int32, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid id %q: %v", ErrBadRequest, raw, err)
	}
	return int32(id), nil
}

// decodeUser reads a User JSON body. Unknown fields are ignored (Jackson
// FAIL_ON_UNKNOWN_PROPERTIES=false). A missing, null or malformed body is a
// 400; a non-JSON Content-Type is a 415.
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
		return model.User{}, fmt.Errorf("%w: request body is null", ErrBadRequest)
	}
	return *u, nil
}

// writeText writes a plain-text 200 response, like ResponseEntity<String>.
func writeText(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", textPlainUTF8)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// saveUser handles POST /save_user_data.
func (h *Handler) saveUser(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodPost) {
		return
	}
	u, err := decodeUser(w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	// @Valid: Bean Validation failure → 400.
	if err := u.Validate(); err != nil {
		writeError(w, r, fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}
	h.log.Info().Msg("inside the saveUser of UserController ")
	if _, err := h.svc.SaveUser(r.Context(), u); err != nil {
		writeError(w, r, err)
		return
	}
	writeText(w, "User data saved successfully!")
}

// fetchUserList handles GET /get_user_data.
func (h *Handler) fetchUserList(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodGet) {
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
// service's NotFoundError, which writeError maps to 404.
func (h *Handler) fetchUserByID(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodGet) {
		return
	}
	id, err := pathID(r)
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
// surfaces as 500, like Spring Data's EmptyResultDataAccessException.
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodDelete) {
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteUser(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	writeText(w, "user data deleted Successfully")
}

// updateUser handles PUT /update_user_data/{id}. There is no validation
// (@Valid was absent). The request body is echoed back rather than the
// persisted entity.
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodPut) {
		return
	}
	id, err := pathID(r)
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
	// MIGRATION_NOTE: the Java service mutated the same User instance
	// (user.setId(id)) before the controller returned it. Go passes the User
	// by value, so the path id is applied here to keep the echoed body the same.
	u.ID = id
	writeJSON(w, http.StatusOK, u)
}

// getUserNameByName handles GET /get_user_name/name/{name}. When no user
// matches, Spring wrote 200 with an empty body (null return value).
// Multiple matches surface as 500.
func (h *Handler) getUserNameByName(w http.ResponseWriter, r *http.Request) {
	if !checkMethod(w, r, http.MethodGet) {
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
