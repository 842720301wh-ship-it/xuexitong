package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"tiantianxiangshang/internal/settings"
)

func taskFixture(t *testing.T) (*Server, settings.Task, string) {
	t.Helper()
	s, _, dir := setup(t)
	e := s.Store.Update(func(c *settings.Config) error {
		c.Accounts = []settings.Account{{ID: "account-one", Name: "张三", Username: "13811112222", Password: "private-password"}, {ID: "account-two", Name: "李四", Username: "13833334444", Password: "private-password-two"}}
		c.DefaultAccountID = "account-one"
		c.Models = []settings.Model{{ID: "provider-one", Name: "模型配置一", APIKey: "private-model-key", Primary: "chat", Models: []string{"chat"}}, {ID: "provider-two", Name: "模型配置二", APIKey: "private-model-key-two", Primary: "reasoner", Models: []string{"reasoner"}}}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return s, settings.Task{Name: "工程检测加固", AccountID: "account-one", ModelConfigID: "provider-one", Courses: []settings.TaskCourse{{ID: "247370145", Name: "工程检测加固"}}, AIRetries: 3, Retries: 2, RetryDelay: 60, Speed: 1, Jobs: 1, Sequential: true, NotOpenAction: "retry", AutoSubmit: true}, dir
}
func taskBody(t *testing.T, task settings.Task) string {
	t.Helper()
	b, e := json.Marshal(task)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func readTask(t *testing.T, w *httptest.ResponseRecorder) settings.Task {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var task settings.Task
	if e := json.Unmarshal(w.Body.Bytes(), &task); e != nil {
		t.Fatal(e)
	}
	return task
}

func TestTaskSelectionPersistenceAndDefaults(t *testing.T) {
	s, task, dir := taskFixture(t)
	h := s.Handler(t.TempDir())
	task.AccountID = "account-two"
	task.ModelConfigID = "provider-two"
	saved := readTask(t, request(h, "POST", "/api/tasks", taskBody(t, task)))
	if saved.ID == "" || saved.AccountID != "account-two" || saved.ModelConfigID != "provider-two" || saved.CreatedAt == "" {
		t.Fatalf("task lost selected configuration: %+v", saved)
	}
	if s.Store.Read().DefaultAccountID != "account-one" {
		t.Fatal("task account selection changed the global default")
	}
	read(t, request(h, "PUT", "/api/accounts/default", `{"id":"account-two"}`))
	saved.AccountID = "account-one"
	saved.ModelConfigID = "provider-one"
	saved.Name = "已编辑的学习计划"
	edited := readTask(t, request(h, "PUT", "/api/tasks/"+saved.ID, taskBody(t, saved)))
	if edited.ID != saved.ID || edited.CreatedAt != saved.CreatedAt || len(s.Store.Read().Tasks) != 1 {
		t.Fatal("editing created a new task or changed its original creation time")
	}
	reopened, e := settings.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	persisted := reopened.Read().Tasks[0]
	if persisted.AccountID != "account-one" || persisted.ModelConfigID != "provider-one" || persisted.Name != edited.Name {
		t.Fatal("selection not restored after restart")
	}
	payload := request(h, "GET", "/api/tasks", "")
	for _, secret := range []string{"private-password", "private-model-key", `"password"`, `"api_key"`} {
		if strings.Contains(payload.Body.String(), secret) {
			t.Fatal("task response contains credentials")
		}
	}
	read(t, request(h, "DELETE", "/api/accounts/account-one", ""))
	if s.Store.Read().Tasks[0].AccountID != "account-one" {
		t.Fatal("removing an account silently reassigned an existing task")
	}
	if response := request(h, "PUT", "/api/tasks/"+saved.ID, taskBody(t, edited)); response.Code != 400 {
		t.Fatal("task accepted a removed account")
	}
	if response := request(h, "DELETE", "/api/tasks/"+saved.ID, ""); response.Code != 200 || len(s.Store.Read().Tasks) != 0 {
		t.Fatal("task removal failed")
	}
}

func TestTaskRejectsInvalidReferencesAndOptions(t *testing.T) {
	tests := []struct {
		name   string
		change func(*settings.Task)
	}{
		{"empty name", func(t *settings.Task) { t.Name = " " }},
		{"missing account", func(t *settings.Task) { t.AccountID = "missing" }},
		{"missing model", func(t *settings.Task) { t.ModelConfigID = "missing" }},
		{"no courses", func(t *settings.Task) { t.Courses = nil }},
		{"invalid course", func(t *settings.Task) { t.Courses[0].ID = "javascript:bad" }},
		{"duplicate course", func(t *settings.Task) { t.Courses = append(t.Courses, t.Courses[0]) }},
		{"bad speed", func(t *settings.Task) { t.Speed = 0 }},
		{"bad retry count", func(t *settings.Task) { t.Retries = -1 }},
		{"bad ai retries", func(t *settings.Task) { t.AIRetries = 11 }},
		{"bad delay", func(t *settings.Task) { t.RetryDelay = 0 }},
		{"bad chapter action", func(t *settings.Task) { t.NotOpenAction = "unknown" }},
		{"invalid concurrency", func(t *settings.Task) { t.Jobs = 2 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, task, _ := taskFixture(t)
			tc.change(&task)
			w := request(s.Handler(t.TempDir()), "POST", "/api/tasks", taskBody(t, task))
			if w.Code != 400 || len(s.Store.Read().Tasks) != 0 {
				t.Fatalf("invalid task saved: %d %s", w.Code, w.Body.String())
			}
		})
	}
	s, task, _ := taskFixture(t)
	h := s.Handler(t.TempDir())
	saved := readTask(t, request(h, "POST", "/api/tasks", taskBody(t, task)))
	read(t, request(h, "DELETE", "/api/models/provider-one", ""))
	if request(h, "PUT", "/api/tasks/"+saved.ID, taskBody(t, saved)).Code != 400 {
		t.Fatal("removed model config accepted")
	}
	if request(h, "POST", "/api/tasks", `{"password":"do-not-duplicate"}`).Code != 400 {
		t.Fatal("unexpected secret fields accepted")
	}
}

func TestTaskLegacyConfigAndEmptyList(t *testing.T) {
	s, _, dir := setup(t)
	// Existing configuration files have no tasks field or a nil task list.
	if e := s.Store.Update(func(c *settings.Config) error { c.Tasks = nil; return nil }); e != nil {
		t.Fatal(e)
	}
	reopened, e := settings.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	w := request(New(reopened).Handler(t.TempDir()), "GET", "/api/tasks", "")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("legacy configuration did not yield an empty task list")
	}
}
