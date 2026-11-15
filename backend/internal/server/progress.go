package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"tiantianxiangshang/internal/settings"
)

// TaskProgress is the latest structured event for a task, not a log history.
type TaskProgress struct {
	State           string `json:"state"`
	CourseName      string `json:"course_name"`
	ChapterName     string `json:"chapter_name"`
	PositionSeconds int    `json:"position_seconds"`
	DurationSeconds int    `json:"duration_seconds"`
	Message         string `json:"message"`
	UpdatedAt       string `json:"updated_at"`
}

// ReportTaskProgress is an internal integration point for the local runner.
// There is deliberately no HTTP endpoint that fabricates running state.
func (s *Server) ReportTaskProgress(taskID string, event TaskProgress) error {
	if !slices.ContainsFunc(s.Store.Read().Tasks, func(t settings.Task) bool { return t.ID == taskID }) {
		return errors.New("任务不存在")
	}
	if !slices.Contains([]string{"starting", "running", "queued", "retrying", "paused", "completed", "failed", "cancelled"}, event.State) {
		return errors.New("运行状态无效")
	}
	if event.PositionSeconds < 0 || event.DurationSeconds < 0 || (event.DurationSeconds > 0 && event.PositionSeconds > event.DurationSeconds) {
		return errors.New("视频时段无效")
	}
	if utf8.RuneCountInString(event.CourseName) > 200 || utf8.RuneCountInString(event.ChapterName) > 200 || utf8.RuneCountInString(event.Message) > 2000 {
		return errors.New("进度文本过长")
	}
	// Terminal log output can use carriage returns. Keep only the newest nonempty line.
	lines := strings.FieldsFunc(event.Message, func(r rune) bool { return r == '\r' || r == '\n' })
	event.Message = ""
	for i := len(lines) - 1; i >= 0; i-- {
		if text := strings.TrimSpace(lines[i]); text != "" {
			event.Message = text
			break
		}
	}
	event.UpdatedAt = settings.Timestamp()
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	if s.progress == nil {
		s.progress = make(map[string]TaskProgress)
	}
	s.progress[taskID] = event
	return nil
}
func (s *Server) taskProgress(w http.ResponseWriter, r *http.Request) {
	valid := make(map[string]bool)
	for _, task := range s.Store.Read().Tasks {
		valid[task.ID] = true
	}
	latest := make(map[string]TaskProgress)
	s.progressMu.Lock()
	for id, event := range s.progress {
		if valid[id] {
			latest[id] = event
		} else {
			delete(s.progress, id)
		}
	}
	s.progressMu.Unlock()
	respond(w, latest)
}
