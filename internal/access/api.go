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
	ID     string `json:"id"`
	Name   string `json:"name"`
	FullName string `json:"full_name,omitempty"`
	Type   string `json:"type,omitempty"`
	IsLocked *bool `json:"is_locked,omitempty"`
	DoorLockRelayStatus string `json:"door_lock_relay_status,omitempty"`
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
	raw, err := a.getDoorsRaw(ctx)
	if err != nil {
		return nil, err
	}
	return parseDoors(raw)
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

func (a *API) getDoorsRaw(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := a.c.GetPathJSON(ctx, "/proxy/access/api/v2/doors", nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseDoors(raw json.RawMessage) ([]Door, error) {
	// Shape A: bare array
	var arr []Door
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// Shape B: {code,msg,data:[...]} or {data:{items:[...]}}
	var env doorsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode doors: %w", err)
	}
	if len(env.Data) == 0 {
		return []Door{}, nil
	}
	if err := json.Unmarshal(env.Data, &arr); err == nil {
		return arr, nil
	}
	var wrapped struct {
		Items []Door `json:"items"`
		Data  []Door `json:"data"`
	}
	if err := json.Unmarshal(env.Data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode doors data: %w", err)
	}
	if len(wrapped.Items) > 0 {
		return wrapped.Items, nil
	}
	return wrapped.Data, nil
}
