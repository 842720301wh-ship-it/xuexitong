package server

import (
	"errors"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"tiantianxiangshang/internal/settings"
)

func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.Store.Read().Tasks
	if tasks == nil {
		tasks = []settings.Task{}
	}
	respond(w, tasks)
}

var taskCourseID = regexp.MustCompile(`^[0-9]{1,30}$`)

func validateTask(t *settings.Task, c *settings.Config) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" || len([]rune(t.Name)) > 100 {
		return errors.New("请填写任务名称（最多 100 字）")
	}
	if !slices.ContainsFunc(c.Accounts, func(a settings.Account) bool { return a.ID == t.AccountID }) {
		return errors.New("所选账号已不存在，请从账号列表重新选择")
	}
	if !slices.ContainsFunc(c.Models, func(m settings.Model) bool { return m.ID == t.ModelConfigID && m.APIKey != "" }) {
		return errors.New("所选模型配置已不存在或缺少密钥，请重新选择")
	}
	if len(t.Courses) == 0 || len(t.Courses) > 100 {
		return errors.New("请选择 1 至 100 门课程")
	}
	seen := map[string]bool{}
	for i := range t.Courses {
		t.Courses[i].ID = strings.TrimSpace(t.Courses[i].ID)
		t.Courses[i].Name = strings.TrimSpace(t.Courses[i].Name)
		if !taskCourseID.MatchString(t.Courses[i].ID) || seen[t.Courses[i].ID] || len([]rune(t.Courses[i].Name)) > 200 {
			return errors.New("课程 ID 必须为不重复的数字，课程名称最多 200 字")
		}
		seen[t.Courses[i].ID] = true
	}
	if t.AIRetries < 0 || t.AIRetries > 10 || t.Retries < 0 || t.Retries > 10 || t.RetryDelay < 1 || t.RetryDelay > 3600 || math.IsNaN(t.Speed) || math.IsInf(t.Speed, 0) || t.Speed < 0.5 || t.Speed > 4 || t.Jobs < 1 || t.Jobs > 10 {
		return errors.New("运行参数超出范围，请检查重试次数、等待时间、视频速度和章节并发数")
	}
	if t.Sequential && t.Jobs != 1 {
		return errors.New("顺序解锁章节时，章节并发数须为 1")
	}
	if t.NotOpenAction != "retry" && t.NotOpenAction != "skip" && t.NotOpenAction != "stop" {
		return errors.New("请选择有效的未开放章节处理方式")
	}
	return nil
}

func (s *Server) saveTask(w http.ResponseWriter, r *http.Request) {
	var task settings.Task
	if e := decode(w, r, &task); e != nil {
		fail(w, e)
		return
	}
	task.ID = r.PathValue("id")
	if s.taskActive(task.ID) {
		fail(w, errors.New("请先停止任务再编辑"))
		return
	}
	e := s.Store.Update(func(c *settings.Config) error {
		index := slices.IndexFunc(c.Tasks, func(t settings.Task) bool { return t.ID == task.ID })
		if task.ID != "" && index < 0 {
			return errors.New("任务已不存在，请刷新列表")
		}
		if e := validateTask(&task, c); e != nil {
			return e
		}
		task.UpdatedAt = settings.Timestamp()
		if index >= 0 {
			task.CreatedAt = c.Tasks[index].CreatedAt
			c.Tasks[index] = task
		} else {
			task.ID = settings.ID()
			task.CreatedAt = task.UpdatedAt
			c.Tasks = append(c.Tasks, task)
		}
		return nil
	})
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, task)
}
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if s.taskActive(r.PathValue("id")) {
		fail(w, errors.New("请先停止任务再删除"))
		return
	}
	e := s.Store.Update(func(c *settings.Config) error {
		index := slices.IndexFunc(c.Tasks, func(t settings.Task) bool { return t.ID == r.PathValue("id") })
		if index < 0 {
			return errors.New("任务已不存在")
		}
		c.Tasks = append(c.Tasks[:index], c.Tasks[index+1:]...)
		return nil
	})
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, map[string]bool{"deleted": true})
}
