package network

import (
	"sort"
	"time"
)

type trafficSelection struct {
	selected        []trafficBucket
	used            []timeInterval
	gaps            []timeInterval
	pending         *timeInterval
	resolutionsUsed []string
	unknown         int
}

type timeInterval struct {
	Start time.Time
	End   time.Time
}

func (i timeInterval) Duration() time.Duration {
	if !i.End.After(i.Start) {
		return 0
	}
	return i.End.Sub(i.Start)
}

func selectTrafficBuckets(buckets []trafficBucket, q TrafficQuery) trafficSelection {
	window := timeInterval{Start: q.Start, End: q.End}
	if q.ObservedAt.Before(window.End) {
		window.End = q.ObservedAt
	}
	complete := make([]trafficBucket, 0, len(buckets))
	for _, b := range buckets {
		if !b.End.After(b.Start) {
			continue
		}
		if !b.End.After(window.Start) || !b.Start.Before(window.End) {
			continue
		}
		if b.End.After(q.ObservedAt) {
			continue
		}
		complete = append(complete, b)
	}

	type slot struct {
		iv      timeInterval
		res     string
		buckets []trafficBucket
	}
	index := map[string]map[int64]*slot{}
	for _, b := range complete {
		byStart := index[b.Resolution]
		if byStart == nil {
			byStart = map[int64]*slot{}
			index[b.Resolution] = byStart
		}
		key := b.Start.UnixMilli()
		s := byStart[key]
		if s == nil {
			s = &slot{iv: timeInterval{Start: b.Start, End: b.End}, res: b.Resolution}
			byStart[key] = s
		}
		s.buckets = append(s.buckets, b)
	}

	used := []timeInterval{}
	selected := []trafficBucket{}
	unknown := 0

	consider := func(res string, requireFullDay bool) {
		byStart := index[res]
		if byStart == nil {
			return
		}
		starts := make([]int64, 0, len(byStart))
		for k := range byStart {
			starts = append(starts, k)
		}
		sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
		for _, k := range starts {
			s := byStart[k]
			iv := s.iv
			if requireFullDay && res == trafficResDay {
				day, ok := localCalendarDay(iv.Start, q.Location)
				if !ok || dstCalendarDay(day) {
					continue
				}
				if !day.Start.Equal(iv.Start) || !day.End.Equal(iv.End) {
					continue
				}
				iv = timeInterval{Start: day.Start, End: day.End}
			}
			if iv.Start.Before(window.Start) || iv.End.After(window.End) {
				continue
			}
			if overlaps(used, iv) {
				continue
			}
			for _, b := range s.buckets {
				if b.Download == nil || b.Upload == nil {
					unknown++
				}
				selected = append(selected, b)
			}
			used = append(used, iv)
		}
	}

	consider(trafficResDay, true)
	consider(trafficResHour, false)
	consider(trafficRes5Min, false)

	used = mergeIntervals(used)
	gaps, pending := remaining(window, used, q.ObservedAt)
	return trafficSelection{
		selected:        selected,
		used:            used,
		gaps:            gaps,
		pending:         pending,
		resolutionsUsed: uniqueResolutions(selected),
		unknown:         unknown,
	}
}

func overlaps(used []timeInterval, iv timeInterval) bool {
	for _, u := range used {
		if iv.Start.Before(u.End) && u.Start.Before(iv.End) && !iv.Start.Equal(u.End) && !u.Start.Equal(iv.End) {
			return true
		}
	}
	return false
}

func mergeIntervals(in []timeInterval) []timeInterval {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].Start.Before(in[j].Start) })
	out := []timeInterval{in[0]}
	for _, iv := range in[1:] {
		last := &out[len(out)-1]
		if !iv.Start.After(last.End) {
			if iv.End.After(last.End) {
				last.End = iv.End
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

func remaining(window timeInterval, used []timeInterval, observed time.Time) (gaps []timeInterval, pending *timeInterval) {
	used = mergeIntervals(used)
	uncovered := subtractIntervals(window, used)
	pendingCutoff := observed.Truncate(5 * time.Minute)
	if pendingCutoff.After(observed) {
		pendingCutoff = observed
	}
	for _, u := range uncovered {
		if !u.End.After(pendingCutoff) {
			gaps = append(gaps, u)
			continue
		}
		if u.Start.Before(pendingCutoff) {
			gaps = append(gaps, timeInterval{Start: u.Start, End: pendingCutoff})
			open := timeInterval{Start: pendingCutoff, End: u.End}
			if open.Duration() > 0 {
				pending = &open
			}
			continue
		}
		open := u
		pending = &open
	}
	return gaps, pending
}

func subtractIntervals(window timeInterval, used []timeInterval) []timeInterval {
	if window.Duration() <= 0 {
		return nil
	}
	cur := window.Start
	var out []timeInterval
	for _, u := range mergeIntervals(used) {
		if u.End.Before(cur) || u.End.Equal(cur) {
			continue
		}
		if u.Start.After(window.End) {
			break
		}
		if u.Start.After(cur) {
			end := u.Start
			if end.After(window.End) {
				end = window.End
			}
			if end.After(cur) {
				out = append(out, timeInterval{Start: cur, End: end})
			}
		}
		if u.End.After(cur) {
			cur = u.End
		}
		if !cur.Before(window.End) {
			return out
		}
	}
	if cur.Before(window.End) {
		out = append(out, timeInterval{Start: cur, End: window.End})
	}
	return out
}

func uniqueResolutions(buckets []trafficBucket) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, res := range trafficResolutions {
		for _, b := range buckets {
			if b.Resolution == res {
				if _, ok := seen[res]; !ok {
					seen[res] = struct{}{}
					out = append(out, res)
				}
				break
			}
		}
	}
	return out
}

func buildCoverage(q TrafficQuery, sel trafficSelection, unknown int) TrafficCoverage {
	window := timeInterval{Start: q.Start, End: q.End}
	if q.ObservedAt.Before(window.End) {
		window.End = q.ObservedAt
	}
	var coveredMS int64
	for _, u := range sel.used {
		coveredMS += u.Duration().Milliseconds()
	}
	var gapMS int64
	gaps := make([]TrafficGap, 0, len(sel.gaps))
	for _, g := range sel.gaps {
		gapMS += g.Duration().Milliseconds()
		gaps = append(gaps, TrafficGap{
			Start:  formatTrafficTime(g.Start, q.Location),
			End:    formatTrafficTime(g.End, q.Location),
			Reason: "missing-bucket",
		})
	}
	var pendingMS int64
	var pending *TrafficGap
	if sel.pending != nil {
		pendingMS = sel.pending.Duration().Milliseconds()
		pending = &TrafficGap{
			Start:  formatTrafficTime(sel.pending.Start, q.Location),
			End:    formatTrafficTime(sel.pending.End, q.Location),
			Reason: "current-interval-open",
		}
	}
	reqMS := window.Duration().Milliseconds()
	completeness := 0.0
	if reqMS > 0 {
		completeness = float64(coveredMS) / float64(reqMS)
	}
	status := "complete"
	switch {
	case coveredMS == 0:
		status = "empty"
	case gapMS > 0 || unknown > 0:
		status = "partial"
	}
	var covered *TrafficInterval
	if len(sel.used) > 0 {
		iv := intervalJSON(sel.used[0].Start, sel.used[len(sel.used)-1].End, q.Location)
		covered = &iv
	}
	if gaps == nil {
		gaps = []TrafficGap{}
	}
	usedRes := sel.resolutionsUsed
	if usedRes == nil {
		usedRes = []string{}
	}
	return TrafficCoverage{
		Status:          status,
		Requested:       intervalJSON(q.Start, q.End, q.Location),
		Covered:         covered,
		RequestedMS:     reqMS,
		CoveredMS:       coveredMS,
		GapMS:           gapMS,
		PendingMS:       pendingMS,
		Completeness:    completeness,
		ResolutionsUsed: usedRes,
		Gaps:            gaps,
		Pending:         pending,
		UnknownBuckets:  unknown,
	}
}

func sumTrafficBuckets(buckets []trafficBucket) (*TrafficTotals, int) {
	var down, up float64
	unknown := 0
	known := 0
	for _, b := range buckets {
		if b.Download == nil || b.Upload == nil {
			unknown++
			continue
		}
		down += *b.Download
		up += *b.Upload
		known++
	}
	if known == 0 {
		return nil, unknown
	}
	d := roundBytes(down)
	u := roundBytes(up)
	return &TrafficTotals{
		DownloadBytes: d,
		UploadBytes:   u,
		TotalBytes:    d + u,
	}, unknown
}

func aggregateClients(buckets []trafficBucket) map[string]*clientAgg {
	out := map[string]*clientAgg{}
	for _, b := range buckets {
		if b.Identity == "" {
			continue
		}
		if b.Download == nil || b.Upload == nil {
			continue
		}
		a := out[b.Identity]
		if a == nil {
			a = &clientAgg{MAC: b.Identity}
			out[b.Identity] = a
		}
		a.Download += *b.Download
		a.Upload += *b.Upload
	}
	return out
}

type clientAgg struct {
	MAC      string
	Download float64
	Upload   float64
}

func rankClients(byMAC map[string]*clientAgg, names map[string]trafficClientMeta, connected map[string]struct{}, connKnown bool, sortKey string) []ClientTrafficRow {
	rows := make([]ClientTrafficRow, 0, len(byMAC))
	for mac, a := range byMAC {
		d := roundBytes(a.Download)
		u := roundBytes(a.Upload)
		row := ClientTrafficRow{
			MAC:           mac,
			DownloadBytes: d,
			UploadBytes:   u,
			TotalBytes:    d + u,
		}
		if meta, ok := names[mac]; ok {
			row.ID = meta.ID
			row.Name = meta.Name
			row.Hostname = meta.Hostname
		}
		if connKnown {
			_, on := connected[mac]
			row.Connected = boolPtr(on)
		}
		rows = append(rows, row)
	}
	sortClients(rows, sortKey)
	return rows
}

func pageClients(rows []ClientTrafficRow, q TrafficQuery) ([]ClientTrafficRow, ClientRanking) {
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
	page := append([]ClientTrafficRow{}, rows[offset:end]...)
	if page == nil {
		page = []ClientTrafficRow{}
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
		Note:          "Ranking covers every client present in historical user reports for the selected windows, including clients that are currently offline.",
	}
}

func boolPtr(v bool) *bool { return &v }
