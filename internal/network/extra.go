package network

import (
	"context"
	"net/url"
)

func (a *API) DNSPolicies(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "dns/policies"), offset, limit)
}

func (a *API) DNSPolicy(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "dns/policies/"+url.PathEscape(id)))
}

func (a *API) CreateDNSPolicy(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "dns/policies"), body)
}

func (a *API) UpdateDNSPolicy(ctx context.Context, siteID, id string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "dns/policies/"+url.PathEscape(id)), body)
}

func (a *API) DeleteDNSPolicy(ctx context.Context, siteID, id string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "dns/policies/"+url.PathEscape(id)), force)
}

func (a *API) Vouchers(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "hotspot/vouchers"), offset, limit)
}

func (a *API) Voucher(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "hotspot/vouchers/"+url.PathEscape(id)))
}

func (a *API) CreateVouchers(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "hotspot/vouchers"), body)
}

func (a *API) DeleteVoucher(ctx context.Context, siteID, id string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "hotspot/vouchers/"+url.PathEscape(id)), force)
}

func (a *API) TrafficMatchingLists(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "traffic-matching-lists"), offset, limit)
}

func (a *API) TrafficMatchingList(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "traffic-matching-lists/"+url.PathEscape(id)))
}

func (a *API) CreateTrafficMatchingList(ctx context.Context, siteID string, body any) (map[string]any, error) {
	return postObject(a, ctx, sitePath(siteID, "traffic-matching-lists"), body)
}

func (a *API) UpdateTrafficMatchingList(ctx context.Context, siteID, id string, body any) (map[string]any, error) {
	return putObject(a, ctx, sitePath(siteID, "traffic-matching-lists/"+url.PathEscape(id)), body)
}

func (a *API) DeleteTrafficMatchingList(ctx context.Context, siteID, id string, force bool) error {
	return deleteObject(a, ctx, sitePath(siteID, "traffic-matching-lists/"+url.PathEscape(id)), force)
}

func (a *API) WANs(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "wans"), offset, limit)
}

func (a *API) VPNServers(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "vpn/servers"), offset, limit)
}

func (a *API) VPNTunnels(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "vpn/site-to-site-tunnels"), offset, limit)
}

func (a *API) DeviceTags(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "device-tags"), offset, limit)
}

func (a *API) PendingDevices(ctx context.Context, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, "pending-devices", offset, limit)
}

func (a *API) RadiusProfiles(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "radius/profiles"), offset, limit)
}

func (a *API) SwitchStacks(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "switching/switch-stacks"), offset, limit)
}

func (a *API) SwitchStack(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "switching/switch-stacks/"+url.PathEscape(id)))
}

func (a *API) LAGs(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "switching/lags"), offset, limit)
}

func (a *API) LAG(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "switching/lags/"+url.PathEscape(id)))
}

func (a *API) MCLAGDomains(ctx context.Context, siteID string, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, sitePath(siteID, "switching/mc-lag-domains"), offset, limit)
}

func (a *API) MCLAGDomain(ctx context.Context, siteID, id string) (map[string]any, error) {
	return getObject(a, ctx, sitePath(siteID, "switching/mc-lag-domains/"+url.PathEscape(id)))
}

func (a *API) DPIApplications(ctx context.Context, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, "dpi/applications", offset, limit)
}

func (a *API) DPICategories(ctx context.Context, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, "dpi/categories", offset, limit)
}

func (a *API) Countries(ctx context.Context, offset, limit int) (*Page[map[string]any], error) {
	return getPage[map[string]any](a, ctx, "countries", offset, limit)
}
