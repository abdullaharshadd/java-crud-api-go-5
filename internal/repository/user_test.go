package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return stubConn{}, nil }

type stubConn struct{}

func (stubConn) Prepare(q string) (driver.Stmt, error) { return stubStmt{q: q}, nil }
func (stubConn) Close() error                          { return nil }
func (stubConn) Begin() (driver.Tx, error)             { return nil, errors.New("no tx") }

type stubStmt struct{ q string }

func (stubStmt) Close() error  { return nil }
func (stubStmt) NumInput() int { return -1 }

func (stubStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }

func (s stubStmt) Query([]driver.Value) (driver.Rows, error) {
	if strings.Contains(s.q, "COUNT(*)") {
		return &stubRows{cols: []string{"c"}, data: [][]driver.Value{{int64(3)}}}, nil
	}
	return &stubRows{cols: []string{"user_id", "user_name", "user_email", "user_password", "user_role", "user_about"}}, nil
}

type stubRows struct {
	cols []string
	data [][]driver.Value
	i    int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.i])
	r.i++
	return nil
}

func init() { sql.Register("repo_user_stub", stubDriver{}) }

func openStub(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("repo_user_stub", "x")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStubCountAndMissing(t *testing.T) {
	db := openStub(t)
	ctx := context.Background()

	var n int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `user`").Scan(&n); err != nil || n != 3 {
		t.Fatalf("count = %d, %v", n, err)
	}

	var id int32
	err := db.QueryRowContext(ctx, "SELECT user_id FROM `user` WHERE user_id = ?", 1).Scan(&id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}

	res, err := db.ExecContext(ctx, "DELETE FROM `user` WHERE user_id = ?", 1)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if affected, err := res.RowsAffected(); err != nil || affected != 0 {
		t.Fatalf("rows affected = %d, %v", affected, err)
	}

	rows, err := db.QueryContext(ctx, "SELECT user_id, user_name, user_email, user_password, user_role, user_about FROM `user`")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil || count != 0 {
		t.Fatalf("rows = %d, %v", count, err)
	}
}