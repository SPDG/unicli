package console

import (
	"context"

	"github.com/SPDG/unicli/internal/access"
	"github.com/SPDG/unicli/internal/client"
	"github.com/SPDG/unicli/internal/network"
	"github.com/SPDG/unicli/internal/protect"
)

// API talks to UniFi OS console endpoints (not Network/Protect Integration).
type API struct {
	c *client.Client
}

func New(c *client.Client) *API {
	return &API{c: c}
}

type AppStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *API) System(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := a.c.GetPathJSON(ctx, "/api/system", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) OSApps(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := a.c.GetPathJSON(ctx, "/api/apps", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) FirmwareUpdate(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := a.c.GetPathJSON(ctx, "/api/firmware/update", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) Reboot(ctx context.Context) error {
	return a.c.PostPathJSON(ctx, "/api/system/reboot", map[string]any{}, nil)
}

func (a *API) ProbeApps(ctx context.Context) []AppStatus {
	out := make([]AppStatus, 0, 3)
	n := network.New(a.c)
	if info, err := n.Info(ctx); err == nil {
		out = append(out, AppStatus{Name: "network", Available: true, Version: info.ApplicationVersion})
	} else {
		out = append(out, AppStatus{Name: "network", Available: false, Error: err.Error()})
	}
	if info, err := protect.New(a.c).Info(ctx); err == nil {
		out = append(out, AppStatus{Name: "protect", Available: true, Version: info.ApplicationVersion})
	} else {
		out = append(out, AppStatus{Name: "protect", Available: false, Error: err.Error()})
	}
	if info, err := access.New(a.c).Info(ctx); err == nil && info.Available {
		out = append(out, AppStatus{Name: "access", Available: true, Version: info.Version, Error: ""})
	} else {
		msg := ""
		if err != nil {
			msg = err.Error()
		} else if info != nil {
			msg = info.Message
		}
		out = append(out, AppStatus{Name: "access", Available: false, Error: msg})
	}
	return out
}
