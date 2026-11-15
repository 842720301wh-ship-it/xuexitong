package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Preserve provider prefixes such as /openai/v1. Do not guess or add /v1.
func modelBaseURL(endpoint string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(endpoint))
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("请填写有效的模型接口地址")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errors.New("模型接口请使用 HTTPS；本机服务可使用 HTTP")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{"/chat/completions", "/responses", "/messages", "/models"} {
		if strings.HasSuffix(u.Path, suffix) {
			u.Path = strings.TrimSuffix(u.Path, suffix)
			break
		}
	}
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func (s *Server) discoverModels(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConfigID string `json:"config_id"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
	}
	if e := decode(w, r, &input); e != nil {
		fail(w, e)
		return
	}
	base, e := modelBaseURL(input.Endpoint)
	if e != nil {
		fail(w, e)
		return
	}
	protocol, e := modelProtocol(input.Protocol, input.Endpoint)
	if e != nil {
		fail(w, e)
		return
	}
	key := strings.TrimSpace(input.APIKey)
	if key == "" && input.ConfigID != "" {
		found := false
		for _, model := range s.Store.Read().Models {
			if model.ID == input.ConfigID {
				found = true
				savedBase, err := modelBaseURL(model.Endpoint)
				if err != nil || base != savedBase {
					fail(w, errors.New("接口地址已改变，请重新输入 API Key 后测试连接"))
					return
				}
				key = model.APIKey
				break
			}
		}
		if !found {
			fail(w, errors.New("模型配置已不存在，请刷新后重试"))
			return
		}
	}
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n") {
		fail(w, errors.New("请先填写有效的 API Key"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if e != nil {
		fail(w, errors.New("无法创建模型列表请求"))
		return
	}
	modelHeaders(req, protocol, key)
	if protocol == "anthropic" {
		req.URL.RawQuery = "limit=1000"
	}
	response, e := s.HTTP.Do(req)
	if e != nil {
		fail(w, errors.New("连接超时或网络异常，请检查接口地址后重试"))
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		case 401, 403:
			fail(w, errors.New("认证失败，请检查 API Key 及模型列表访问权限"))
		case 404, 405:
			fail(w, errors.New("未找到模型列表接口，请确认 API 基础地址及服务商是否支持 /models"))
		default:
			fail(w, fmt.Errorf("模型列表接口返回 HTTP %d，请稍后重试", response.StatusCode))
		}
		return
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if e != nil || len(b) > 4<<20 {
		fail(w, errors.New("模型列表响应异常或过大"))
		return
	}
	var catalog struct {
		HasMore bool `json:"has_more"`
		Data    []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &catalog) != nil {
		fail(w, errors.New("返回的模型列表格式无效，需要兼容 OpenAI 的 data[].id 格式"))
		return
	}
	if catalog.HasMore {
		fail(w, errors.New("模型列表超过单次获取范围，请使用范围更小的接口配置"))
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, model := range catalog.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		if len(id) > 256 || strings.ContainsAny(id, "\r\n\x00") {
			fail(w, errors.New("模型列表包含无效的模型 ID"))
			return
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
		if len(ids) > 1000 {
			fail(w, errors.New("模型列表超过 1000 项，请使用范围更小的接口配置"))
			return
		}
	}
	if len(ids) == 0 {
		fail(w, errors.New("接口已连接，但未返回可选模型，请检查 Key 的访问权限"))
		return
	}
	// Discovery never saves the draft, and never returns the key to the browser.
	respond(w, map[string]any{"models": ids, "message": fmt.Sprintf("连接成功，已获取 %d 个模型", len(ids))})
}
