package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/SPDG/unicli/internal/client"
)

const (
	TrafficSchemaWAN      = "unicli.network.traffic.wan/v1"
	TrafficSchemaClients  = "unicli.network.traffic.clients/v1"
	TrafficSchemaInternet = "unicli.network.traffic.internet/v1"

	MaxTrafficCalendarDays = 32
	DefaultTrafficTop      = 10
	MaxTrafficTop          = 100
	MaxTrafficRankedAll    = 200

	trafficScopeWAN        = "wan"
	trafficScopeClientAll  = "client-all-traffic"
	trafficScopeInternet   = "internet"
	trafficBackendLegacy   = "legacy-controller"
	trafficSourceKind      = "unifi-controller-report"
	trafficSourceTrafficID = "unifi-traffic-identification"
	trafficUnidentifiedApp = 65535
	trafficUnidentifiedCat = 255
	trafficSortDownload    = "download"
	trafficSortUpload      = "upload"
	trafficSortTotal       = "total"
	trafficRes5Min         = "5minutes"
	trafficResHour         = "hourly"
	trafficResDay          = "daily"
	trafficReportSite      = "site"
	trafficReportUser      = "user"
)

var (
	trafficWANAttrs = []string{
		"time", "wan-rx_bytes", "wan-tx_bytes", "wan2-rx_bytes", "wan2-tx_bytes",
	}
	trafficUserAttrs = []string{
		"time", "rx_bytes", "tx_bytes", "wired-rx_bytes", "wired-tx_bytes",
	}
	trafficResolutions = []string{trafficResDay, trafficResHour, trafficRes5Min}
)

// TrafficQuery is a live controller report window.
type TrafficQuery struct {
	Start               time.Time
	End                 time.Time
	Location            *time.Location
	Timezone            string
	ObservedAt          time.Time
	Sort                string
	Offset              int
	Top                 int
	All                 bool
	AppVersion          string
	ExcludeUnidentified bool
}

// TrafficSite is the selected Network site for a report.
type TrafficSite struct {
	ID                string `json:"id"`
	InternalReference string `json:"internal_reference"`
	Name              string `json:"name"`
}

type TrafficInterval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type TrafficSource struct {
	Kind               string   `json:"kind"`
	Backend            string   `json:"backend"`
	ApplicationVersion string   `json:"application_version,omitempty"`
	Endpoints          []string `json:"endpoints"`
	Attrs              []string `json:"attrs"`
	CounterSemantics   string   `json:"counter_semantics"`
}

type TrafficUnits struct {
	Bytes string `json:"bytes"`
	Note  string `json:"note"`
}

type TrafficTotals struct {
	DownloadBytes int64  `json:"download_bytes"`
	UploadBytes   int64  `json:"upload_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	Bound         string `json:"bound,omitempty"`
}

type TrafficGap struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Reason string `json:"reason"`
}

type TrafficCoverage struct {
	Status          string           `json:"status"`
	Requested       TrafficInterval  `json:"requested"`
	Covered         *TrafficInterval `json:"covered"`
	RequestedMS     int64            `json:"requested_ms"`
	CoveredMS       int64            `json:"covered_ms"`
	GapMS           int64            `json:"gap_ms"`
	PendingMS       int64            `json:"pending_ms"`
	Completeness    float64          `json:"completeness"`
	ResolutionsUsed []string         `json:"resolutions_used"`
	Gaps            []TrafficGap     `json:"gaps"`
	Pending         *TrafficGap      `json:"pending,omitempty"`
	UnknownBuckets  int              `json:"unknown_buckets"`
}

type WANTrafficReport struct {
	Schema      string           `json:"schema"`
	Backend     string           `json:"backend"`
	Source      TrafficSource    `json:"source"`
	Site        TrafficSite      `json:"site"`
	ObservedAt  string           `json:"observed_at"`
	Timezone    string           `json:"timezone"`
	Requested   TrafficInterval  `json:"requested_interval"`
	Covered     *TrafficInterval `json:"covered_interval"`
	Units       TrafficUnits     `json:"units"`
	Scope       string           `json:"scope"`
	Totals      *TrafficTotals   `json:"totals"`
	Coverage    TrafficCoverage  `json:"coverage"`
	Limitations []string         `json:"limitations"`
}

type ClientTrafficRow struct {
	MAC           string `json:"mac"`
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	Connected     *bool  `json:"connected"`
	DownloadBytes int64  `json:"download_bytes"`
	UploadBytes   int64  `json:"upload_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type ClientRanking struct {
	Sort          string `json:"sort"`
	Partial       bool   `json:"partial"`
	Bounded       bool   `json:"bounded"`
	Truncated     bool   `json:"truncated"`
	Top           int    `json:"top"`
	Offset        int    `json:"offset"`
	Returned      int    `json:"returned"`
	RankedClients int    `json:"ranked_clients"`
	Note          string `json:"note"`
}

type ClientTrafficReport struct {
	Schema      string             `json:"schema"`
	Backend     string             `json:"backend"`
	Source      TrafficSource      `json:"source"`
	Site        TrafficSite        `json:"site"`
	ObservedAt  string             `json:"observed_at"`
	Timezone    string             `json:"timezone"`
	Requested   TrafficInterval    `json:"requested_interval"`
	Covered     *TrafficInterval   `json:"covered_interval"`
	Units       TrafficUnits       `json:"units"`
	Scope       string             `json:"scope"`
	Sort        string             `json:"sort"`
	Totals      *TrafficTotals     `json:"totals"`
	Coverage    TrafficCoverage    `json:"coverage"`
	Ranking     ClientRanking      `json:"ranking"`
	Clients     []ClientTrafficRow `json:"clients"`
	Limitations []string           `json:"limitations"`
}

type TrafficErrorReport struct {
	Schema      string           `json:"schema"`
	Status      string           `json:"status"`
	Backend     string           `json:"backend"`
	Site        *TrafficSite     `json:"site,omitempty"`
	ObservedAt  string           `json:"observed_at,omitempty"`
	Timezone    string           `json:"timezone,omitempty"`
	Requested   *TrafficInterval `json:"requested_interval,omitempty"`
	Totals      any              `json:"totals"`
	Error       string           `json:"error"`
	Limitations []string         `json:"limitations"`
}

type trafficBucket struct {
	Start      time.Time
	End        time.Time
	Resolution string
	Identity   string
	Download   *float64
	Upload     *float64
}

type trafficClientMeta struct {
	ID       string
	Name     string
	Hostname string
}

func (a *API) WANTraffic(ctx context.Context, site TrafficSite, q TrafficQuery) (*WANTrafficReport, error) {
	q = normalizeTrafficQuery(q)
	limitations := []string{
		"WAN totals use controller site-report counters wan-rx_bytes/wan-tx_bytes (plus wan2-* when present).",
		"wan-rx_bytes is Internet download; wan-tx_bytes is Internet upload.",
		"Report POST queries are read-only; they are not configuration mutations.",
	}
	points, err := a.fetchResolutionReports(ctx, site.InternalReference, trafficReportSite, trafficWANAttrs, q)
	if err != nil {
		return nil, err
	}
	buckets := parseTrafficBuckets(points, q.Location, trafficReportSite)
	sel := selectTrafficBuckets(buckets, q)
	totals, unknown := sumTrafficBuckets(sel.selected)
	if unknown < sel.unknown {
		unknown = sel.unknown
	}
	cover := buildCoverage(q, sel, unknown)
	if cover.GapMS > 0 || unknown > 0 {
		limitations = append(limitations, "Totals omit missing intervals and buckets without WAN counters; they are a lower bound, not a filled-in zero.")
	}
	rep := &WANTrafficReport{
		Schema:      TrafficSchemaWAN,
		Backend:     trafficBackendLegacy,
		Source:      wanTrafficSource(q.AppVersion),
		Site:        site,
		ObservedAt:  formatTrafficTime(q.ObservedAt, q.Location),
		Timezone:    q.Timezone,
		Requested:   intervalJSON(q.Start, q.End, q.Location),
		Covered:     cover.Covered,
		Units:       trafficUnits(),
		Scope:       trafficScopeWAN,
		Coverage:    cover,
		Limitations: limitations,
	}
	if totals != nil {
		if cover.GapMS > 0 || unknown > 0 {
			totals.Bound = "lower"
		}
		rep.Totals = totals
	}
	return rep, nil
}

func (a *API) ClientTraffic(ctx context.Context, site TrafficSite, q TrafficQuery) (*ClientTrafficReport, error) {
	q = normalizeTrafficQuery(q)
	limitations := []string{
		"Client ranking uses historical user-report counters (rx_bytes/tx_bytes and wired-* when present), not the connected-client list and not lifetime/session totals.",
		"Those client counters are LAN-inclusive (all traffic observed for the client). They are not WAN/Internet usage.",
		"rx_bytes is download to the client; tx_bytes is upload from the client. Wired and wireless fields are summed when both are present.",
		"Currently offline clients are included when they appear in historical reports for the requested period.",
		"Report POST queries are read-only; they are not configuration mutations.",
	}
	points, err := a.fetchResolutionReports(ctx, site.InternalReference, trafficReportUser, trafficUserAttrs, q)
	if err != nil {
		return nil, err
	}
	buckets := parseTrafficBuckets(points, q.Location, trafficReportUser)
	sel := selectTrafficBuckets(buckets, q)
	byMAC := aggregateClients(sel.selected)
	names, nameErr := a.legacyUserIndex(ctx, site.InternalReference)
	if nameErr != nil {
		limitations = append(limitations, "Could not load rest/user identities; ranking uses report MAC addresses only.")
	}
	connected, connKnown, connErr := a.connectedMACSet(ctx, site.ID)
	if connErr != nil {
		limitations = append(limitations, "Could not load currently connected clients; connected is null rather than inferred as offline.")
	}
	ranked := rankClients(byMAC, names, connected, connKnown, q.Sort)
	page, ranking := pageClients(ranked, q)
	totals, unknown := sumTrafficBuckets(sel.selected)
	if unknown < sel.unknown {
		unknown = sel.unknown
	}
	cover := buildCoverage(q, sel, unknown)
	if cover.GapMS > 0 || unknown > 0 {
		limitations = append(limitations, "Client totals omit missing intervals and buckets without counters; they are a lower bound, not a filled-in zero.")
	}
	rep := &ClientTrafficReport{
		Schema:      TrafficSchemaClients,
		Backend:     trafficBackendLegacy,
		Source:      clientTrafficSource(q.AppVersion),
		Site:        site,
		ObservedAt:  formatTrafficTime(q.ObservedAt, q.Location),
		Timezone:    q.Timezone,
		Requested:   intervalJSON(q.Start, q.End, q.Location),
		Covered:     cover.Covered,
		Units:       trafficUnits(),
		Scope:       trafficScopeClientAll,
		Sort:        q.Sort,
		Coverage:    cover,
		Ranking:     ranking,
		Clients:     page,
		Limitations: limitations,
	}
	if totals != nil {
		if cover.GapMS > 0 || unknown > 0 {
			totals.Bound = "lower"
		}
		rep.Totals = totals
	}
	return rep, nil
}

func wanTrafficSource(version string) TrafficSource {
	return TrafficSource{
		Kind:               trafficSourceKind,
		Backend:            trafficBackendLegacy,
		ApplicationVersion: version,
		Endpoints: []string{
			"POST /proxy/network/api/s/{site}/stat/report/{interval}.site",
		},
		Attrs:            append([]string{}, trafficWANAttrs...),
		CounterSemantics: "WAN-only site counters (wan-rx_bytes download, wan-tx_bytes upload). Integration v1 has no historical traffic reports.",
	}
}

func clientTrafficSource(version string) TrafficSource {
	return TrafficSource{
		Kind:               trafficSourceKind,
		Backend:            trafficBackendLegacy,
		ApplicationVersion: version,
		Endpoints: []string{
			"POST /proxy/network/api/s/{site}/stat/report/{interval}.user",
		},
		Attrs:            append([]string{}, trafficUserAttrs...),
		CounterSemantics: "Client-interface historical counters (LAN-inclusive). Not WAN/Internet usage.",
	}
}

func trafficUnits() TrafficUnits {
	return TrafficUnits{
		Bytes: "B",
		Note:  "Integer bytes. Raw controller values may be fractional; they are summed as floats and rounded once after aggregation.",
	}
}

type fetchedReport struct {
	Resolution string
	Points     []map[string]any
}

func (a *API) fetchResolutionReports(ctx context.Context, slug, kind string, attrs []string, q TrafficQuery) ([]fetchedReport, error) {
	out := make([]fetchedReport, 0, len(trafficResolutions))
	var lastUnsupported error
	got := 0
	for _, res := range trafficResolutions {
		resStart, resEnd := resolutionQueryWindow(res, q)
		points, err := a.StatReport(ctx, slug, res, kind, resStart, resEnd, attrs)
		if err != nil {
			if IsTrafficPermission(err) {
				return nil, err
			}
			if IsTrafficUnsupported(err) {
				lastUnsupported = err
				continue
			}
			var ae client.APIError
			if errors.As(err, &ae) && (ae.Status == 401 || ae.Status == 403) {
				return nil, err
			}
			return nil, err
		}
		got++
		out = append(out, fetchedReport{Resolution: res, Points: points})
	}
	if got == 0 {
		if lastUnsupported != nil {
			return nil, lastUnsupported
		}
		return nil, fmt.Errorf("no traffic report resolutions returned data")
	}
	return out, nil
}

// StatReport POSTs a read-only controller stats query.
func (a *API) StatReport(ctx context.Context, siteSlug, interval, kind string, startMs, endMs int64, attrs []string) ([]map[string]any, error) {
	var env legacyEnvelope
	path := "/proxy/network/api/s/" + url.PathEscape(siteSlug) + "/stat/report/" + interval + "." + kind
	payload := map[string]any{
		"attrs": attrs,
		"start": startMs,
		"end":   endMs,
	}
	if err := a.c.PostPathJSON(ctx, path, payload, &env); err != nil {
		return nil, err
	}
	if err := env.check(); err != nil {
		return nil, err
	}
	return decodeLegacyList(env.Data)
}

func (a *API) legacyUserIndex(ctx context.Context, slug string) (map[string]trafficClientMeta, error) {
	items, err := a.LegacyList(ctx, slug, "user")
	if err != nil {
		return nil, err
	}
	out := make(map[string]trafficClientMeta, len(items))
	for _, item := range items {
		mac := strings.ToLower(strings.TrimSpace(anyStringVal(item["mac"])))
		if mac == "" {
			continue
		}
		out[mac] = trafficClientMeta{
			ID:       firstNonEmpty(anyStringVal(item["_id"]), anyStringVal(item["id"])),
			Name:     anyStringVal(item["name"]),
			Hostname: firstNonEmpty(anyStringVal(item["hostname"]), anyStringVal(item["display_name"])),
		}
	}
	return out, nil
}

func (a *API) connectedMACSet(ctx context.Context, siteID string) (map[string]struct{}, bool, error) {
	if strings.TrimSpace(siteID) == "" {
		return nil, false, fmt.Errorf("missing site id")
	}
	page, err := Collect(func(offset, limit int) (*Page[Client], error) {
		return a.Clients(ctx, siteID, offset, limit)
	}, 0, 0, true)
	if err != nil {
		return nil, false, err
	}
	out := make(map[string]struct{}, len(page.Data))
	for _, c := range page.Data {
		mac := strings.ToLower(strings.TrimSpace(c.MacAddress))
		if mac != "" {
			out[mac] = struct{}{}
		}
	}
	return out, true, nil
}

func normalizeTrafficQuery(q TrafficQuery) TrafficQuery {
	if q.Location == nil {
		q.Location = time.UTC
	}
	if q.Timezone == "" {
		q.Timezone = q.Location.String()
	}
	if q.ObservedAt.IsZero() {
		q.ObservedAt = time.Now().In(q.Location)
	} else {
		q.ObservedAt = q.ObservedAt.In(q.Location)
	}
	q.Start = q.Start.In(q.Location)
	q.End = q.End.In(q.Location)
	switch strings.ToLower(strings.TrimSpace(q.Sort)) {
	case trafficSortDownload, trafficSortUpload, trafficSortTotal:
		q.Sort = strings.ToLower(strings.TrimSpace(q.Sort))
	default:
		q.Sort = trafficSortTotal
	}
	if q.Top <= 0 {
		q.Top = DefaultTrafficTop
	}
	if q.All && q.Top < MaxTrafficRankedAll {
		q.Top = MaxTrafficRankedAll
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return q
}

func parseTrafficBuckets(reports []fetchedReport, loc *time.Location, kind string) []trafficBucket {
	var out []trafficBucket
	for _, rep := range reports {
		dur := resolutionDuration(rep.Resolution)
		if dur <= 0 {
			continue
		}
		for _, p := range rep.Points {
			startMs, ok := jsonInt64(p["time"])
			if !ok {
				continue
			}
			start := time.UnixMilli(startMs).In(loc)
			end := start.Add(dur)
			if rep.Resolution == trafficResDay {
				if day, ok := localCalendarDay(start, loc); ok {
					end = day.End
				} else {
					end = start.Add(24 * time.Hour)
				}
			}
			b := trafficBucket{
				Start:      start,
				End:        end,
				Resolution: rep.Resolution,
			}
			if kind == trafficReportUser {
				b.Identity = strings.ToLower(strings.TrimSpace(firstNonEmpty(anyStringVal(p["user"]), anyStringVal(p["oid"]), anyStringVal(p["mac"]))))
				if b.Identity == "" {
					continue
				}
				b.Download = mergeByteCounters(p, "rx_bytes", "wired-rx_bytes")
				b.Upload = mergeByteCounters(p, "tx_bytes", "wired-tx_bytes")
			} else {
				b.Download = mergeByteCounters(p, "wan-rx_bytes", "wan2-rx_bytes")
				b.Upload = mergeByteCounters(p, "wan-tx_bytes", "wan2-tx_bytes")
			}
			out = append(out, b)
		}
	}
	return out
}

func mergeByteCounters(m map[string]any, keys ...string) *float64 {
	var sum float64
	found := false
	for _, k := range keys {
		v, ok := jsonFloat(m[k])
		if !ok {
			continue
		}
		found = true
		sum += v
	}
	if !found {
		return nil
	}
	return &sum
}

func jsonFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func jsonInt64(v any) (int64, bool) {
	f, ok := jsonFloat(v)
	if !ok {
		return 0, false
	}
	return int64(f), true
}

func anyStringVal(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprint(v)
}

func roundBytes(v float64) int64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v))
}

func NewTrafficError(schema, status, message string, site *TrafficSite, q *TrafficQuery, limitations []string) TrafficErrorReport {
	rep := TrafficErrorReport{
		Schema:      schema,
		Status:      status,
		Backend:     trafficBackendLegacy,
		Site:        site,
		Totals:      nil,
		Error:       message,
		Limitations: limitations,
	}
	if q != nil {
		rep.ObservedAt = formatTrafficTime(q.ObservedAt, q.Location)
		rep.Timezone = q.Timezone
		iv := intervalJSON(q.Start, q.End, q.Location)
		rep.Requested = &iv
	}
	return rep
}

func IsTrafficPermission(err error) bool {
	var ae client.APIError
	return errors.As(err, &ae) && ae.Status == 403
}

func IsTrafficUnsupported(err error) bool {
	var unavailable client.AppUnavailableError
	if errors.As(err, &unavailable) {
		return true
	}
	var ae client.APIError
	return errors.As(err, &ae) && (ae.Status == 404 || ae.Status == 400)
}

func sortClients(rows []ClientTrafficRow, key string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		var av, bv int64
		switch key {
		case trafficSortDownload:
			av, bv = a.DownloadBytes, b.DownloadBytes
		case trafficSortUpload:
			av, bv = a.UploadBytes, b.UploadBytes
		default:
			av, bv = a.TotalBytes, b.TotalBytes
		}
		if av != bv {
			return av > bv
		}
		return a.MAC < b.MAC
	})
}
