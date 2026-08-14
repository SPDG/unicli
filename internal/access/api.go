package access

import (
	"context"
	"encoding/json"
	"errors"
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
	ID        string `json:"id,omitempty"`
	UniqueID  string `json:"unique_id,omitempty"`
	Name      string `json:"name,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status,omitempty"`
}

type doorsEnvelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (a *API) Info(ctx context.Context) (*Info, error) {
	var raw json.RawMessage
	err := a.c.GetPathJSON(ctx, "/proxy/access/api/v2/info", nil, &raw)
	if err != nil {
		var ue client.AppUnavailableError
		if errors.As(err, &ue) {
			return nil, ue
		}
		return nil, mapUnavailable(err, "/proxy/access/api/v2/info")
	}
	version := ""
	var env doorsEnvelope
	if json.Unmarshal(raw, &env) == nil && len(env.Data) > 0 {
		var data struct {
			Version string `json:"version"`
		}
		_ = json.Unmarshal(env.Data, &data)
		version = data.Version
	}
	return &Info{Available: true, Version: version, Message: "Access API reachable via UniFi OS proxy"}, nil
}

func (a *API) Doors(ctx context.Context) ([]Door, error) {
	raw, err := a.getRaw(ctx, "/proxy/access/api/v2/doors")
	if err != nil {
		if isJSONNotFound(err) {
			return []Door{}, nil
		}
		return nil, err
	}
	return parseList[Door](raw)
}

func (a *API) Users(ctx context.Context) ([]User, error) {
	raw, err := a.getRaw(ctx, "/proxy/access/api/v2/users")
	if err != nil {
		return nil, err
	}
	users, err := parseList[User](raw)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == "" {
			users[i].ID = users[i].UniqueID
		}
	}
	return users, nil
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

func mapUnavailable(err error, path string) error {
	var ue client.AppUnavailableError
	if errors.As(err, &ue) {
		return err
	}
	var ae client.APIError
	if errors.As(err, &ae) && (ae.Status == 404 || ae.Status == 502 || ae.Status == 503) {
		return client.AppUnavailableError{Path: path, Status: ae.Status}
	}
	return err
}

func isJSONNotFound(err error) bool {
	var ae client.APIError
	return errors.As(err, &ae) && ae.Status == 404
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
