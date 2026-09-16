package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetrics_RecordsByRoutePatternAndGuardsWithToken(t *testing.T) {
	r := gin.New()
	r.Use(Metrics())
	r.GET("/v1/things/:id", func(c *gin.Context) { c.Status(200) })
	r.GET("/metrics", MetricsHandler("s3cret"))

	for _, id := range []string{"a", "b"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/things/"+id, nil))
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 401 {
		t.Fatalf("no token: %d, want 401", w.Code)
	}

	w = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("with token: %d", w.Code)
	}
	body := w.Body.String()
	// Two hits collapse onto the pattern, not two raw paths.
	if !strings.Contains(body, `http_requests_total{method="GET",route="/v1/things/:id",status="200"} 2`) {
		t.Errorf("expected pattern-labelled counter with value 2, got:\n%s", body)
	}
	if strings.Contains(body, "/v1/things/a") {
		t.Error("raw path leaked into a label")
	}
	if !strings.Contains(body, `http_request_duration_seconds_count{method="GET",route="/v1/things/:id"} 2`) {
		t.Error("histogram not recorded per route")
	}
}
