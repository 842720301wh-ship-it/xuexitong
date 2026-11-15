package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"tiantianxiangshang/internal/learning"
	"tiantianxiangshang/internal/settings"
)

type Server struct {
	Store       *settings.Store
	Lookup      func(context.Context, string, string) (learning.AccountData, error)
	HTTP        *http.Client
	progressMu  sync.Mutex
	progress    map[string]TaskProgress
	NativeToken string
	nativeMu    sync.Mutex
	nativeTask  string
	nativeRun   string
	records     []RunRecord
}

func New(store *settings.Store) *Server {
	return &Server{Store: store, Lookup: func(ctx context.Context, u, p string) (learning.AccountData, error) {
		return learning.NewCourseClient().Account(ctx, u, p)
	}, HTTP: &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeLimit(w, r, v, 64<<10)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("请求内容无效")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求内容无效")
	}
	return nil
}
func (s *Server) Handler(web string) http.Handler {
	return s.HandlerFS(os.DirFS(web))
}

// HandlerFS serves the same API with desktop files or APK-embedded web assets.
func (s *Server) HandlerFS(web fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) { respond(w, settings.Public(s.Store.Read())) })
	mux.HandleFunc("POST /api/accounts", s.addAccount)
	mux.HandleFunc("PUT /api/accounts/default", s.selectAccount)
	mux.HandleFunc("DELETE /api/accounts/{id}", s.deleteAccount)
	mux.HandleFunc("POST /api/accounts/{id}/check", s.checkAccount)
	mux.HandleFunc("GET /api/accounts/{id}/courses", s.courses)
	mux.HandleFunc("POST /api/models", s.saveModel)
	mux.HandleFunc("POST /api/models/discover", s.discoverModels)
	mux.HandleFunc("POST /api/models/probe", s.probeModel)
	mux.HandleFunc("POST /api/native/tasks/{id}/claim", s.claimNativeTask)
	mux.HandleFunc("POST /api/native/tasks/{id}/progress", s.nativeProgress)
	mux.HandleFunc("GET /api/records", s.runRecords)
	mux.HandleFunc("PUT /api/models/{id}", s.saveModel)
	mux.HandleFunc("DELETE /api/models/{id}", s.deleteModel)
	mux.HandleFunc("POST /api/models/{id}/test", s.testModel)
	mux.HandleFunc("GET /api/tasks", s.tasks)
	mux.HandleFunc("GET /api/task-progress", s.taskProgress)
	mux.HandleFunc("POST /api/tasks", s.saveTask)
	mux.HandleFunc("PUT /api/tasks/{id}", s.saveTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.deleteTask)
	mux.HandleFunc("GET /api/task-defaults", func(w http.ResponseWriter, r *http.Request) {
		c := s.Store.Read()
		respond(w, map[string]string{"account_id": c.DefaultAccountID})
	})
	mux.Handle("/", http.FileServer(http.FS(web)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			host, _, e := net.SplitHostPort(r.Host)
			if e != nil {
				host = r.Host
			}
			ip := net.ParseIP(host)
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				http.Error(w, "Invalid host", 403)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host {
					http.Error(w, "Invalid origin", 403)
					return
				}
			}
			if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != "GET" && r.Method != "DELETE" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "JSON required", 415)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) config(w http.ResponseWriter, e error) {
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, settings.Public(s.Store.Read()))
}
func (s *Server) addAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Name = strings.TrimSpace(in.Name)
	if in.Username == "" || in.Password == "" || len(in.Username) > 256 || len(in.Password) > 4096 || len(in.Name) > 100 {
		fail(w, errors.New("请填写有效的账号与密码"))
		return
	}
	profile, e := s.Lookup(r.Context(), in.Username, in.Password)
	if e != nil {
		fail(w, e)
		return
	}
	s.config(w, s.Store.Update(func(c *settings.Config) error {
		for i := range c.Accounts {
			if c.Accounts[i].Username == in.Username {
				c.Accounts[i].Password = in.Password
				c.Accounts[i].Name = profile.Name
				c.Accounts[i].Status = "connected"
				c.Accounts[i].CheckedAt = settings.Timestamp()
				return nil
			}
		}
		a := settings.Account{ID: settings.ID(), Name: profile.Name, Username: in.Username, Password: in.Password, Status: "connected", CheckedAt: settings.Timestamp()}
		c.Accounts = append(c.Accounts, a)
		if c.DefaultAccountID == "" {
			c.DefaultAccountID = a.ID
		}
		return nil
	}))
}
func (s *Server) selectAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID string `json:"id"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	s.config(w, s.Store.Update(func(c *settings.Config) error {
		for _, a := range c.Accounts {
			if a.ID == in.ID {
				c.DefaultAccountID = a.ID
				return nil
			}
		}
		return errors.New("账号已不存在，请刷新")
	}))
}
func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	s.config(w, s.Store.Update(func(c *settings.Config) error {
		id := r.PathValue("id")
		i := slices.IndexFunc(c.Accounts, func(a settings.Account) bool { return a.ID == id })
		if i < 0 {
			return errors.New("账号不存在")
		}
		c.Accounts = append(c.Accounts[:i], c.Accounts[i+1:]...)
		if c.DefaultAccountID == id {
			c.DefaultAccountID = ""
			if len(c.Accounts) > 0 {
				c.DefaultAccountID = c.Accounts[0].ID
			}
		}
		return nil
	}))
}
func (s *Server) account(id string) (settings.Account, error) {
	for _, a := range s.Store.Read().Accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return settings.Account{}, errors.New("账号不存在")
}
func (s *Server) loadCourses(r *http.Request) ([]learning.Course, error) {
	a, e := s.account(r.PathValue("id"))
	if e != nil {
		return nil, e
	}
	profile, lookupErr := s.Lookup(r.Context(), a.Username, a.Password)
	e = s.Store.Update(func(c *settings.Config) error {
		for i := range c.Accounts {
			if c.Accounts[i].ID == a.ID && c.Accounts[i].Password == a.Password {
				c.Accounts[i].Status = "unknown"
				if lookupErr == nil {
					c.Accounts[i].Status = "connected"
					c.Accounts[i].CheckedAt = settings.Timestamp()
					c.Accounts[i].Name = profile.Name
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return profile.Courses, lookupErr
}
func (s *Server) checkAccount(w http.ResponseWriter, r *http.Request) {
	_, e := s.loadCourses(r)
	s.config(w, e)
}
func (s *Server) courses(w http.ResponseWriter, r *http.Request) {
	items, e := s.loadCourses(r)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, map[string]any{"courses": items})
}
func validateModel(m settings.Model) error {
	if _, err := modelBaseURL(m.Endpoint); err != nil {
		return err
	}
	if _, err := modelProtocol(m.Protocol, m.Endpoint); err != nil {
		return err
	}
	if m.Name == "" || len([]rune(m.Name)) > 100 || m.APIKey == "" || len(m.APIKey) > 4096 || strings.ContainsAny(m.APIKey, "\r\n") || len(m.Models) == 0 || len(m.Models) > 1000 || !slices.Contains(m.Models, m.Primary) || (m.Fallback != "" && (!slices.Contains(m.Models, m.Fallback) || m.Fallback == m.Primary)) {
		return errors.New("请填写配置名称、API Key、模型列表并选择有效的主模型和备用模型")
	}
	for _, id := range m.Models {
		if len(id) > 256 || strings.ContainsAny(id, "\r\n\x00") {
			return errors.New("模型 ID 无效")
		}
	}
	return nil
}
func (s *Server) saveModel(w http.ResponseWriter, r *http.Request) {
	var m settings.Model
	if e := decodeLimit(w, r, &m, 512<<10); e != nil {
		fail(w, e)
		return
	}
	m.ID = r.PathValue("id")
	m.Name = strings.TrimSpace(m.Name)
	m.Endpoint = strings.TrimRight(strings.TrimSpace(m.Endpoint), "/")
	base, err := modelBaseURL(m.Endpoint)
	if err != nil {
		fail(w, err)
		return
	}
	m.Protocol, err = modelProtocol(m.Protocol, m.Endpoint)
	if err != nil {
		fail(w, err)
		return
	}
	m.Endpoint = base
	m.APIKey = strings.TrimSpace(m.APIKey)
	models := []string{}
	for _, model := range m.Models {
		v := strings.TrimSpace(model)
		if v != "" && !slices.Contains(models, v) {
			models = append(models, v)
		}
	}
	m.Models = models
	s.config(w, s.Store.Update(func(c *settings.Config) error {
		i := slices.IndexFunc(c.Models, func(old settings.Model) bool { return old.ID == m.ID })
		if m.ID != "" && i < 0 {
			return errors.New("模型配置不存在")
		}
		if i >= 0 && m.APIKey == "" {
			previousBase, err := modelBaseURL(c.Models[i].Endpoint)
			if err != nil || previousBase != m.Endpoint {
				return errors.New("接口地址已改变，请重新输入 API Key 后测试连接")
			}
			m.APIKey = c.Models[i].APIKey
		}
		if e := validateModel(m); e != nil {
			return e
		}
		m.HasKey = false
		if i >= 0 {
			c.Models[i] = m
		} else {
			m.ID = settings.ID()
			c.Models = append(c.Models, m)
		}
		return nil
	}))
}
func (s *Server) deleteModel(w http.ResponseWriter, r *http.Request) {
	s.config(w, s.Store.Update(func(c *settings.Config) error {
		i := slices.IndexFunc(c.Models, func(m settings.Model) bool { return m.ID == r.PathValue("id") })
		if i < 0 {
			return errors.New("模型配置不存在")
		}
		c.Models = append(c.Models[:i], c.Models[i+1:]...)
		return nil
	}))
}
