package service

import (
	"context"

	"migrated-app/internal/model"
)

// UserService is the business-layer contract for User operations. It
// replaces the Java UserService interface; the concrete implementation
// (ported from UserServiceImp) lives in a sibling file of this package and
// is injected into HTTP handlers through this interface.
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
