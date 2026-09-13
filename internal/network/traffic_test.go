package network

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func warsaw(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func ptrf(v float64) *float64 { return &v }

func synBucket(res string, start time.Time, ident string, rx, tx float64) trafficBucket {
	end := start.Add(resolutionDuration(res))
	if res == trafficResDay {
		if day, ok := localCalendarDay(start, start.Location()); ok {
			end = day.End
		}
	}
	return trafficBucket{
		Start: start, End: end, Resolution: res, Identity: ident,
		Download: ptrf(rx), Upload: ptrf(tx),
	}
}

func TestMonthToDateIncludesTodayCompletedIntervals(t *testing.T) {
	loc := warsaw(t)
	observed := time.Date(2026, 9, 13, 1, 22, 0, 0, loc)
	start, end := MonthToDateWindow(observed, loc)
	if !start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("start=%s", start)
	}
	q := TrafficQuery{Start: start, End: end, Location: loc, Timezone: "Europe/Warsaw", ObservedAt: observed}

	var buckets []trafficBucket
	for d := 1; d <= 13; d++ {
		day := time.Date(2026, 9, d, 0, 0, 0, 0, loc)
		buckets = append(buckets, synBucket(trafficResDay, day, "", 100, 10))
	}
	// overlapping hourly for complete days must not be counted
	for h := 0; h < 24; h++ {
		buckets = append(buckets, synBucket(trafficResHour, time.Date(2026, 9, 12, h, 0, 0, 0, loc), "", 999, 999))
	}
	buckets = append(buckets, synBucket(trafficResHour, time.Date(2026, 9, 13, 0, 0, 0, 0, loc), "", 7, 3))
	buckets = append(buckets, synBucket(trafficResHour, time.Date(2026, 9, 13, 1, 0, 0, 0, loc), "", 500, 500)) // incomplete
	for m := 0; m < 25; m += 5 {
		buckets = append(buckets, synBucket(trafficRes5Min, time.Date(2026, 9, 13, 1, m, 0, 0, loc), "", 1, 1))
	}

	sel := selectTrafficBuckets(buckets, q)
	totals, unknown := sumTrafficBuckets(sel.selected)
	if unknown != 0 {
		t.Fatalf("unknown=%d", unknown)
	}
	if totals == nil {
		t.Fatal("totals")
	}
	// 12 complete days * 100 + today 00:00 hour 7 + four completed 5min * 1 (01:00,05,10,15)
	if totals.DownloadBytes != 12*100+7+4 {
		t.Fatalf("download=%d selected=%d res=%v", totals.DownloadBytes, len(sel.selected), sel.resolutionsUsed)
	}
	cover := buildCoverage(q, sel, unknown)
	if cover.Status == "empty" {
		t.Fatalf("%+v", cover)
	}
	if cover.Pending == nil {
		t.Fatal("expected pending current interval")
	}
	if got := stringsJoin(sel.resolutionsUsed); got != "daily,hourly,5minutes" {
		t.Fatalf("resolutions=%s", got)
	}
}

func stringsJoin(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func TestOverlappingBucketsPreferCoarser(t *testing.T) {
	loc := warsaw(t)
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, loc)
	q := TrafficQuery{
		Start: day, End: day.AddDate(0, 0, 1), Location: loc, Timezone: "Europe/Warsaw",
		ObservedAt: day.AddDate(0, 0, 1),
	}
	var buckets []trafficBucket
	buckets = append(buckets, synBucket(trafficResDay, day, "", 50, 5))
	for h := 0; h < 24; h++ {
		buckets = append(buckets, synBucket(trafficResHour, day.Add(time.Duration(h)*time.Hour), "", 1000, 1000))
	}
	sel := selectTrafficBuckets(buckets, q)
	totals, _ := sumTrafficBuckets(sel.selected)
	if totals.DownloadBytes != 50 {
		t.Fatalf("got %d want daily 50 not hourly overlap", totals.DownloadBytes)
	}
}

func TestDSTSpringUsesHourlyNotDaily(t *testing.T) {
	loc := warsaw(t)
	day := time.Date(2026, 3, 29, 0, 0, 0, 0, loc)
	next := day.AddDate(0, 0, 1)
	if next.Sub(day) == 24*time.Hour {
		t.Fatal("expected 23h spring-forward day")
	}
	q := TrafficQuery{Start: day, End: next, Location: loc, Timezone: "Europe/Warsaw", ObservedAt: next}
	var buckets []trafficBucket
	buckets = append(buckets, synBucket(trafficResDay, day, "", 9999, 9999))
	cur := day
	nHour := 0
	for cur.Before(next) {
		buckets = append(buckets, synBucket(trafficResHour, cur, "", 2, 1))
		cur = cur.Add(time.Hour)
		nHour++
	}
	sel := selectTrafficBuckets(buckets, q)
	for _, b := range sel.selected {
		if b.Resolution == trafficResDay {
			t.Fatal("daily should be skipped on DST")
		}
	}
	totals, _ := sumTrafficBuckets(sel.selected)
	if totals.DownloadBytes != int64(2*nHour) {
		t.Fatalf("download=%d hours=%d", totals.DownloadBytes, nHour)
	}
}

func TestDSTFallUsesHourlyNotDaily(t *testing.T) {
	loc := warsaw(t)
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	next := day.AddDate(0, 0, 1)
	if next.Sub(day) != 25*time.Hour {
		t.Fatalf("expected 25h fall-back day, got %s", next.Sub(day))
	}
	q := TrafficQuery{Start: day, End: next, Location: loc, Timezone: "Europe/Warsaw", ObservedAt: next}
	var buckets []trafficBucket
	buckets = append(buckets, synBucket(trafficResDay, day, "", 1, 1))
	cur := day
	nHour := 0
	for cur.Before(next) {
		buckets = append(buckets, synBucket(trafficResHour, cur, "", 3, 0))
		cur = cur.Add(time.Hour)
		nHour++
	}
	sel := selectTrafficBuckets(buckets, q)
	totals, _ := sumTrafficBuckets(sel.selected)
	if nHour != 25 || totals.DownloadBytes != 75 {
		t.Fatalf("hours=%d download=%d", nHour, totals.DownloadBytes)
	}
}

func TestGapsAreNotZeroFilled(t *testing.T) {
	loc := warsaw(t)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 9, 4, 0, 0, 0, 0, loc)
	q := TrafficQuery{Start: start, End: end, Location: loc, Timezone: "Europe/Warsaw", ObservedAt: end}
	buckets := []trafficBucket{
		synBucket(trafficResDay, start, "", 10, 1),
		synBucket(trafficResDay, start.AddDate(0, 0, 2), "", 20, 2),
	}
	sel := selectTrafficBuckets(buckets, q)
	totals, _ := sumTrafficBuckets(sel.selected)
	if totals.DownloadBytes != 30 {
		t.Fatalf("download=%d", totals.DownloadBytes)
	}
	cover := buildCoverage(q, sel, 0)
	if cover.Status != "partial" || cover.GapMS == 0 || len(cover.Gaps) == 0 {
		t.Fatalf("%+v", cover)
	}
	if totals.DownloadBytes == 0 {
		t.Fatal("gap must not become zero totals")
	}
}

func TestMissingCountersStayNil(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	b := synBucket(trafficResHour, start, "", 1, 1)
	b.Download = nil
	sel := selectTrafficBuckets([]trafficBucket{b, synBucket(trafficResHour, start.Add(time.Hour), "", 8, 2)}, TrafficQuery{
		Start: start, End: start.Add(2 * time.Hour), Location: loc, Timezone: "UTC", ObservedAt: start.Add(2 * time.Hour),
	})
	totals, unknown := sumTrafficBuckets(sel.selected)
	if unknown == 0 || totals == nil || totals.DownloadBytes != 8 {
		t.Fatalf("totals=%+v unknown=%d", totals, unknown)
	}
}

func TestOfflineClientsRemainInRanking(t *testing.T) {
	byMAC := map[string]*clientAgg{
		"aa:00:00:00:00:01": {MAC: "aa:00:00:00:00:01", Download: 100, Upload: 1},
		"aa:00:00:00:00:02": {MAC: "aa:00:00:00:00:02", Download: 50, Upload: 50},
	}
	names := map[string]trafficClientMeta{
		"aa:00:00:00:00:01": {ID: "id-offline", Name: "nas", Hostname: "nas.lab"},
		"aa:00:00:00:00:02": {ID: "id-online", Name: "laptop"},
	}
	connected := map[string]struct{}{"aa:00:00:00:00:02": {}}
	rows := rankClients(byMAC, names, connected, true, trafficSortTotal)
	if len(rows) != 2 {
		t.Fatalf("%d", len(rows))
	}
	if rows[0].MAC != "aa:00:00:00:00:01" || rows[0].Connected == nil || *rows[0].Connected {
		t.Fatalf("offline winner %+v", rows[0])
	}
	if rows[1].Connected == nil || !*rows[1].Connected {
		t.Fatalf("online %+v", rows[1])
	}
}

func TestRankingSortAndPagination(t *testing.T) {
	byMAC := map[string]*clientAgg{}
	for i := 1; i <= 5; i++ {
		mac := "aa:00:00:00:00:0" + string(rune('0'+i))
		byMAC[mac] = &clientAgg{MAC: mac, Download: float64(i * 10), Upload: float64(6 - i)}
	}
	rows := rankClients(byMAC, nil, nil, false, trafficSortDownload)
	if rows[0].DownloadBytes != 50 || rows[4].DownloadBytes != 10 {
		t.Fatalf("%+v", rows)
	}
	page, ranking := pageClients(rows, TrafficQuery{Sort: trafficSortDownload, Offset: 2, Top: 2})
	if !ranking.Partial || ranking.RankedClients != 5 || ranking.Returned != 2 {
		t.Fatalf("%+v", ranking)
	}
	if page[0].DownloadBytes != 30 {
		t.Fatalf("%+v", page)
	}
	up := rankClients(byMAC, nil, nil, false, trafficSortUpload)
	if up[0].UploadBytes != 5 {
		t.Fatalf("%+v", up[0])
	}
}

func TestResolutionQueryWindowNarrowsFineGrained(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, 9, 13, 1, 22, 0, 0, loc)
	q := TrafficQuery{Start: start, End: end, ObservedAt: end, Location: loc}
	d0, d1 := resolutionQueryWindow(trafficResDay, q)
	if d0 != start.UnixMilli() || d1 != end.UnixMilli() {
		t.Fatalf("daily window %d %d", d0, d1)
	}
	h0, _ := resolutionQueryWindow(trafficResHour, q)
	if h0 != end.Add(-8*24*time.Hour).UnixMilli() {
		t.Fatalf("hourly start %d", h0)
	}
	m0, _ := resolutionQueryWindow(trafficRes5Min, q)
	if m0 != end.Add(-24*time.Hour).UnixMilli() {
		t.Fatalf("5min start %d", m0)
	}
}

func TestValidateRangeAndParseTime(t *testing.T) {
	loc := warsaw(t)
	start, err := ParseTrafficTime("2026-09-01", loc, false)
	if err != nil || !start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("%v %s", err, start)
	}
	end, err := ParseTrafficTime("2026-09-13", loc, true)
	if err != nil || !end.Equal(time.Date(2026, 9, 14, 0, 0, 0, 0, loc)) {
		t.Fatalf("%v %s", err, end)
	}
	if err := ValidateTrafficRange(start, end, loc); err != nil {
		t.Fatal(err)
	}
	tooLongStart := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
	if err := ValidateTrafficRange(tooLongStart, end, loc); err == nil {
		t.Fatal("expected range error")
	}
	if _, err := ParseTrafficTime("not-a-time", loc, false); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAggregateBeforeRounding(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	buckets := []trafficBucket{
		synBucket(trafficResHour, start, "", 0.4, 0),
		synBucket(trafficResHour, start.Add(time.Hour), "", 0.4, 0),
		synBucket(trafficResHour, start.Add(2*time.Hour), "", 0.4, 0),
	}
	sel := selectTrafficBuckets(buckets, TrafficQuery{
		Start: start, End: start.Add(3 * time.Hour), Location: loc, Timezone: "UTC", ObservedAt: start.Add(3 * time.Hour),
	})
	totals, _ := sumTrafficBuckets(sel.selected)
	if totals.DownloadBytes != 1 {
		t.Fatalf("rounded after sum, got %d", totals.DownloadBytes)
	}
}

func TestSchemaFixtureSyntheticOnly(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "traffic_wan_v1.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if containsRealish(string(raw)) {
		t.Fatalf("fixture leaked a real identity")
	}
	var wan WANTrafficReport
	if err := json.Unmarshal(raw, &wan); err != nil {
		t.Fatal(err)
	}
	if wan.Schema != TrafficSchemaWAN || wan.Scope != trafficScopeWAN || wan.Totals == nil {
		t.Fatalf("%+v", wan)
	}
	clients, err := os.ReadFile(filepath.Join("testdata", "traffic_clients_v1.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if containsRealish(string(clients)) {
		t.Fatal("client fixture leaked a real identity")
	}
	var crep ClientTrafficReport
	if err := json.Unmarshal(clients, &crep); err != nil {
		t.Fatal(err)
	}
	if crep.Schema != TrafficSchemaClients || len(crep.Clients) == 0 || crep.Clients[0].MAC != "aa:00:00:00:00:01" {
		t.Fatalf("%+v", crep)
	}
	internet, err := os.ReadFile(filepath.Join("testdata", "traffic_internet_v1.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if containsRealish(string(internet)) {
		t.Fatal("internet fixture leaked a real identity")
	}
	var irep InternetTrafficReport
	if err := json.Unmarshal(internet, &irep); err != nil {
		t.Fatal(err)
	}
	if irep.Schema != TrafficSchemaInternet || irep.Scope != trafficScopeInternet || irep.Totals == nil {
		t.Fatalf("%+v", irep)
	}
	if irep.Unidentified == nil || irep.Unidentified.TotalBytes == irep.Totals.TotalBytes {
		t.Fatalf("unidentified must not equal site totals: %+v", irep)
	}
}

func containsRealish(s string) bool {
	needles := []string{"leska", "dukielska", "192.168.", "00:06:18"}
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
