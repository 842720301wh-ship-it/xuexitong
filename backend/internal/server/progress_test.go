package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskProgressKeepsOnlyLatestEvent(t *testing.T) {
	s, task, _ := taskFixture(t)
	h := s.Handler(t.TempDir())
	saved := readTask(t, request(h, "POST", "/api/tasks", taskBody(t, task)))
	if got := request(h, "GET", "/api/task-progress", ""); got.Code != 200 || strings.TrimSpace(got.Body.String()) != "{}" {
		t.Fatal("idle task had fabricated progress")
	}
	event := TaskProgress{State: "running", CourseName: "工程检测加固", ChapterName: "第二节", PositionSeconds: 6, DurationSeconds: 357, Message: "旧记录\r最新记录\n  "}
	if e := s.ReportTaskProgress(saved.ID, event); e != nil {
		t.Fatal(e)
	}
	event.PositionSeconds = 12
	event.Message = "已学习至 00:12"
	if e := s.ReportTaskProgress(saved.ID, event); e != nil {
		t.Fatal(e)
	}
	response := request(h, "GET", "/api/task-progress", "")
	var result map[string]TaskProgress
	if e := json.Unmarshal(response.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if len(result) != 1 || result[saved.ID].PositionSeconds != 12 || result[saved.ID].Message != "已学习至 00:12" || result[saved.ID].UpdatedAt == "" {
		t.Fatalf("latest event not returned: %s", response.Body.String())
	}
	for _, secret := range []string{"private-password", "private-model-key"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Fatal("progress leaked credentials")
		}
	}
	request(h, "DELETE", "/api/tasks/"+saved.ID, "")
	if got := request(h, "GET", "/api/task-progress", ""); strings.TrimSpace(got.Body.String()) != "{}" {
		t.Fatal("removed task retained a visible progress entry")
	}
	if e := s.ReportTaskProgress(saved.ID, event); e == nil {
		t.Fatal("removed task accepted progress")
	}
}

func TestTaskProgressRejectsInvalidInputAndNormalizesLog(t *testing.T) {
	s, task, _ := taskFixture(t)
	h := s.Handler(t.TempDir())
	saved := readTask(t, request(h, "POST", "/api/tasks", taskBody(t, task)))
	for _, event := range []TaskProgress{{State: "fake"}, {State: "running", PositionSeconds: -1}, {State: "running", PositionSeconds: 30, DurationSeconds: 10}} {
		if s.ReportTaskProgress(saved.ID, event) == nil {
			t.Fatal("invalid progress accepted")
		}
	}
	if e := s.ReportTaskProgress(saved.ID, TaskProgress{State: "running", Message: "old line\rnew line\n \n"}); e != nil {
		t.Fatal(e)
	}
	var result map[string]TaskProgress
	_ = json.Unmarshal(request(h, "GET", "/api/task-progress", "").Body.Bytes(), &result)
	if result[saved.ID].Message != "new line" {
		t.Fatal("log did not retain only the latest nonempty line")
	}
	if got := request(h, "POST", "/api/task-progress", `{"state":"running"}`); got.Code != 405 && got.Code != 404 {
		t.Fatal("public progress fabrication endpoint exposed")
	}
}
