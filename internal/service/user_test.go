package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/repository"
)

// ---------------------------------------------------------------------------
// In-memory fake of repository.UserRepository (mocks the DB).
// ---------------------------------------------------------------------------

type tstFakeUserRepo struct {
	mu      sync.Mutex
	rows    map[int32]model.User
	next    int32
	saveErr error
	calls   map[string]int
}

func newTstFakeUserRepo() *tstFakeUserRepo {
	return &tstFakeUserRepo{rows: map[int32]model.User{}, next: 1, calls: map[string]int{}}
}

var _ repository.UserRepository = (*tstFakeUserRepo)(nil)

func (r *tstFakeUserRepo) Save(_ context.Context, u model.User) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls["Save"]++
	if r.saveErr != nil {
		return model.User{}, r.saveErr
	}
	if u.ID != 0 {
		if _, ok := r.rows[u.ID]; ok {
			r.rows[u.ID] = u
			return u, nil
		}
	}
	for {
		if _, ok := r.rows[r.next]; !ok {
			break
		}
		r.next++
	}
	u.ID = r.next
	r.next++
	r.rows[u.ID] = u
	return u, nil
}

func (r *tstFakeUserRepo) FindAll(_ context.Context) ([]model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls["FindAll"]++
	out := make([]model.User, 0, len(r.rows))
	for _, u := range r.rows {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *tstFakeUserRepo) FindByID(_ context.Context, id int32) (model.User, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls["FindByID"]++
	u, ok := r.rows[id]
	return u, ok, nil
}

func (r *tstFakeUserRepo) DeleteByID(_ context.Context, id int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls["DeleteByID"]++
	if _, ok := r.rows[id]; !ok {
		return errors.Join(errors.New("delete"), repository.ErrEmptyResult)
	}
	delete(r.rows, id)
	return nil
}

func (r *tstFakeUserRepo) Count(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.rows)), nil
}

func (r *tstFakeUserRepo) FindByName(_ context.Context, name string) (model.User, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls["FindByName"]++
	var found model.User
	n := 0
	for _, u := range r.rows {
		if u.Name != nil && *u.Name == name {
			found = u
			n++
		}
	}
	switch n {
	case 0:
		return model.User{}, false, nil
	case 1:
		return found, true, nil
	default:
		return model.User{}, false, errors.Join(errors.New("find by name"), repository.ErrNonUniqueResult)
	}
}

func (r *tstFakeUserRepo) snapshot() map[int32]model.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := make(map[int32]model.User, len(r.rows))
	for k, v := range r.rows {
		m[k] = v
	}
	return m
}

// ---------------------------------------------------------------------------
// Reference adapter satisfying the UserService contract by delegating to the
// repository, used to exercise the contract documented in user.go.
// ---------------------------------------------------------------------------

type tstContractService struct{ repo repository.UserRepository }

var _ UserService = (*tstContractService)(nil)

func (s *tstContractService) SaveUser(ctx context.Context, u model.User) (model.User, error) {
	return s.repo.Save(ctx, u)
}
func (s *tstContractService) FetchUserList(ctx context.Context) ([]model.User, error) {
	return s.repo.FindAll(ctx)
}
func (s *tstContractService) FetchUserByID(ctx context.Context, id int32) (model.User, error) {
	u, ok, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	if !ok {
		return model.User{}, ErrUserNotFound
	}
	return u, nil
}
func (s *tstContractService) DeleteUser(ctx context.Context, id int32) error {
	return s.repo.DeleteByID(ctx, id)
}
func (s *tstContractService) UpdateUser(ctx context.Context, id int32, u model.User) error {
	u.ID = id
	_, err := s.repo.Save(ctx, u)
	return err
}
func (s *tstContractService) GetUserByName(ctx context.Context, name string) (model.User, bool, error) {
	return s.repo.FindByName(ctx, name)
}

func tstNewSvc() (UserService, *tstFakeUserRepo) {
	r := newTstFakeUserRepo()
	return &tstContractService{repo: r}, r
}

func tstUser(name, email string) model.User {
	return model.NewUserBuilder().Name(name).Email(email).Password("pw").Role("ROLE_USER").About("about").Build()
}

func tstSeed(t *testing.T, svc UserService, users ...model.User) []model.User {
	t.Helper()
	out := make([]model.User, 0, len(users))
	for _, u := range users {
		s, err := svc.SaveUser(context.Background(), u)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		out = append(out, s)
	}
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSaveUser(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		seed    []model.User
		input   func(seeded []model.User) model.User
		repoErr error
		wantErr bool
		check   func(t *testing.T, in, got model.User, seeded []model.User, r *tstFakeUserRepo)
	}{
		{
			name:  "new user gets id and same fields",
			input: func([]model.User) model.User { return tstUser("alice", "a@x") },
			check: func(t *testing.T, in, got model.User, _ []model.User, r *tstFakeUserRepo) {
				if got.ID == 0 {
					t.Fatal("expected assigned id")
				}
				exp := in
				exp.ID = got.ID
				if !got.Equal(exp) {
					t.Fatalf("got %v want %v", got, exp)
				}
				if len(r.snapshot()) != 1 {
					t.Fatal("expected one stored record")
				}
			},
		},
		{
			name: "existing id is overwritten (upsert)",
			seed: []model.User{tstUser("bob", "b@x")},
			input: func(s []model.User) model.User {
				u := tstUser("bobby", "bb@x")
				u.ID = s[0].ID
				return u
			},
			check: func(t *testing.T, in, got model.User, s []model.User, r *tstFakeUserRepo) {
				if got.ID != s[0].ID || !got.Equal(in) {
					t.Fatalf("got %v want %v", got, in)
				}
				snap := r.snapshot()
				if len(snap) != 1 || !snap[s[0].ID].Equal(in) {
					t.Fatalf("record not overwritten: %v", snap)
				}
			},
		},
		{
			name:    "persistence error propagates",
			input:   func([]model.User) model.User { return tstUser("dup", "d@x") },
			repoErr: errors.New("duplicate entry 1062"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := tstNewSvc()
			seeded := tstSeed(t, svc, tc.seed...)
			repo.saveErr = tc.repoErr
			in := tc.input(seeded)
			got, err := svc.SaveUser(ctx, in)
			if tc.wantErr {
				if !errors.Is(err, tc.repoErr) {
					t.Fatalf("want %v, got %v", tc.repoErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, in, got, seeded, repo)
			// invariant: retrievable by id
			f, err := svc.FetchUserByID(ctx, got.ID)
			if err != nil || !f.Equal(got) {
				t.Fatalf("not retrievable: %v %v", f, err)
			}
		})
	}
}

func TestFetchUserList(t *testing.T) {
	tests := []struct {
		name string
		seed []model.User
	}{
		{"no users returns empty non-nil", nil},
		{"returns every user", []model.User{tstUser("a", "a@x"), tstUser("b", "b@x"), tstUser("c", "c@x")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := tstNewSvc()
			seeded := tstSeed(t, svc, tc.seed...)
			before := repo.snapshot()
			got, err := svc.FetchUserList(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("slice must not be nil")
			}
			if len(got) != len(seeded) {
				t.Fatalf("len %d want %d", len(got), len(seeded))
			}
			for _, s := range seeded {
				found := false
				for _, g := range got {
					if g.Equal(s) {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %v", s)
				}
			}
			if len(repo.snapshot()) != len(before) || repo.calls["Save"] != len(tc.seed) {
				t.Fatal("read must not modify store")
			}
		})
	}
}

func TestFetchUserByID(t *testing.T) {
	svc, _ := tstNewSvc()
	seeded := tstSeed(t, svc, tstUser("a", "a@x"), tstUser("b", "b@x"))
	tests := []struct {
		name    string
		id      int32
		want    *model.User
		wantNF  bool
	}{
		{"existing first", seeded[0].ID, &seeded[0], false},
		{"existing second", seeded[1].ID, &seeded[1], false},
		{"missing", 9999, nil, true},
		{"zero id", 0, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.FetchUserByID(context.Background(), tc.id)
			if tc.wantNF {
				if !errors.Is(err, ErrUserNotFound) || !IsNotFound(err) {
					t.Fatalf("want not found, got %v", err)
				}
				var nf *NotFoundError
				if !errors.As(err, &nf) || nf.Error() != UserNotFoundMessage {
					t.Fatalf("bad message: %v", err)
				}
				return
			}
			if err != nil || !got.Equal(*tc.want) {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	tests := []struct {
		name      string
		missing   bool
		wantEmpty bool
	}{
		{"existing user removed", false, false},
		{"missing id returns ErrEmptyResult", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, _ := tstNewSvc()
			s := tstSeed(t, svc, tstUser("a", "a@x"), tstUser("b", "b@x"))
			id := s[0].ID
			if tc.missing {
				id = 4242
			}
			err := svc.DeleteUser(ctx, id)
			if tc.wantEmpty {
				if !errors.Is(err, repository.ErrEmptyResult) {
					t.Fatalf("want ErrEmptyResult, got %v", err)
				}
				if IsNotFound(err) {
					t.Fatal("delete must not return NotFoundError")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := svc.FetchUserByID(ctx, id); !errors.Is(err, ErrUserNotFound) {
					t.Fatalf("expected not found after delete, got %v", err)
				}
			}
			other, err := svc.FetchUserByID(ctx, s[1].ID)
			if err != nil || !other.Equal(s[1]) {
				t.Fatal("other user affected")
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	tests := []struct {
		name    string
		missing bool
	}{
		{"existing id updated", false},
		{"missing id inserts new record", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, repo := tstNewSvc()
			s := tstSeed(t, svc, tstUser("a", "a@x"), tstUser("b", "b@x"))
			upd := tstUser("new", "new@x")
			upd.ID = 777 // must be overwritten by id argument
			id := s[0].ID
			if tc.missing {
				id = 5000
			}
			if err := svc.UpdateUser(ctx, id, upd); err != nil {
				t.Fatal(err)
			}
			if !tc.missing {
				got, err := svc.FetchUserByID(ctx, id)
				if err != nil || got.ID != id || *got.Name != "new" || *got.Email != "new@x" {
					t.Fatalf("not updated: %v %v", got, err)
				}
				if len(repo.snapshot()) != 2 {
					t.Fatal("update must not add records")
				}
				if _, ok := repo.snapshot()[777]; ok {
					t.Fatal("id from body must not be used")
				}
			} else if len(repo.snapshot()) != 3 {
				t.Fatalf("expected new record, got %d", len(repo.snapshot()))
			}
			other, _ := svc.FetchUserByID(ctx, s[1].ID)
			if !other.Equal(s[1]) {
				t.Fatal("other user affected")
			}
		})
	}
}

func TestGetUserByName(t *testing.T) {
	tests := []struct {
		name      string
		seed      []model.User
		query     string
		wantOK    bool
		wantNonUq bool
	}{
		{"match", []model.User{tstUser("alice", "a@x"), tstUser("bob", "b@x")}, "alice", true, false},
		{"no match", []model.User{tstUser("alice", "a@x")}, "zed", false, false},
		{"empty store", nil, "alice", false, false},
		{"multiple match", []model.User{tstUser("dup", "1@x"), tstUser("dup", "2@x")}, "dup", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := tstNewSvc()
			tstSeed(t, svc, tc.seed...)
			before := len(repo.snapshot())
			got, ok, err := svc.GetUserByName(context.Background(), tc.query)
			if tc.wantNonUq {
				if !errors.Is(err, repository.ErrNonUniqueResult) {
					t.Fatalf("want non-unique, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if ok && (got.Name == nil || *got.Name != tc.query) {
				t.Fatalf("name mismatch: %v", got)
			}
			if len(repo.snapshot()) != before {
				t.Fatal("read must not modify store")
			}
		})
	}
}