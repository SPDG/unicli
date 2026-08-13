package access

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/SPDG/unicli/internal/client"
)

// API talks to UniFi Access when the application is installed on the console.
// Official Access OpenAPI often uses a dedicated token/port; here we probe the
// UniFi OS proxy paths first so the same host/API key profile can detect presence.
type API struct {
	c *client.Client
}

func New(c *client.Client) *API {
	return &API{c: c}
}

type Info struct {
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
	Version   string `json:"version,omitempty"`
}

type Door struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	FullName            string `json:"full_name,omitempty"`
	Type                string `json:"type,omitempty"`
	IsLocked            *bool  `json:"is_locked,omitempty"`
	DoorLockRelayStatus string `json:"door_lock_relay_status,omitempty"`
}

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status,omitempty"`
}

type doorsEnvelope struct {
	Code   string          `json:"code"`
	Msg    string          `json:"msg"`
	Data   json.RawMessage `json:"data"`
}

func (a *API) Info(ctx context.Context) (*Info, error) {
	// Prefer a light probe that fails closed when Access is not installed.
	var raw json.RawMessage
	err := a.c.GetPathJSON(ctx, "/proxy/access/api/v2/doors", nil, &raw)
	if err != nil {
		return nil, err
	}
	return &Info{Available: true, Message: "Access API reachable via UniFi OS proxy"}, nil
}

func (a *API) Doors(ctx context.Context) ([]Door, error) {
	raw, err := a.getRaw(ctx, "/proxy/access/api/v2/doors")
	if err != nil {
		return nil, err
	}
	return parseList[Door](raw)
}

func (a *API) Users(ctx context.Context) ([]User, error) {
	raw, err := a.getRaw(ctx, "/proxy/access/api/v2/users")
	if err != nil {
		return nil, err
	}
	return parseList[User](raw)
}

func (a *API) User(ctx context.Context, id string) (*User, error) {
	users, err := a.Users(ctx)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == id {
			return &users[i], nil
		}
	}
	return nil, fmt.Errorf("user %q not found", id)
}

func (a *API) Door(ctx context.Context, id string) (*Door, error) {
	doors, err := a.Doors(ctx)
	if err != nil {
		return nil, err
	}
	for i := range doors {
		if doors[i].ID == id {
			return &doors[i], nil
		}
	}
	return nil, fmt.Errorf("door %q not found", id)
}

func (a *API) UnlockDoor(ctx context.Context, id string) error {
	path := "/proxy/access/api/v2/doors/" + url.PathEscape(id) + "/unlock"
	// Some consoles use PUT without body; others expect {}.
	return a.c.PutJSON(ctx, path, map[string]any{}, nil)
}

func (a *API) getRaw(ctx context.Context, path string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := a.c.GetPathJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseList[T any](raw json.RawMessage) ([]T, error) {
	var arr []T
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var env doorsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	if len(env.Data) == 0 {
		return []T{}, nil
	}
	if err := json.Unmarshal(env.Data, &arr); err == nil {
		return arr, nil
	}
	var wrapped struct {
		Items []T `json:"items"`
		Data  []T `json:"data"`
	}
	if err := json.Unmarshal(env.Data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode list data: %w", err)
	}
	if len(wrapped.Items) > 0 {
		return wrapped.Items, nil
	}
	return wrapped.Data, nil
}
