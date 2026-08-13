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
		_ = json.NewEncoder(w).Encode(Camera{ID: "cam-1", Name: "vestibule", State: "CONNECTED"})
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
}
