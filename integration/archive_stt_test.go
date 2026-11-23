package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestArchiveSttService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/cus":
			w.Write([]byte(`[{"uid":"Unit0001","language":"he"},{"uid":"Unit0001","language":"ru"},{"uid":"Unit0002","language":"he"}]`))
		case r.URL.Path == "/cus/Unit0001/content" && r.URL.Query().Get("lang") == "he":
			w.Write([]byte("שלום עולם\n\nפסקה שנייה"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := NewArchiveSttService(srv.URL)

	units, err := s.ContentUnits()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]bool{"Unit0001": {"he": true, "ru": true}, "Unit0002": {"he": true}}
	if !reflect.DeepEqual(units, want) {
		t.Fatalf("got %v want %v", units, want)
	}

	text, err := s.Content("Unit0001", "he")
	if err != nil || text != "שלום עולם\n\nפסקה שנייה" {
		t.Fatalf("got %q, %v", text, err)
	}
	if _, err := s.Content("Unit0001", "en"); err == nil {
		t.Fatal("404 should be an error")
	}
}
