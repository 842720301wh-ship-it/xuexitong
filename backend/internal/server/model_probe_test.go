package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tiantianxiangshang/internal/settings"
)

func TestProtocolProbeDoesNotMutateSavedConfiguration(t *testing.T) {
	s, h, _ := setup(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/responses" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"model does not support Responses; private-key token=private-token"}}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer private-key" {
			t.Error("wrong chat probe")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer upstream.Close()
	_ = s.Store.Update(func(c *settings.Config) error {
		c.Models = []settings.Model{{ID: "saved", Endpoint: upstream.URL + "/v1", Protocol: "responses", APIKey: "private-key", Primary: "model", Models: []string{"model"}}}
		return nil
	})
	bad := request(h, "POST", "/api/models/saved/test", `{}`)
	if bad.Code == 200 || !strings.Contains(bad.Body.String(), "does not support Responses") || strings.Contains(bad.Body.String(), "private-key") || strings.Contains(bad.Body.String(), "private-token") {
		t.Fatal(bad.Body.String())
	}
	good := request(h, "POST", "/api/models/saved/test", `{"protocol":"chat"}`)
	if good.Code != 200 {
		t.Fatal(good.Body.String())
	}
	if s.Store.Read().Models[0].Protocol != "responses" {
		t.Fatal("probe changed saved configuration")
	}
	body, _ := json.Marshal(map[string]string{"config_id": "saved", "endpoint": upstream.URL + "/v1", "protocol": "chat", "primary": "model"})
	if r := request(h, "POST", "/api/models/probe", string(body)); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	body, _ = json.Marshal(map[string]string{"config_id": "saved", "endpoint": upstream.URL + "/different", "protocol": "chat", "primary": "model"})
	if request(h, "POST", "/api/models/probe", string(body)).Code == 200 {
		t.Fatal("key reused for a changed endpoint")
	}
}
