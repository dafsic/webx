package postgres

import (
	"testing"

	sq "github.com/Masterminds/squirrel"
)

type crudUser struct {
	ID   *int64  `db:"id"`
	Name *string `db:"name"`
	Age  *int    `db:"age"`
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

func TestBuildInsert(t *testing.T) {
	u := crudUser{Name: strp("alice")} // Age nil -> skipped, id skipped
	q, args, err := BuildInsert("users", u)
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO users (name) VALUES ($1) RETURNING age, id, name"
	if q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	if len(args) != 1 {
		t.Fatalf("args = %v", args)
	}
	if p, ok := args[0].(*string); !ok || *p != "alice" {
		t.Errorf("args[0] = %v, want *string \"alice\"", args[0])
	}
}

func TestBuildUpdate(t *testing.T) {
	u := crudUser{Name: strp("bob"), Age: intp(30)}
	q, args, err := BuildUpdate("users", u, sq.Eq{"id": 7})
	if err != nil {
		t.Fatal(err)
	}
	want := "UPDATE users SET age = $1, name = $2 WHERE id = $3 RETURNING age, id, name"
	if q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	if len(args) != 3 {
		t.Errorf("args = %v", args)
	}
}

func TestBuildUpsert(t *testing.T) {
	u := crudUser{Name: strp("carol")}
	q, _, err := BuildUpsert("users", u, "name")
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO users (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING age, id, name"
	if q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
}

func TestBuildSelectAndDelete(t *testing.T) {
	q, args, err := BuildSelect("users", crudUser{}, sq.Eq{"id": 1})
	if err != nil {
		t.Fatal(err)
	}
	if q != "SELECT age, id, name FROM users WHERE id = $1" {
		t.Errorf("select query = %q", q)
	}
	if len(args) != 1 {
		t.Errorf("select args = %v", args)
	}

	q, _, err = BuildDelete("users", sq.Eq{"id": 1})
	if err != nil {
		t.Fatal(err)
	}
	if q != "DELETE FROM users WHERE id = $1" {
		t.Errorf("delete query = %q", q)
	}
}
