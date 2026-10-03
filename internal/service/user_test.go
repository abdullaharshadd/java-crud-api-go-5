package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

type fakeRepo struct {
	saveCalls   int
	saveArg     model.User
	saveRet     model.User
	saveErr     error
	findAllRet  []model.User
	findAllErr  error
	findAllCall int
	findByIDRet model.User
	findByIDOK  bool
	findByIDErr error
	findByIDArg int32
	deleteArg   int32
	deleteCalls int
	deleteErr   error
	byNameArg   string
	byNameRet   model.User
	byNameOK    bool
	byNameErr   error
}

func (f *fakeRepo) Save(ctx context.Context, u model.User) (model.User, error) {
	f.saveCalls++
	f.saveArg = u
	return f.saveRet, f.saveErr
}
func (f *fakeRepo) FindAll(ctx context.Context) ([]model.User, error) {
	f.findAllCall++
	return f.findAllRet, f.findAllErr
}
func (f *fakeRepo) FindByID(ctx context.Context, id int32) (model.User, bool, error) {
	f.findByIDArg = id
	return f.findByIDRet, f.findByIDOK, f.findByIDErr
}
func (f *fakeRepo) DeleteByID(ctx context.Context, id int32) error {
	f.deleteCalls++
	f.deleteArg = id
	return f.deleteErr
}
func (f *fakeRepo) Count(ctx context.Context) (int64, error) { return 0, nil }
func (f *fakeRepo) FindByName(ctx context.Context, name string) (model.User, bool, error) {
	f.byNameArg = name
	return f.byNameRet, f.byNameOK, f.byNameErr
}

var _ repository.UserRepository = (*fakeRepo)(nil)

var errDB = errors.New("db down")

func hemraj() model.User {
	return model.NewUserBuilder().ID(3).Name("hemraj").Email("hemrajmalhi1234@gmail.com").
		About("Sr").Role("java developer").Password("pw").Build()
}

func TestSaveUser(t *testing.T) {
	in := model.NewUserBuilder().Name("a").Build()
	saved := model.NewUserBuilder().ID(7).Name("a").Build()
	tests := []struct {
		name    string
		retErr  error
		want    model.User
		wantErr bool
	}{
		{"success returns repo result", nil, saved, false},
		{"repo error wrapped", errDB, model.User{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{saveRet: saved, saveErr: tc.retErr}
			got, err := NewUserService(r).SaveUser(context.Background(), in)
			if r.saveCalls != 1 {
				t.Fatalf("save calls = %d, want 1", r.saveCalls)
			}
			if !r.saveArg.Equal(in) {
				t.Errorf("save arg = %v, want %v", r.saveArg, in)
			}
			if tc.wantErr {
				if !errors.Is(err, errDB) {
					t.Fatalf("err = %v, want wrapping errDB", err)
				}
				if IsNotFound(err) {
					t.Error("unexpected NotFoundError")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err %v", err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFetchUserList(t *testing.T) {
	users := []model.User{hemraj(), model.NewUserBuilder().ID(4).Name("b").Build()}
	tests := []struct {
		name    string
		ret     []model.User
		err     error
		wantLen int
		wantErr bool
	}{
		{"users exist", users, nil, 2, false},
		{"nil becomes empty", nil, nil, 0, false},
		{"empty slice", []model.User{}, nil, 0, false},
		{"repo error", nil, errDB, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{findAllRet: tc.ret, findAllErr: tc.err}
			got, err := NewUserService(r).FetchUserList(context.Background())
			if r.findAllCall != 1 {
				t.Errorf("FindAll calls = %d", r.findAllCall)
			}
			if r.saveCalls != 0 || r.deleteCalls != 0 {
				t.Error("read-only operation modified data")
			}
			if tc.wantErr {
				if !errors.Is(err, errDB) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("got nil slice")
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			for i := range got {
				if !got[i].Equal(tc.ret[i]) {
					t.Errorf("user %d = %v", i, got[i])
				}
			}
		})
	}
}

func TestFetchUserByID(t *testing.T) {
	tests := []struct {
		name         string
		ok           bool
		err          error
		wantNotFound bool
		wantErr      error
	}{
		{"found", true, nil, false, nil},
		{"not found", false, nil, true, ErrUserNotFound},
		{"repo error", false, errDB, false, errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{findByIDRet: hemraj(), findByIDOK: tc.ok, findByIDErr: tc.err}
			got, err := NewUserService(r).FetchUserByID(context.Background(), 3)
			if r.findByIDArg != 3 {
				t.Errorf("FindByID arg = %d", r.findByIDArg)
			}
			if r.saveCalls != 0 || r.deleteCalls != 0 {
				t.Error("read-only operation modified data")
			}
			if tc.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
				if !got.Equal(hemraj()) {
					t.Errorf("got %v", got)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if IsNotFound(err) != tc.wantNotFound {
				t.Errorf("IsNotFound = %v", IsNotFound(err))
			}
			if tc.wantNotFound {
				var nf *NotFoundError
				if !errors.As(err, &nf) {
					t.Fatal("not *NotFoundError")
				}
				if err.Error() != "User are not available" || err.Error() != UserNotFoundMessage {
					t.Errorf("message = %q", err.Error())
				}
			}
			if !got.Equal(model.User{}) {
				t.Errorf("expected zero user on error, got %v", got)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	missing := fmt.Errorf("repository: delete user 9: %w", repository.ErrEmptyResult)
	tests := []struct {
		name    string
		id      int32
		err     error
		wantErr error
	}{
		{"existing", 3, nil, nil},
		{"missing", 9, missing, repository.ErrEmptyResult},
		{"db error", 3, errDB, errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{deleteErr: tc.err}
			err := NewUserService(r).DeleteUser(context.Background(), tc.id)
			if r.deleteCalls != 1 || r.deleteArg != tc.id {
				t.Errorf("delete calls=%d arg=%d", r.deleteCalls, r.deleteArg)
			}
			if r.findByIDArg != 0 {
				t.Error("service performed existence check")
			}
			if tc.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v", err)
			}
			if IsNotFound(err) {
				t.Error("DeleteUser must not return NotFoundError")
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	tests := []struct {
		name    string
		id      int32
		in      model.User
		err     error
		wantErr bool
	}{
		{"different id overridden", 5, model.NewUserBuilder().ID(99).Name("x").Email("e").Build(), nil, false},
		{"no id", 6, model.NewUserBuilder().Name("y").Build(), nil, false},
		{"nonexistent id still saves", 1000, model.NewUserBuilder().Name("z").Build(), nil, false},
		{"repo error", 5, model.NewUserBuilder().Name("x").Build(), errDB, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{saveErr: tc.err}
			err := NewUserService(r).UpdateUser(context.Background(), tc.id, tc.in)
			if r.saveCalls != 1 {
				t.Fatalf("save calls = %d", r.saveCalls)
			}
			if r.findByIDArg != 0 {
				t.Error("unexpected not-found check")
			}
			want := tc.in
			want.ID = tc.id
			if !r.saveArg.Equal(want) {
				t.Errorf("saved %v, want %v", r.saveArg, want)
			}
			if tc.wantErr {
				if !errors.Is(err, errDB) || IsNotFound(err) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGetUserByName(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		ret     model.User
		ok      bool
		err     error
		wantOK  bool
		wantErr error
	}{
		{"found hemraj", "hemraj", hemraj(), true, nil, true, nil},
		{"not found", "nobody", model.User{}, false, nil, false, nil},
		{"non unique", "dup", model.User{}, false,
			fmt.Errorf("repository: find user by name: %w", repository.ErrNonUniqueResult), false, repository.ErrNonUniqueResult},
		{"db error", "x", model.User{}, false, errDB, false, errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRepo{byNameRet: tc.ret, byNameOK: tc.ok, byNameErr: tc.err}
			got, ok, err := NewUserService(r).GetUserByName(context.Background(), tc.query)
			if r.byNameArg != tc.query {
				t.Errorf("query arg = %q", r.byNameArg)
			}
			if r.saveCalls != 0 || r.deleteCalls != 0 {
				t.Error("read-only operation modified data")
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || ok {
					t.Fatalf("err=%v ok=%v", err, ok)
				}
				if IsNotFound(err) {
					t.Error("unexpected NotFoundError")
				}
				if !strings.Contains(err.Error(), "get user by name") {
					t.Errorf("err msg = %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v", ok)
			}
			if ok {
				if got.Name == nil || *got.Name != tc.query {
					t.Errorf("name = %v", got.Name)
				}
				if got.ID != 3 || *got.Email != "hemrajmalhi1234@gmail.com" || *got.About != "Sr" || *got.Role != "java developer" {
					t.Errorf("got %v", got)
				}
			}
		})
	}
}