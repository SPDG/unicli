package network

import (
	"testing"
	"time"
)

func TestInternetAppsDoesNotTreatUnidentifiedAsSiteTotal(t *testing.T) {
	apps, unidentified := internetApps([]v2AppUsage{
		{Application: 65535, Category: 255, BytesReceived: 1000, BytesTransmitted: 100, TotalBytes: 1100, ClientCount: 2},
		{Application: 185, Category: 20, BytesReceived: 400, BytesTransmitted: 50, TotalBytes: 450, ClientCount: 1},
	})
	if unidentified == nil || unidentified.TotalBytes != 1100 {
		t.Fatalf("unidentified=%+v", unidentified)
	}
	if apps[0].Name != "Unidentified" || !apps[0].Unidentified {
		t.Fatalf("%+v", apps[0])
	}
	tot := sumInternetAppBytes(apps)
	if tot == nil || tot.DownloadBytes != 1400 || tot.UploadBytes != 150 || tot.TotalBytes != 1550 {
		t.Fatalf("site totals %+v", tot)
	}
	if tot.TotalBytes == unidentified.TotalBytes {
		t.Fatal("unidentified row must not be used as the site total")
	}
}

func TestInternetClientsRankByTotal(t *testing.T) {
	wired := true
	wifi := false
	rows := internetClients([]v2ClientUsage{
		{
			Client: v2TrafficClient{MAC: "AA:00:00:00:00:02", Name: "laptop", IsWired: &wifi},
			UsageByApp: []v2AppUsage{
				{Application: 65535, BytesReceived: 500, BytesTransmitted: 50, TotalBytes: 550},
			},
		},
		{
			Client: v2TrafficClient{MAC: "aa:00:00:00:00:01", Name: "nas", Hostname: "nas.lab", IsWired: &wired},
			UsageByApp: []v2AppUsage{
				{Application: 65535, BytesReceived: 700, BytesTransmitted: 70, TotalBytes: 770},
				{Application: 185, BytesReceived: 200, BytesTransmitted: 30, TotalBytes: 230},
			},
		},
	})
	sortInternetClients(rows, trafficSortTotal)
	if len(rows) != 2 || rows[0].MAC != "aa:00:00:00:00:01" || rows[0].TotalBytes != 1000 {
		t.Fatalf("%+v", rows)
	}
	if rows[0].Name != "nas" || rows[0].Hostname != "nas.lab" || rows[0].Wired == nil || !*rows[0].Wired {
		t.Fatalf("%+v", rows[0])
	}
	if rows[0].AppCount != 2 {
		t.Fatalf("apps=%d", rows[0].AppCount)
	}
}

func TestInternetEmptyTotalsStayNull(t *testing.T) {
	if sumInternetAppBytes(nil) != nil {
		t.Fatal("empty apps")
	}
	loc := warsaw(t)
	q := TrafficQuery{
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, loc),
		End:      time.Date(2026, 9, 2, 0, 0, 0, 0, loc),
		Location: loc,
		Timezone: "Europe/Warsaw",
	}
	cover := internetCoverage(q, false)
	if cover.Status != "empty" || cover.Covered != nil {
		t.Fatalf("%+v", cover)
	}
}
