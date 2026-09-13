package network

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type InternetClientRow struct {
	MAC           string `json:"mac"`
	Name          string `json:"name,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	Wired         *bool  `json:"wired"`
	DownloadBytes int64  `json:"download_bytes"`
	UploadBytes   int64  `json:"upload_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	AppCount      int    `json:"app_count"`
}

type InternetAppRow struct {
	Application   int    `json:"application"`
	Category      int    `json:"category"`
	Name          string `json:"name,omitempty"`
	Unidentified  bool   `json:"unidentified"`
	DownloadBytes int64  `json:"download_bytes"`
	UploadBytes   int64  `json:"upload_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	ClientCount   int    `json:"client_count,omitempty"`
}

type InternetTrafficReport struct {
	Schema              string              `json:"schema"`
	Backend             string              `json:"backend"`
	Source              TrafficSource       `json:"source"`
	Site                TrafficSite         `json:"site"`
	ObservedAt          string              `json:"observed_at"`
	Timezone            string              `json:"timezone"`
	Requested           TrafficInterval     `json:"requested_interval"`
	Covered             *TrafficInterval    `json:"covered_interval"`
	Units               TrafficUnits        `json:"units"`
	Scope               string              `json:"scope"`
	Sort                string              `json:"sort"`
	IncludeUnidentified bool                `json:"include_unidentified"`
	Totals              *TrafficTotals      `json:"totals"`
	Unidentified        *TrafficTotals      `json:"unidentified"`
	Coverage            TrafficCoverage     `json:"coverage"`
	Ranking             ClientRanking       `json:"ranking"`
	Clients             []InternetClientRow `json:"clients"`
	Apps                []InternetAppRow    `json:"apps"`
	Limitations         []string            `json:"limitations"`
}

type v2TrafficResponse struct {
	ClientUsageByApp []v2ClientUsage `json:"client_usage_by_app"`
	TotalUsageByApp  []v2AppUsage    `json:"total_usage_by_app"`
}

type v2ClientUsage struct {
	Client     v2TrafficClient `json:"client"`
	UsageByApp []v2AppUsage    `json:"usage_by_app"`
}

type v2TrafficClient struct {
	MAC      string `json:"mac"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IsWired  *bool  `json:"is_wired"`
}

type v2AppUsage struct {
	Application      int     `json:"application"`
	Category         int     `json:"category"`
	BytesReceived    float64 `json:"bytes_received"`
	BytesTransmitted float64 `json:"bytes_transmitted"`
	TotalBytes       float64 `json:"total_bytes"`
	ClientCount      int     `json:"client_count"`
}

func (a *API) InternetTraffic(ctx context.Context, site TrafficSite, q TrafficQuery) (*InternetTrafficReport, error) {
	q = normalizeTrafficQuery(q)
	includeUnidentified := !q.ExcludeUnidentified
	limitations := []string{
		"Internet totals and ranking use Traffic Identification (GET /proxy/network/v2/api/site/{site}/traffic), not LAN-inclusive user reports.",
		"bytes_received is Internet download; bytes_transmitted is Internet upload. Site totals are the sum of total_usage_by_app, not the Unidentified (application 65535) row alone.",
		"includeUnidentified=true matches the Network UI checkbox. Per-client application lists are aggregated into client totals.",
		"The controller returns one aggregate for the requested window; intra-window gaps are not reported.",
	}

	raw, err := a.fetchInternetTraffic(ctx, site.InternalReference, q.Start, q.End, includeUnidentified)
	if err != nil {
		return nil, err
	}

	apps, unidentified := internetApps(raw.TotalUsageByApp)
	clients := internetClients(raw.ClientUsageByApp)
	sortInternetClients(clients, q.Sort)
	page, ranking := pageInternetClients(clients, q)
	ranking.Note = "Ranking covers every client in Traffic Identification for the requested window, including clients that are currently offline."

	totals := sumInternetAppBytes(apps)
	clientSum := sumInternetClientBytes(clients)
	if totals != nil && clientSum != nil && (totals.DownloadBytes != clientSum.DownloadBytes || totals.UploadBytes != clientSum.UploadBytes) {
		limitations = append(limitations, "Site app totals and summed client totals differed; site totals use total_usage_by_app.")
	}

	hasData := len(apps) > 0 || len(clients) > 0
	cover := internetCoverage(q, hasData)
	rep := &InternetTrafficReport{
		Schema:              TrafficSchemaInternet,
		Backend:             trafficBackendLegacy,
		Source:              internetTrafficSource(q.AppVersion),
		Site:                site,
		ObservedAt:          formatTrafficTime(q.ObservedAt, q.Location),
		Timezone:            q.Timezone,
		Requested:           intervalJSON(q.Start, q.End, q.Location),
		Covered:             cover.Covered,
		Units:               trafficUnits(),
		Scope:               trafficScopeInternet,
		Sort:                q.Sort,
		IncludeUnidentified: includeUnidentified,
		Unidentified:        unidentified,
		Coverage:            cover,
		Ranking:             ranking,
		Clients:             page,
		Apps:                apps,
		Limitations:         limitations,
	}
	if hasData {
		rep.Totals = totals
	}
	if apps == nil {
		rep.Apps = []InternetAppRow{}
	}
	if page == nil {
		rep.Clients = []InternetClientRow{}
	}
	return rep, nil
}

func (a *API) fetchInternetTraffic(ctx context.Context, slug string, start, end time.Time, includeUnidentified bool) (v2TrafficResponse, error) {
	var out v2TrafficResponse
	if strings.TrimSpace(slug) == "" {
		return out, fmt.Errorf("missing site slug")
	}
	query := url.Values{}
	query.Set("start", strconv.FormatInt(start.UnixMilli(), 10))
	query.Set("end", strconv.FormatInt(end.UnixMilli(), 10))
	if includeUnidentified {
		query.Set("includeUnidentified", "true")
	}
	path := "/proxy/network/v2/api/site/" + url.PathEscape(slug) + "/traffic"
	if err := a.c.GetPathJSON(ctx, path, query, &out); err != nil {
		return out, err
	}
	return out, nil
}

func internetTrafficSource(version string) TrafficSource {
	return TrafficSource{
		Kind:               trafficSourceTrafficID,
		Backend:            trafficBackendLegacy,
		ApplicationVersion: version,
		Endpoints: []string{
			"GET /proxy/network/v2/api/site/{site}/traffic",
		},
		Attrs: []string{
			"bytes_received", "bytes_transmitted", "total_bytes", "application", "category",
		},
		CounterSemantics: "Internet/DPI Traffic Identification. Site totals are sum(total_usage_by_app). Application 65535 is Unidentified, not the grand total.",
	}
}

func internetApps(rows []v2AppUsage) ([]InternetAppRow, *TrafficTotals) {
	out := make([]InternetAppRow, 0, len(rows))
	var unidentified *TrafficTotals
	for _, row := range rows {
		item := InternetAppRow{
			Application:   row.Application,
			Category:      row.Category,
			Unidentified:  row.Application == trafficUnidentifiedApp || row.Category == trafficUnidentifiedCat,
			DownloadBytes: roundBytes(row.BytesReceived),
			UploadBytes:   roundBytes(row.BytesTransmitted),
			TotalBytes:    roundBytes(row.TotalBytes),
			ClientCount:   row.ClientCount,
		}
		if item.TotalBytes == 0 && (item.DownloadBytes != 0 || item.UploadBytes != 0) {
			item.TotalBytes = item.DownloadBytes + item.UploadBytes
		}
		if item.Unidentified {
			item.Name = "Unidentified"
			unidentified = &TrafficTotals{
				DownloadBytes: item.DownloadBytes,
				UploadBytes:   item.UploadBytes,
				TotalBytes:    item.TotalBytes,
			}
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalBytes == out[j].TotalBytes {
			return out[i].Application < out[j].Application
		}
		return out[i].TotalBytes > out[j].TotalBytes
	})
	return out, unidentified
}

func internetClients(rows []v2ClientUsage) []InternetClientRow {
	out := make([]InternetClientRow, 0, len(rows))
	for _, row := range rows {
		mac := strings.ToLower(strings.TrimSpace(row.Client.MAC))
		if mac == "" {
			continue
		}
		var rx, tx, tot float64
		for _, app := range row.UsageByApp {
			rx += app.BytesReceived
			tx += app.BytesTransmitted
			tot += app.TotalBytes
		}
		item := InternetClientRow{
			MAC:           mac,
			Name:          strings.TrimSpace(row.Client.Name),
			Hostname:      strings.TrimSpace(row.Client.Hostname),
			Wired:         row.Client.IsWired,
			DownloadBytes: roundBytes(rx),
			UploadBytes:   roundBytes(tx),
			TotalBytes:    roundBytes(tot),
			AppCount:      len(row.UsageByApp),
		}
		if item.TotalBytes == 0 && (item.DownloadBytes != 0 || item.UploadBytes != 0) {
			item.TotalBytes = item.DownloadBytes + item.UploadBytes
		}
		out = append(out, item)
	}
	return out
}

func sumInternetAppBytes(rows []InternetAppRow) *TrafficTotals {
	if len(rows) == 0 {
		return nil
	}
	var tot TrafficTotals
	for _, row := range rows {
		tot.DownloadBytes += row.DownloadBytes
		tot.UploadBytes += row.UploadBytes
		tot.TotalBytes += row.TotalBytes
	}
	return &tot
}

func sumInternetClientBytes(rows []InternetClientRow) *TrafficTotals {
	if len(rows) == 0 {
		return nil
	}
	var tot TrafficTotals
	for _, row := range rows {
		tot.DownloadBytes += row.DownloadBytes
		tot.UploadBytes += row.UploadBytes
		tot.TotalBytes += row.TotalBytes
	}
	return &tot
}

func sortInternetClients(rows []InternetClientRow, key string) {
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
		if av == bv {
			return a.MAC < b.MAC
		}
		return av > bv
	})
}

func pageInternetClients(rows []InternetClientRow, q TrafficQuery) ([]InternetClientRow, ClientRanking) {
	total := len(rows)
	offset := q.Offset
	if offset > total {
		offset = total
	}
	top := q.Top
	if top <= 0 {
		top = DefaultTrafficTop
	}
	if top > MaxTrafficTop && !q.All {
		top = MaxTrafficTop
	}
	if q.All && top > MaxTrafficRankedAll {
		top = MaxTrafficRankedAll
	}
	end := offset + top
	truncated := false
	if end > total {
		end = total
	}
	if q.All && total > MaxTrafficRankedAll {
		end = MaxTrafficRankedAll
		truncated = true
		if offset >= end {
			offset = end
		}
	}
	page := append([]InternetClientRow{}, rows[offset:end]...)
	if page == nil {
		page = []InternetClientRow{}
	}
	partial := offset > 0 || end < total || truncated
	return page, ClientRanking{
		Sort:          q.Sort,
		Partial:       partial,
		Bounded:       true,
		Truncated:     truncated,
		Top:           top,
		Offset:        offset,
		Returned:      len(page),
		RankedClients: total,
	}
}

func internetCoverage(q TrafficQuery, hasData bool) TrafficCoverage {
	req := intervalJSON(q.Start, q.End, q.Location)
	ms := q.End.Sub(q.Start).Milliseconds()
	if ms < 0 {
		ms = 0
	}
	cover := TrafficCoverage{
		Status:          "empty",
		Requested:       req,
		RequestedMS:     ms,
		ResolutionsUsed: []string{"traffic-identification"},
		Gaps:            []TrafficGap{},
	}
	if hasData {
		cover.Status = "complete"
		cover.Covered = &req
		cover.CoveredMS = ms
		cover.Completeness = 1
	}
	return cover
}
