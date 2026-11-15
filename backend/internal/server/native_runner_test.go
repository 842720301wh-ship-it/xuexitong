package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNativeRunnerAuthorizationAndLifecycle(t *testing.T) {
	s, task, _ := taskFixture(t)
	s.NativeToken = "native-secret"
	h := s.Handler(t.TempDir())
	saved := readTask(t, request(h, "POST", "/api/tasks", taskBody(t, task)))
	path := "/api/native/tasks/" + saved.ID
	if request(h, "POST", path+"/claim", `{}`).Code != 403 {
		t.Fatal("WebView can obtain credentials")
	}
	native := func(suffix, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1"+path+suffix, strings.NewReader(body))
		r.Header.Set("X-Tiantian-Native", "native-secret")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	claim := native("/claim", `{}`)
	if claim.Code != 200 || !strings.Contains(claim.Body.String(), "private-password") {
		t.Fatal(claim.Body.String())
	}
	var result struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(claim.Body.Bytes(), &result)
	if result.RunID == "" || native("/claim", `{}`).Code == 200 {
		t.Fatal("duplicate execution accepted")
	}
	if request(h, "DELETE", "/api/tasks/"+saved.ID, "").Code == 200 {
		t.Fatal("running task deleted")
	}
	if native("/progress", `{"run_id":"stale","event":{"state":"running"}}`).Code == 200 {
		t.Fatal("stale worker accepted")
	}
	progress := `{"run_id":"` + result.RunID + `","event":{"state":"cancelled","message":"已停止"}}`
	if native("/progress", progress).Code != 200 {
		t.Fatal("terminal update rejected")
	}
	records := request(h, "GET", "/api/records", "").Body.String()
	if !strings.Contains(records, "已停止") || strings.Contains(records, "private-password") || strings.Contains(records, "private-model-key") {
		t.Fatal("invalid public records")
	}
	if native("/progress", progress).Code == 200 {
		t.Fatal("closed worker accepted")
	}
	if native("/claim", `{}`).Code != 200 {
		t.Fatal("task cannot restart after stop")
	}
}
