package server

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"

	"tiantianxiangshang/internal/settings"
)

type RunRecord struct {
	TaskID   string `json:"task_id"`
	TaskName string `json:"task_name"`
	RunID    string `json:"run_id"`
	TaskProgress
}

func (s *Server) nativeAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if s.NativeToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Tiantian-Native")), []byte(s.NativeToken)) != 1 {
		http.Error(w, "Native runtime authorization required", http.StatusForbidden)
		return false
	}
	return true
}

// Decrypted credentials are available only to the in-process Android worker.
// The token is created natively and is never exposed to the WebView.
func (s *Server) claimNativeTask(w http.ResponseWriter, r *http.Request) {
	if !s.nativeAuthorized(w, r) {
		return
	}
	config := s.Store.Read()
	i := slices.IndexFunc(config.Tasks, func(t settings.Task) bool { return t.ID == r.PathValue("id") })
	if i < 0 {
		fail(w, errors.New("任务不存在"))
		return
	}
	task := config.Tasks[i]
	if e := validateTask(&task, &config); e != nil {
		fail(w, e)
		return
	}
	s.nativeMu.Lock()
	if s.nativeTask != "" {
		s.nativeMu.Unlock()
		fail(w, errors.New("已有任务在执行，请先停止当前任务"))
		return
	}
	s.nativeTask = task.ID
	s.nativeRun = settings.ID()
	runID := s.nativeRun
	s.nativeMu.Unlock()
	ai := slices.IndexFunc(config.Accounts, func(a settings.Account) bool { return a.ID == task.AccountID })
	mi := slices.IndexFunc(config.Models, func(m settings.Model) bool { return m.ID == task.ModelConfigID })
	model := config.Models[mi]
	if protocol, err := modelProtocol(model.Protocol, model.Endpoint); err == nil {
		model.Protocol = protocol
	}
	if base, err := modelBaseURL(model.Endpoint); err == nil {
		model.Endpoint = base
	}
	_ = s.ReportTaskProgress(task.ID, TaskProgress{State: "starting", Message: "正在启动手机本地执行器"})
	respond(w, map[string]any{"task": task, "account": config.Accounts[ai], "model": model, "run_id": runID})
}
func (s *Server) nativeProgress(w http.ResponseWriter, r *http.Request) {
	if !s.nativeAuthorized(w, r) {
		return
	}
	var input struct {
		RunID string       `json:"run_id"`
		Event TaskProgress `json:"event"`
	}
	if e := decode(w, r, &input); e != nil {
		fail(w, e)
		return
	}
	s.nativeMu.Lock()
	defer s.nativeMu.Unlock()
	if input.RunID == "" || s.nativeRun != input.RunID || s.nativeTask != r.PathValue("id") {
		fail(w, errors.New("运行已结束或已被替换"))
		return
	}
	if e := s.ReportTaskProgress(s.nativeTask, input.Event); e != nil {
		fail(w, e)
		return
	}
	// Copy the sanitized event, not upstream request/response dumps.
	s.progressMu.Lock()
	event := s.progress[s.nativeTask]
	s.progressMu.Unlock()
	name := ""
	for _, task := range s.Store.Read().Tasks {
		if task.ID == s.nativeTask {
			name = task.Name
			break
		}
	}
	s.records = append(s.records, RunRecord{TaskID: s.nativeTask, TaskName: name, RunID: s.nativeRun, TaskProgress: event})
	if len(s.records) > 500 {
		s.records = append([]RunRecord(nil), s.records[len(s.records)-500:]...)
	}
	if slices.Contains([]string{"completed", "failed", "cancelled", "paused"}, event.State) {
		s.nativeTask = ""
		s.nativeRun = ""
	}
	respond(w, map[string]bool{"accepted": true})
}
func (s *Server) runRecords(w http.ResponseWriter, r *http.Request) {
	s.nativeMu.Lock()
	records := append([]RunRecord{}, s.records...)
	s.nativeMu.Unlock()
	slices.Reverse(records)
	respond(w, records)
}
func (s *Server) taskActive(id string) bool {
	s.nativeMu.Lock()
	defer s.nativeMu.Unlock()
	return id != "" && s.nativeTask == id
}
