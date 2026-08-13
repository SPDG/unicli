package access

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SPDG/unicli/internal/client"
)

func TestParseListShapes(t *testing.T) {
	arr, err := parseList[Door](json.RawMessage(`[{"id":"d1","name":"Front"}]`))
	if err != nil || len(arr) != 1 || arr[0].Name != "Front" {
		t.Fatalf("%v %+v", err, arr)
	}
	env, err := parseList[Door](json.RawMessage(`{"code":"SUCCESS","data":[{"id":"d2","name":"Back"}]}`))
	if err != nil || env[0].ID != "d2" {
		t.Fatalf("%v %+v", err, env)
	}
	items, err := parseList[Door](json.RawMessage(`{"data":{"items":[{"id":"d3","name":"Side"}]}}`))
	if err != nil || items[0].Name != "Side" {
		t.Fatalf("%v %+v", err, items)
	}
}

func TestDoorsAndUnlock(t *testing.T) {
	var unlocked string
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/access/api/v2/doors", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Door{{ID: "door-1", Name: "Front"}})
	})
	mux.HandleFunc("/proxy/access/api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]User{{ID: "u1", Name: "Ada", Email: "ada@example.com"}})
	})
	mux.HandleFunc("/proxy/access/api/v2/doors/door-1/unlock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method=%s", r.Method)
		}
		unlocked = "door-1"
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/proxy/access/api/v2/doors/door-1/lock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("lock method=%s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/proxy/access/api/v2/visitors", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "v1", "name": "Guest", "status": "ACTIVE"}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	api := New(c)
	info, err := api.Info(context.Background())
	if err != nil || !info.Available {
		t.Fatalf("%v %+v", err, info)
	}
	doors, err := api.Doors(context.Background())
	if err != nil || doors[0].ID != "door-1" {
		t.Fatalf("%v %+v", err, doors)
	}
	door, err := api.Door(context.Background(), "door-1")
	if err != nil || door.Name != "Front" {
		t.Fatalf("%v %+v", err, door)
	}
	if _, err := api.Door(context.Background(), "missing"); err == nil {
		t.Fatal("expected missing door error")
	}
	if err := api.UnlockDoor(context.Background(), "door-1"); err != nil {
		t.Fatal(err)
	}
	if unlocked != "door-1" {
		t.Fatal("unlock not called")
	}
	if err := api.LockDoor(context.Background(), "door-1"); err != nil {
		t.Fatal(err)
	}
	vis, err := api.Collection(context.Background(), "visitors")
	if err != nil || vis[0]["name"] != "Guest" {
		t.Fatalf("%v %+v", err, vis)
	}
	users, err := api.Users(context.Background())
	if err != nil || users[0].Email != "ada@example.com" {
		t.Fatalf("%v %+v", err, users)
	}
	user, err := api.User(context.Background(), "u1")
	if err != nil || user.Name != "Ada" {
		t.Fatalf("%v %+v", err, user)
	}
}

func TestInfoWhenAccessMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><title>UniFi OS</title></html>`))
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(c).Info(context.Background())
	var ue client.AppUnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want AppUnavailableError, got %T %v", err, err)
	}
}
