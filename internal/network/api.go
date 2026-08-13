package network

import (
	"context"
	"net/url"
	"strconv"

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

type Page[T any] struct {
	Offset     int `json:"offset"`
	Limit      int `json:"limit"`
	Count      int `json:"count"`
	TotalCount int `json:"totalCount"`
	Data       []T `json:"data"`
}

type Site struct {
	ID          string `json:"id"`
	InternalRef string `json:"internalReference"`
	Name        string `json:"name"`
}

type Device struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Model      string   `json:"model"`
	MacAddress string   `json:"macAddress"`
	IPAddress  string   `json:"ipAddress"`
	State      string   `json:"state"`
	Features   []string `json:"features"`
	Interfaces []string `json:"interfaces"`
}

type Client struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	MacAddress  string `json:"macAddress"`
	IPAddress   string `json:"ipAddress"`
	ConnectedAt string `json:"connectedAt"`
}

type DeviceStatistics struct {
	UptimeSec            int64   `json:"uptimeSec"`
	CPUUtilizationPct    float64 `json:"cpuUtilizationPct"`
	MemoryUtilizationPct float64 `json:"memoryUtilizationPct"`
	LoadAverage1Min      float64 `json:"loadAverage1Min"`
	LoadAverage5Min      float64 `json:"loadAverage5Min"`
	LoadAverage15Min     float64 `json:"loadAverage15Min"`
}

type actionRequest struct {
	Action string `json:"action"`
}

func (a *API) Info(ctx context.Context) (*Info, error) {
	var out Info
	if err := a.c.GetJSON(ctx, "network", "info", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Sites(ctx context.Context, offset, limit int) (*Page[Site], error) {
	var out Page[Site]
	if err := a.c.GetJSON(ctx, "network", "sites", pageQuery(offset, limit), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Devices(ctx context.Context, siteID string, offset, limit int) (*Page[Device], error) {
	var out Page[Device]
	path := "sites/" + url.PathEscape(siteID) + "/devices"
	if err := a.c.GetJSON(ctx, "network", path, pageQuery(offset, limit), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Device(ctx context.Context, siteID, deviceID string) (*Device, error) {
	var out Device
	path := "sites/" + url.PathEscape(siteID) + "/devices/" + url.PathEscape(deviceID)
	if err := a.c.GetJSON(ctx, "network", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Clients(ctx context.Context, siteID string, offset, limit int) (*Page[Client], error) {
	var out Page[Client]
	path := "sites/" + url.PathEscape(siteID) + "/clients"
	if err := a.c.GetJSON(ctx, "network", path, pageQuery(offset, limit), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) Client(ctx context.Context, siteID, clientID string) (*Client, error) {
	var out Client
	path := "sites/" + url.PathEscape(siteID) + "/clients/" + url.PathEscape(clientID)
	if err := a.c.GetJSON(ctx, "network", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) DeviceStatistics(ctx context.Context, siteID, deviceID string) (*DeviceStatistics, error) {
	var out DeviceStatistics
	path := "sites/" + url.PathEscape(siteID) + "/devices/" + url.PathEscape(deviceID) + "/statistics/latest"
	if err := a.c.GetJSON(ctx, "network", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *API) RestartDevice(ctx context.Context, siteID, deviceID string) error {
	path := "sites/" + url.PathEscape(siteID) + "/devices/" + url.PathEscape(deviceID) + "/actions"
	return a.c.PostJSON(ctx, "network", path, actionRequest{Action: "RESTART"}, nil)
}

func (a *API) PowerCyclePort(ctx context.Context, siteID, deviceID string, portIdx int) error {
	path := "sites/" + url.PathEscape(siteID) + "/devices/" + url.PathEscape(deviceID) +
		"/interfaces/ports/" + url.PathEscape(strconv.Itoa(portIdx)) + "/actions"
	return a.c.PostJSON(ctx, "network", path, actionRequest{Action: "POWER_CYCLE"}, nil)
}

func (a *API) AuthorizeGuest(ctx context.Context, siteID, clientID string) error {
	path := "sites/" + url.PathEscape(siteID) + "/clients/" + url.PathEscape(clientID) + "/actions"
	return a.c.PostJSON(ctx, "network", path, actionRequest{Action: "AUTHORIZE_GUEST_ACCESS"}, nil)
}

func (a *API) UnauthorizeGuest(ctx context.Context, siteID, clientID string) error {
	path := "sites/" + url.PathEscape(siteID) + "/clients/" + url.PathEscape(clientID) + "/actions"
	return a.c.PostJSON(ctx, "network", path, actionRequest{Action: "UNAUTHORIZE_GUEST_ACCESS"}, nil)
}

func (a *API) KickClient(ctx context.Context, siteID, clientID string) error {
	path := "sites/" + url.PathEscape(siteID) + "/clients/" + url.PathEscape(clientID) + "/actions"
	return a.c.PostJSON(ctx, "network", path, actionRequest{Action: "KICK"}, nil)
}

func pageQuery(offset, limit int) url.Values {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	q := url.Values{}
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(limit))
	return q
}
