package model

import (
	"encoding/json"
	"testing"
)

func userStrp(s string) *string { return &s }

func TestUserZeroValue(t *testing.T) {
	var u User
	if got := u.String(); got != "User(id=0, name=null, email=null, password=null, role=null, about=null)" {
		t.Fatalf("String() = %q", got)
	}
	if err := u.Validate(); err != ErrUserNameBlank {
		t.Fatalf("Validate() = %v, want ErrUserNameBlank", err)
	}
}

func TestNewUser(t *testing.T) {
	u := NewUser(1, userStrp("n"), userStrp("e"), userStrp("p"), userStrp("r"), userStrp("a"))
	if u.ID != 1 || *u.Name != "n" || *u.Email != "e" || *u.Password != "p" || *u.Role != "r" || *u.About != "a" {
		t.Fatalf("unexpected user %v", u)
	}
	if got := u.String(); got != "User(id=1, name=n, email=e, password=p, role=r, about=a)" {
		t.Fatalf("String() = %q", got)
	}
}

func TestUserValidate(t *testing.T) {
	tests := []struct {
		name    string
		n       *string
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", userStrp(""), true},
		{"blank", userStrp(" \t\n"), true},
		{"ok", userStrp("alice"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := User{Name: tt.n}.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserEqual(t *testing.T) {
	a := NewUserBuilder().ID(1).Name("x").Build()
	b := NewUserBuilder().ID(1).Name("x").Build()
	c := NewUserBuilder().ID(1).Name("y").Build()
	d := NewUserBuilder().ID(1).Build()
	if !a.Equal(b) || !b.Equal(a) {
		t.Fatal("expected equal")
	}
	if a.Equal(c) || a.Equal(d) || d.Equal(a) {
		t.Fatal("expected not equal")
	}
	if !(User{}).Equal(User{}) {
		t.Fatal("zero values should be equal")
	}
}

func TestUserBuilder(t *testing.T) {
	u := NewUserBuilder().ID(5).Name("n").Email("e").Password("p").Role("r").About("a").Build()
	want := NewUser(5, userStrp("n"), userStrp("e"), userStrp("p"), userStrp("r"), userStrp("a"))
	if !u.Equal(want) {
		t.Fatalf("got %v, want %v", u, want)
	}
}

func TestUserJSON(t *testing.T) {
	u := NewUserBuilder().ID(2).Name("n").Build()
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":2,"name":"n","email":null,"password":null,"role":null,"about":null}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
	var back User
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Equal(u) {
		t.Fatalf("round trip got %v", back)
	}
}