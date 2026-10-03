package service

import (
	"context"
	"fmt"

	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

// UserService is the business-layer contract for User operations. It
// replaces the Java UserService interface; the concrete implementation is
// ported from UserServiceImp and is injected into HTTP handlers through this
// interface.
//
// MIGRATION_NOTE: IDs are int32 to match model.User.ID and
// repository.UserRepository (Java int is 32-bit).
type UserService interface {
	// SaveUser persists u and returns the saved user, which may now carry a
	// freshly generated id.
	SaveUser(ctx context.Context, u model.User) (model.User, error)

	// FetchUserList returns every persisted user. The slice is never nil.
	FetchUserList(ctx context.Context) ([]model.User, error)

	// FetchUserByID returns the user with the given id. If no such user
	// exists, the returned error matches ErrUserNotFound (a *NotFoundError
	// with message UserNotFoundMessage), replacing the Java checked
	// UserNotFoundException.
	FetchUserByID(ctx context.Context, id int32) (model.User, error)

	// DeleteUser deletes the user with the given id. Deleting a missing id
	// returns an error wrapping repository.ErrEmptyResult (Spring's
	// EmptyResultDataAccessException).
	DeleteUser(ctx context.Context, id int32) error

	// UpdateUser stores u under the given id, overwriting the id carried by
	// u (JPA save/merge semantics: inserts under a new id if none exists).
	UpdateUser(ctx context.Context, id int32, u model.User) error

	// GetUserByName looks up the single user whose name equals name. The
	// bool is false when no user matches (Java returned null); an error
	// wrapping repository.ErrNonUniqueResult is returned when several match.
	GetUserByName(ctx context.Context, name string) (model.User, bool, error)
}

// userService is the UserService implementation ported from the Java
// UserServiceImp. It is a thin delegation layer over the repository.
//
// MIGRATION_NOTE: the Java class had no @Transactional, so every repository
// call ran in its own transaction; this port likewise performs each
// operation as an independent repository call with no enclosing transaction.
type userService struct {
	repo repository.UserRepository
}

// NewUserService returns a UserService backed by repo. It replaces Spring's
// @Service component scanning and @Autowired field injection with explicit
// constructor injection; wire it in cmd/server/main.go.
func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

// Compile-time interface check.
var _ UserService = (*userService)(nil)

// SaveUser persists u via repository Save (insert when u.ID is 0, otherwise
// merge/upsert) and returns the saved entity. No validation is performed
// here, matching the original service.
func (s *userService) SaveUser(ctx context.Context, u model.User) (model.User, error) {
	saved, err := s.repo.Save(ctx, u)
	if err != nil {
		return model.User{}, fmt.Errorf("service: save user: %w", err)
	}
	return saved, nil
}

// FetchUserList returns all stored users.
func (s *userService) FetchUserList(ctx context.Context) ([]model.User, error) {
	users, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: fetch user list: %w", err)
	}
	if users == nil {
		users = make([]model.User, 0)
	}
	return users, nil
}

// FetchUserByID looks up a user by id, returning ErrUserNotFound (message
// "User are not available") when absent.
func (s *userService) FetchUserByID(ctx context.Context, id int32) (model.User, error) {
	u, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return model.User{}, fmt.Errorf("service: fetch user %d: %w", id, err)
	}
	if !ok {
		// Returned unwrapped so Error() is exactly UserNotFoundMessage, which
		// the HTTP layer surfaces verbatim in the 404 body.
		return model.User{}, ErrUserNotFound
	}
	return u, nil
}

// DeleteUser deletes the user with the given id. A missing id yields an
// error wrapping repository.ErrEmptyResult, as Spring Data's deleteById did.
func (s *userService) DeleteUser(ctx context.Context, id int32) error {
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("service: delete user %d: %w", id, err)
	}
	return nil
}

// UpdateUser sets the path id on u and saves it, overwriting the existing
// record or creating a new one (JPA merge semantics). There is no
// not-found check and no validation, matching the original service; the
// saved entity is discarded just as the Java void method did.
func (s *userService) UpdateUser(ctx context.Context, id int32, u model.User) error {
	u.ID = id
	if _, err := s.repo.Save(ctx, u); err != nil {
		return fmt.Errorf("service: update user %d: %w", id, err)
	}
	return nil
}

// GetUserByName returns the user whose name exactly matches name, delegating
// to the repository's FindByName (Java: getUserNameByName -> findByName).
// ok is false when no user matches (Java returned null).
func (s *userService) GetUserByName(ctx context.Context, name string) (model.User, bool, error) {
	u, ok, err := s.repo.FindByName(ctx, name)
	if err != nil {
		return model.User{}, false, fmt.Errorf("service: get user by name: %w", err)
	}
	return u, ok, nil
}
