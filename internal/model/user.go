// Package model defines the domain entities of the Smart Contact Manager.
package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Column and table names follow Hibernate's default Spring physical naming
// strategy (lowercased), which is what ddl-auto=update created in the
// existing MySQL schema for @Table(name = "USER") and @Column(name = "User_*").
const (
	// UserTable is the table backing the User entity.
	UserTable = "user"
	// ColUserID is the primary key column.
	ColUserID = "user_id"
	// ColUserName is the user name column.
	ColUserName = "user_name"
	// ColUserEmail is the unique e-mail column.
	ColUserEmail = "user_email"
	// ColUserPassword is the password column.
	ColUserPassword = "user_password"
	// ColUserRole is the role column.
	ColUserRole = "user_role"
	// ColUserAbout is the "about" column (max length 500).
	ColUserAbout = "user_about"
)

// userSchemaDDL reproduces what Hibernate's ddl-auto=update generated for the
// User entity on MySQL with GenerationType.AUTO (Hibernate 5 => a shared
// hibernate_sequence table rather than AUTO_INCREMENT).
var userSchemaDDL = []string{
	"CREATE TABLE IF NOT EXISTS hibernate_sequence (next_val BIGINT) ENGINE=InnoDB",
	"INSERT INTO hibernate_sequence (next_val) SELECT 1 FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM hibernate_sequence)",
	"CREATE TABLE IF NOT EXISTS `user` (" +
		"user_id INT NOT NULL, " +
		"user_about VARCHAR(500), " +
		"user_email VARCHAR(255), " +
		"user_name VARCHAR(255), " +
		"user_password VARCHAR(255), " +
		"user_role VARCHAR(255), " +
		"PRIMARY KEY (user_id), " +
		"UNIQUE KEY uk_user_email (user_email)" +
		") ENGINE=InnoDB",
}

// EnsureUserSchema creates the user table and the hibernate_sequence table if
// they do not exist yet. It replaces spring.jpa.hibernate.ddl-auto=update.
func EnsureUserSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("model: ensure user schema: nil db")
	}
	for _, stmt := range userSchemaDDL {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("model: ensure user schema: %w", err)
		}
	}
	return nil
}

// ErrUserNameBlank is returned by Validate when Name is nil or blank,
// mirroring the source's @NotBlank constraint.
var ErrUserNameBlank = errors.New("please Add the department Name")

// User is the application user. String fields are pointers so that SQL NULL
// and JSON null round-trip exactly like Java's nullable String.
type User struct {
	ID       int32   `json:"id"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
	About    *string `json:"about"`
}

// NewUser creates a User with every field set, in declaration order
// (the equivalent of Lombok's @AllArgsConstructor). The zero value User{}
// is the equivalent of @NoArgsConstructor.
func NewUser(id int32, name, email, password, role, about *string) User {
	return User{ID: id, Name: name, Email: email, Password: password, Role: role, About: about}
}

// Validate enforces @NotBlank on Name using Java String.trim semantics
// (strips every rune <= ' ' from both ends).
func (u User) Validate() error {
	if u.Name == nil || javaTrim(*u.Name) == "" {
		return ErrUserNameBlank
	}
	return nil
}

func javaTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
}

// Equal reports value equality across all six fields (Lombok @EqualsAndHashCode).
func (u User) Equal(o User) bool {
	return u.ID == o.ID &&
		strPtrEqual(u.Name, o.Name) &&
		strPtrEqual(u.Email, o.Email) &&
		strPtrEqual(u.Password, o.Password) &&
		strPtrEqual(u.Role, o.Role) &&
		strPtrEqual(u.About, o.About)
}

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// String renders every field in Lombok's toString format.
func (u User) String() string {
	return "User(id=" + strconv.FormatInt(int64(u.ID), 10) +
		", name=" + strOrNull(u.Name) +
		", email=" + strOrNull(u.Email) +
		", password=" + strOrNull(u.Password) +
		", role=" + strOrNull(u.Role) +
		", about=" + strOrNull(u.About) + ")"
}

func strOrNull(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// UserBuilder is a fluent builder for User (Lombok @Builder).
type UserBuilder struct {
	u User
}

// NewUserBuilder returns an empty UserBuilder.
func NewUserBuilder() *UserBuilder { return &UserBuilder{} }

// ID sets the id.
func (b *UserBuilder) ID(id int32) *UserBuilder { b.u.ID = id; return b }

// Name sets the name.
func (b *UserBuilder) Name(v string) *UserBuilder { b.u.Name = &v; return b }

// Email sets the e-mail.
func (b *UserBuilder) Email(v string) *UserBuilder { b.u.Email = &v; return b }

// Password sets the password.
func (b *UserBuilder) Password(v string) *UserBuilder { b.u.Password = &v; return b }

// Role sets the role.
func (b *UserBuilder) Role(v string) *UserBuilder { b.u.Role = &v; return b }

// About sets the about text.
func (b *UserBuilder) About(v string) *UserBuilder { b.u.About = &v; return b }

// Build returns the constructed User.
func (b *UserBuilder) Build() User { return b.u }
