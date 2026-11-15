package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tiantianxiangshang/internal/settings"
)

func TestModelProtocols(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			s, h, _ := setup(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := map[string]string{"chat": "/proxy/v1/chat/completions", "responses": "/proxy/v1/responses", "anthropic": "/proxy/v1/messages"}[protocol]
				if r.URL.Path != path {
					t.Errorf("wrong path: %s", r.URL.Path)
				}
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != "test-model" {
					t.Error("invalid request")
				}
				if protocol == "anthropic" {
					if r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") != "2023-06-01" || body["max_tokens"] == nil {
						t.Error("missing Anthropic auth/parameters")
					}
				} else if r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("missing bearer auth")
				}
				if protocol == "responses" {
					if body["input"] == nil || body["messages"] != nil || body["max_tokens"] != nil {
						t.Error("chat parameters leaked into Responses")
					}
					w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}`))
				} else if protocol == "anthropic" {
					w.Write([]byte(`{"content":[{"type":"text","text":"OK"}]}`))
				} else {
					if body["messages"] == nil || body["max_tokens"] != nil {
						t.Error("invalid chat body")
					}
					w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
				}
			}))
			defer upstream.Close()
			if e := s.Store.Update(func(c *settings.Config) error {
				c.Models = []settings.Model{{ID: "test", Endpoint: upstream.URL + "/proxy/v1", Protocol: protocol, APIKey: "secret", Primary: "test-model"}}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
			result := request(h, "POST", "/api/models/test/test", `{}`)
			if result.Code != 200 {
				t.Fatal(result.Body.String())
			}
		})
	}
}
func TestRejectInvalidProtocolReplies(t *testing.T) {
	for _, sample := range []struct{ protocol, body string }{
		{"responses", `{"status":"failed","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]}`},
		{"responses", `{"output":[{"type":"reasoning"}]}`},
		{"responses", `{"choices":[{"message":{"content":"wrong format"}}]}`},
		{"anthropic", `{"content":[]}`}, {"chat", `{"choices":[{"message":{"content":" "}}]}`},
		{"chat", `{"choices":[{"message":{"content":"","reasoning_content":"thinking only"}}]}`},
		{"chat", `{"error":{"message":"failed"},"choices":[{"message":{"content":"OK"}}]}`},
	} {
		if validModelReply(sample.protocol, []byte(sample.body)) {
			t.Errorf("accepted %s", sample.body)
		}
	}
}
func TestCatalogDiscoveryAndKeyIsolation(t *testing.T) {
	s, h, _ := setup(t)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/openai/v1/models" || r.Header.Get("Authorization") != "Bearer private-secret" {
			t.Error("wrong catalog request")
		}
		w.Write([]byte(`{"data":[{"id":" model-b "},{"id":"model-a"},{"id":"model-b"}]}`))
	}))
	defer upstream.Close()
	payload := func(endpoint, key string) string {
		b, _ := json.Marshal(map[string]string{"config_id": "saved", "endpoint": endpoint, "api_key": key, "protocol": "responses"})
		return string(b)
	}
	_ = s.Store.Update(func(c *settings.Config) error {
		c.Models = []settings.Model{{ID: "saved", Endpoint: upstream.URL + "/openai/v1", APIKey: "private-secret"}}
		return nil
	})
	result := request(h, "POST", "/api/models/discover", payload(upstream.URL+"/openai/v1/responses", ""))
	if result.Code != 200 || !strings.Contains(result.Body.String(), `["model-b","model-a"]`) || strings.Contains(result.Body.String(), "secret") {
		t.Fatal(result.Body.String())
	}
	if len(s.Store.Read().Models) != 1 || s.Store.Read().Models[0].Primary != "" {
		t.Fatal("discovery saved a draft")
	}
	result = request(h, "POST", "/api/models/discover", payload(upstream.URL+"/other", ""))
	if result.Code == 200 || calls != 1 {
		t.Fatal("saved key reused for changed endpoint")
	}
	for _, endpoint := range []string{"http://example.com/v1", "https://name:password@example.com/v1", "https://example.com/v1?key=secret"} {
		if request(h, "POST", "/api/models/discover", payload(endpoint, "private-secret")).Code == 200 {
			t.Fatal("accepted invalid endpoint")
		}
	}
}
func TestCatalogRejectsFailureAndEmptyList(t *testing.T) {
	for _, sample := range []struct {
		code int
		body string
	}{{401, `{"error":"private-secret"}`}, {200, `{"data":[]}`}, {200, `<html>not a catalog</html>`}, {302, `redirect`}} {
		t.Run(sample.body, func(t *testing.T) {
			_, h, _ := setup(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.com")
				w.WriteHeader(sample.code)
				w.Write([]byte(sample.body))
			}))
			defer upstream.Close()
			b, _ := json.Marshal(map[string]string{"endpoint": upstream.URL, "api_key": "private-secret"})
			result := request(h, "POST", "/api/models/discover", string(b))
			if result.Code == 200 || strings.Contains(result.Body.String(), "private-secret") {
				t.Fatal(result.Body.String())
			}
		})
	}
}
