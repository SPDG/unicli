package protect

import (
	"context"
	"net/url"

	"github.com/SPDG/unicli/internal/client"
)

type API struct {
	c *client.Client
}

func New(c *client.Client) *API {
	return &API{c: c}
}

type Info struct {
	ApplicationVersion string `json:"applicationVersion"`
}

type Camera struct {
	ID           string `json:"id"`
	ModelKey     string `json:"modelKey"`
	State        string `json:"state"`
	Name         string `json:"name"`
	Mac          string `json:"mac"`
	IsMicEnabled bool   `json:"isMicEnabled"`
}

func (a *API) Info(ctx context.Context) (*Info, error) {
	var out Info
	if err := a.c.GetJSON(ctx, "protect", "meta/info", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Cameras(ctx context.Context) ([]Camera, error) {
	var out []Camera
	if err := a.c.GetJSON(ctx, "protect", "cameras", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *API) Camera(ctx context.Context, id string) (*Camera, error) {
	var out Camera
	path := "cameras/" + url.PathEscape(id)
	if err := a.c.GetJSON(ctx, "protect", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
