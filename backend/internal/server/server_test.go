package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"tiantianxiangshang/internal/learning"
	"tiantianxiangshang/internal/settings"
)

func TestEmbeddedWebAndAPIShareLoopbackOrigin(t *testing.T) {
	s, _, _ := setup(t)
	web := fstest.MapFS{
		"index.html":    {Data: []byte("<html>APK frontend</html>")},
		"assets/app.js": {Data: []byte("console.log('embedded')")},
		"api/settings":  {Data: []byte("must not shadow the API")},
	}
	service := httptest.NewServer(s.HandlerFS(web))
	defer service.Close()
	for path, expected := range map[string]string{
		"/":              "APK frontend",
		"/assets/app.js": "console.log('embedded')",
		"/api/settings":  `"default_account_id":""`,
	} {
		response, err := service.Client().Get(service.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), expected) {
			t.Fatalf("%s: status %d, body %q, error %v", path, response.StatusCode, body, err)
		}
	}
}

func setup(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	store, e := settings.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	s := New(store)
	s.Lookup = func(_ context.Context, u, p string) (learning.AccountData, error) {
		if p == "wrong" {
			return learning.AccountData{}, errors.New("登录失败")
		}
		return learning.AccountData{Name: "平台真实姓名", Courses: []learning.Course{{ID: "247370145", Name: "工程检测加固"}}}, nil
	}
	return s, s.Handler(t.TempDir()), dir
}
func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:18081"+path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func read(t *testing.T, w *httptest.ResponseRecorder) settings.Config {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var c settings.Config
	if e := json.Unmarshal(w.Body.Bytes(), &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func TestAccountsDefaultPersistenceAndRedaction(t *testing.T) {
	s, h, dir := setup(t)
	c := read(t, request(h, "POST", "/api/accounts", `{"name":"账号一","username":"13811112222","password":"secret-one"}`))
	a := c.Accounts[0].ID
	if c.Accounts[0].Name != "平台真实姓名" {
		t.Fatal("caller-supplied name used instead of platform profile")
	}
	if c.DefaultAccountID != a || c.Accounts[0].Status != "connected" {
		t.Fatal("first verified account must become default")
	}
	c = read(t, request(h, "POST", "/api/accounts", `{"name":"账号二","username":"13833334444","password":"secret-two"}`))
	b := c.Accounts[1].ID
	if c.DefaultAccountID != a {
		t.Fatal("adding second account changed default")
	}
	c = read(t, request(h, "PUT", "/api/accounts/default", `{"id":"`+b+`"}`))
	if c.DefaultAccountID != b {
		t.Fatal("switch failed")
	}
	defaults := request(h, "GET", "/api/task-defaults", "")
	if !strings.Contains(defaults.Body.String(), b) {
		t.Fatal("new tasks must use selected default")
	}
	reopened, e := settings.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if reopened.Read().DefaultAccountID != b {
		t.Fatal("default lost after restart")
	}
	if reopened.Read().Accounts[0].Password != "secret-one" {
		t.Fatal("credential lost after restart")
	}
	for _, path := range []string{"/api/settings"} {
		res := request(h, "GET", path, "")
		if strings.Contains(res.Body.String(), "secret-") || strings.Contains(res.Body.String(), `"password"`) {
			t.Fatal("credential exposed")
		}
	}
	disk, e := os.ReadFile(filepath.Join(dir, "settings.enc"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(disk, []byte("secret-one")) || bytes.Contains(disk, []byte("13811112222")) {
		t.Fatal("plaintext persisted")
	}
	failed := request(h, "POST", "/api/accounts", `{"username":"13855556666","password":"wrong"}`)
	if failed.Code != 400 || len(s.Store.Read().Accounts) != 2 {
		t.Fatal("failed login was saved")
	}
	c = read(t, request(h, "POST", "/api/accounts", `{"name":"已更新","username":"13811112222","password":"replacement"}`))
	if len(c.Accounts) != 2 || c.DefaultAccountID != b || s.Store.Read().Accounts[0].Password != "replacement" {
		t.Fatal("duplicate not updated correctly")
	}
	c = read(t, request(h, "DELETE", "/api/accounts/"+b, ""))
	if c.DefaultAccountID != a {
		t.Fatal("deleted default not replaced")
	}
	c = read(t, request(h, "DELETE", "/api/accounts/"+a, ""))
	if c.DefaultAccountID != "" || len(c.Accounts) != 0 {
		t.Fatal("last account not cleared")
	}
}
func TestCheckFailureClearsConnectedStatus(t *testing.T) {
	s, h, _ := setup(t)
	c := read(t, request(h, "POST", "/api/accounts", `{"username":"user1","password":"secret"}`))
	s.Lookup = func(context.Context, string, string) (learning.AccountData, error) {
		return learning.AccountData{}, errors.New("网络超时")
	}
	w := request(h, "POST", "/api/accounts/"+c.Accounts[0].ID+"/check", `{}`)
	if w.Code != 400 || s.Store.Read().Accounts[0].Status != "unknown" {
		t.Fatal("failed check left misleading connected state")
	}
}
func TestModelsCRUDAndRealProtocolTest(t *testing.T) {
	s, h, dir := setup(t)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("incorrect model request")
		}
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		if body["model"] != "chat" {
			t.Error("wrong primary model")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
	}))
	defer upstream.Close()
	body := `{"name":"测试供应商","endpoint":"` + upstream.URL + `/v1","api_key":"test-key","models":["chat","reasoner"],"primary":"chat","fallback":"reasoner"}`
	c := read(t, request(h, "POST", "/api/models", body))
	id := c.Models[0].ID
	c = read(t, request(h, "POST", "/api/models", strings.Replace(body, "测试供应商", "第二组配置", 1)))
	secondID := c.Models[1].ID
	if len(c.Models) != 2 || secondID == id {
		t.Fatal("multiple model configs were not saved independently")
	}
	if c.Models[0].APIKey != "" || !c.Models[0].HasKey {
		t.Fatal("key leaked or missing flag")
	}
	if w := request(h, "POST", "/api/models/"+id+"/test", `{}`); w.Code != 200 || calls != 1 {
		t.Fatalf("model test failed %s", w.Body.String())
	}
	c = read(t, request(h, "PUT", "/api/models/"+id, `{"name":"编辑后","endpoint":"`+upstream.URL+`/v1","api_key":"","models":["chat"],"primary":"chat","fallback":""}`))
	if c.Models[0].Name != "编辑后" || s.Store.Read().Models[0].APIKey != "test-key" {
		t.Fatal("editing did not retain key")
	}
	reopened, e := settings.Open(dir)
	if e != nil || reopened.Read().Models[0].APIKey != "test-key" {
		t.Fatal("model persistence failed")
	}
	if w := request(h, "POST", "/api/models", `{"name":"坏配置","endpoint":"https://example.com/v1","api_key":"key","models":["chat"],"primary":"missing"}`); w.Code != 400 {
		t.Fatal("invalid primary accepted")
	}
	c = read(t, request(h, "DELETE", "/api/models/"+id, ""))
	if len(c.Models) != 1 || c.Models[0].ID != secondID {
		t.Fatal("model not removed")
	}
	c = read(t, request(h, "DELETE", "/api/models/"+secondID, ""))
	if len(c.Models) != 0 {
		t.Fatal("last model not removed")
	}
}
func TestModelTestRejectsFakeSuccessAndRedirect(t *testing.T) {
	_, h, _ := setup(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"ok":true}`) }))
	defer upstream.Close()
	c := read(t, request(h, "POST", "/api/models", `{"name":"测试","endpoint":"`+upstream.URL+`","api_key":"key","models":["chat"],"primary":"chat"}`))
	if request(h, "POST", "/api/models/"+c.Models[0].ID+"/test", `{}`).Code != 400 {
		t.Fatal("arbitrary 200 response accepted")
	}
}
func TestRejectCrossOriginAndDNSRebinding(t *testing.T) {
	_, h, _ := setup(t)
	for _, tc := range []struct{ host, origin string }{{"evil.example", ""}, {"127.0.0.1:18081", "https://evil.example"}, {"127.0.0.1:18081", "null"}} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:18081/api/settings", nil)
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("accepted host/origin: %+v", tc)
		}
	}
}
