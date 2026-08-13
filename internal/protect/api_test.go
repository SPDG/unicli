package protect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SPDG/unicli/internal/client"
)

func TestCameras(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/protect/integration/v1/meta/info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Info{ApplicationVersion: "7.1.87"})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Camera{
			{ID: "cam-1", Name: "vestibule", State: "CONNECTED", ModelKey: "camera"},
		})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "cam-1", "hdrType": "off", "name": "vestibule"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "cam-1", "name": "vestibule", "state": "CONNECTED", "featureFlags": map[string]any{"hasHdr": true},
		})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/nvrs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "nvr-1", "armMode": map[string]any{"status": "armed"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "nvr-1", "modelKey": "nvr", "name": "lab-nvr", "armMode": map[string]any{"status": "disabled"}})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/liveviews", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "lv-1", "name": "Default", "modelKey": "liveview"}})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1/rtsps-stream", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"high": "rtsps://192.168.5.1:7441/secretToken?enableSrtp"})
	})
	mux.HandleFunc("/proxy/protect/integration/v1/cameras/cam-1/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "k", false)
	if err != nil {
		t.Fatal(err)
	}
	api := New(c)
	info, err := api.Info(context.Background())
	if err != nil || info.ApplicationVersion != "7.1.87" {
		t.Fatalf("%v %+v", err, info)
	}
	cams, err := api.Cameras(context.Background())
	if err != nil || len(cams) != 1 || cams[0].Name != "vestibule" {
		t.Fatalf("%v %+v", err, cams)
	}
	cam, err := api.Camera(context.Background(), "cam-1")
	if err != nil || cam.ID != "cam-1" {
		t.Fatalf("%v %+v", err, cam)
	}
	nvr, err := api.NVR(context.Background())
	if err != nil || nvr["name"] != "lab-nvr" {
		t.Fatalf("%v %+v", err, nvr)
	}
	lvs, err := api.Devices(context.Background(), "liveviews")
	if err != nil || DeviceName(lvs[0]) != "Default" {
		t.Fatalf("%v %+v", err, lvs)
	}
	snap, err := api.Snapshot(context.Background(), "cam-1")
	if err != nil || len(snap) < 3 || snap[0] != 0xff {
		t.Fatalf("%v %v", err, snap)
	}
	stream, err := api.RTSPS(context.Background(), "cam-1")
	if err != nil || stream["high"] == nil {
		t.Fatalf("%v %+v", err, stream)
	}
	if err := api.RestartCamera(context.Background(), "cam-1"); err != nil {
		t.Fatal(err)
	}
	patched, err := api.PatchCamera(context.Background(), "cam-1", map[string]any{"hdrType": "off"})
	if err != nil || patched["hdrType"] != "off" {
		t.Fatalf("%v %+v", err, patched)
	}
	armed, err := api.PatchNVR(context.Background(), map[string]any{"armMode": map[string]any{"status": "armed"}})
	if err != nil || armed["id"] != "nvr-1" {
		t.Fatalf("%v %+v", err, armed)
	}
}
