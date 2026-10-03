// Package repository contains the persistence layer of the smart-contact
// service. It replaces the Spring Data JPA repositories of the original
// application with explicit database/sql implementations for MySQL.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"migrated-app/internal/model"
)

// ErrEmptyResult mirrors Spring's EmptyResultDataAccessException: it is
// returned by DeleteByID when no row with the given id exists.
var ErrEmptyResult = errors.New("repository: no entity with the given id exists")

// ErrNonUniqueResult mirrors Spring's IncorrectResultSizeDataAccessException:
// it is returned by FindByName when more than one user has the given name.
var ErrNonUniqueResult = errors.New("repository: query did not return a unique result")

// UserRepository is the persistence contract for model.User. It covers the
// JpaRepository operations the application relies on plus the derived
// findByName query of the original UserDao.
type UserRepository interface {
	// Save inserts u when u.ID is 0 (allocating a new id) or fully replaces
	// the stored row otherwise. If no row with u.ID exists, it is inserted
	// under a freshly generated id (JPA merge semantics). The persisted user
	// is returned.
	Save(ctx context.Context, u model.User) (model.User, error)
	// FindAll returns every user. The slice is never nil.
	FindAll(ctx context.Context) ([]model.User, error)
	// FindByID loads a user by primary key. The bool is false if none exists.
	FindByID(ctx context.Context, id int32) (model.User, bool, error)
	// DeleteByID deletes a user by primary key, returning ErrEmptyResult if
	// no such user exists.
	DeleteByID(ctx context.Context, id int32) error
	// Count returns the number of stored users.
	Count(ctx context.Context) (int64, error)
	// FindByName loads the single user whose name exactly equals name. The
	// bool is false if none exists; ErrNonUniqueResult is returned if more
	// than one user matches.
	FindByName(ctx context.Context, name string) (model.User, bool, error)
}

// MySQLUserRepository implements UserRepository on top of MySQL using the
// schema created by model.EnsureUserSchema.
type MySQLUserRepository struct {
	db *sql.DB
}

// NewMySQLUserRepository returns a MySQL-backed UserRepository.
func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

// Compile-time interface check.
var _ UserRepository = (*MySQLUserRepository)(nil)

const userColumns = "user_id, user_name, user_email, user_password, user_role, user_about"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(s rowScanner) (model.User, error) {
	var (
		u                                  model.User
		name, email, password, role, about sql.NullString
	)
	if err := s.Scan(&u.ID, &name, &email, &password, &role, &about); err != nil {
		return model.User{}, err
	}
	u.Name = nullToPtr(name)
	u.Email = nullToPtr(email)
	u.Password = nullToPtr(password)
	u.Role = nullToPtr(role)
	u.About = nullToPtr(about)
	return u, nil
}

func nullToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func ptrToNull(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

// Save implements UserRepository.
func (r *MySQLUserRepository) Save(ctx context.Context, u model.User) (model.User, error) {
	if u.ID == 0 {
		return r.insertNew(ctx, u)
	}

	res, err := r.db.ExecContext(ctx,
		"UPDATE `user` SET user_name = ?, user_email = ?, user_password = ?, user_role = ?, user_about = ? WHERE user_id = ?",
		ptrToNull(u.Name), ptrToNull(u.Email), ptrToNull(u.Password), ptrToNull(u.Role), ptrToNull(u.About), u.ID)
	if err != nil {
		return model.User{}, fmt.Errorf("repository: update user %d: %w", u.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return model.User{}, fmt.Errorf("repository: update user %d: rows affected: %w", u.ID, err)
	}
	if n > 0 {
		return u, nil
	}

	// Without clientFoundRows=true MySQL reports 0 affected rows for an
	// UPDATE that changes nothing, so confirm the row is really absent
	// before falling back to an insert.
	exists, err := r.exists(ctx, u.ID)
	if err != nil {
		return model.User{}, err
	}
	if exists {
		return u, nil
	}
	// JPA merge of a detached entity whose id is unknown: insert a copy
	// under a freshly generated id.
	return r.insertNew(ctx, u)
}

func (r *MySQLUserRepository) exists(ctx context.Context, id int32) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM `user` WHERE user_id = ?", id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("repository: check user %d exists: %w", id, err)
	}
	return true, nil
}

func (r *MySQLUserRepository) insertNew(ctx context.Context, u model.User) (model.User, error) {
	id, err := r.nextID(ctx)
	if err != nil {
		return model.User{}, err
	}
	u.ID = id
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO `user` ("+userColumns+") VALUES (?, ?, ?, ?, ?, ?)",
		u.ID, ptrToNull(u.Name), ptrToNull(u.Email), ptrToNull(u.Password), ptrToNull(u.Role), ptrToNull(u.About))
	if err != nil {
		// Duplicate e-mail (MySQL error 1062) surfaces here; like Spring's
		// DataIntegrityViolationException it is propagated as a server error.
		return model.User{}, fmt.Errorf("repository: insert user: %w", err)
	}
	return u, nil
}

// nextID allocates an id from hibernate_sequence in its own transaction,
// reproducing Hibernate 5's table-backed GenerationType.AUTO generator.
//
// MIGRATION_NOTE: the sequence is seeded with 1 by model.EnsureUserSchema;
// to stay safe against pre-existing rows the allocated id is never lower
// than MAX(user_id)+1. Review if the database is shared with other
// Hibernate entities that also draw from hibernate_sequence.
func (r *MySQLUserRepository) nextID(ctx context.Context) (id int32, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("repository: next id: begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var next int64
	if err = tx.QueryRowContext(ctx, "SELECT next_val FROM hibernate_sequence LIMIT 1 FOR UPDATE").Scan(&next); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = errors.New("hibernate_sequence is empty")
		}
		return 0, fmt.Errorf("repository: next id: read sequence: %w", err)
	}

	var maxID sql.NullInt64
	if err = tx.QueryRowContext(ctx, "SELECT MAX(user_id) FROM `user`").Scan(&maxID); err != nil {
		return 0, fmt.Errorf("repository: next id: read max id: %w", err)
	}
	if maxID.Valid && maxID.Int64+1 > next {
		next = maxID.Int64 + 1
	}
	if next > int64(^uint32(0)>>1) {
		err = errors.New("id space exhausted")
		return 0, fmt.Errorf("repository: next id: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "UPDATE hibernate_sequence SET next_val = ?", next+1); err != nil {
		return 0, fmt.Errorf("repository: next id: advance sequence: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("repository: next id: commit: %w", err)
	}
	return int32(next), nil
}

// FindAll implements UserRepository.
func (r *MySQLUserRepository) FindAll(ctx context.Context) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+userColumns+" FROM `user`")
	if err != nil {
		return nil, fmt.Errorf("repository: find all users: %w", err)
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("repository: find all users: scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: find all users: iterate: %w", err)
	}
	return users, nil
}

// FindByID implements UserRepository.
func (r *MySQLUserRepository) FindByID(ctx context.Context, id int32) (model.User, bool, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM `user` WHERE user_id = ?", id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, false, nil
	}
	if err != nil {
		return model.User{}, false, fmt.Errorf("repository: find user %d: %w", id, err)
	}
	return u, true, nil
}

// DeleteByID implements UserRepository.
func (r *MySQLUserRepository) DeleteByID(ctx context.Context, id int32) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM `user` WHERE user_id = ?", id)
	if err != nil {
		return fmt.Errorf("repository: delete user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repository: delete user %d: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("repository: delete user %d: %w", id, ErrEmptyResult)
	}
	return nil
}

// Count implements UserRepository.
func (r *MySQLUserRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `user`").Scan(&n); err != nil {
		return 0, fmt.Errorf("repository: count users: %w", err)
	}
	return n, nil
}

// FindByName implements UserRepository (derived query findByName).
func (r *MySQLUserRepository) FindByName(ctx context.Context, name string) (model.User, bool, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+userColumns+" FROM `user` WHERE user_name = ? LIMIT 2", name)
	if err != nil {
		return model.User{}, false, fmt.Errorf("repository: find user by name: %w", err)
	}
	defer rows.Close()

	var (
		found model.User
		count int
	)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return model.User{}, false, fmt.Errorf("repository: find user by name: scan: %w", err)
		}
		found = u
		count++
	}
	if err := rows.Err(); err != nil {
		return model.User{}, false, fmt.Errorf("repository: find user by name: iterate: %w", err)
	}
	switch count {
	case 0:
		return model.User{}, false, nil
	case 1:
		return found, true, nil
	default:
		return model.User{}, false, fmt.Errorf("repository: find user by name: %w", ErrNonUniqueResult)
	}
}
