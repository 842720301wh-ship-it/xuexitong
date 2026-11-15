package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"tiantianxiangshang/internal/settings"
)

func modelProtocol(protocol, endpoint string) (string, error) {
	if protocol == "" {
		switch {
		case strings.HasSuffix(strings.TrimRight(endpoint, "/"), "/responses"):
			protocol = "responses"
		case strings.HasSuffix(strings.TrimRight(endpoint, "/"), "/messages"):
			protocol = "anthropic"
		default:
			protocol = "chat"
		}
	}
	switch protocol {
	case "chat", "responses", "anthropic":
		return protocol, nil
	}
	return "", errors.New("请选择有效的接口格式")
}
func modelHeaders(req *http.Request, protocol, key string) {
	req.Header.Set("Accept", "application/json")
	if protocol == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}
func protocolLabel(protocol string) string {
	switch protocol {
	case "responses":
		return "Responses"
	case "anthropic":
		return "Anthropic Messages"
	default:
		return "Chat Completions"
	}
}
func modelHTTPError(status int, protocol string) error {
	hint := "请检查接口地址、模型 ID 及额度"
	switch status {
	case 400, 422:
		hint = "请检查所选接口格式、模型 ID 与服务商要求的参数"
	case 401, 403:
		hint = "请检查 API Key 及模型访问权限"
	case 402:
		hint = "调用额度已达上限，请在服务商控制台检查用量、余额或额度重置时间"
	case 404, 405:
		hint = "请检查接口基础地址及所选接口格式是否受支持"
	case 429:
		hint = "请求频率或用量受限，请稍后重试或检查服务商限额"
	}
	return fmt.Errorf("%s 接口返回 HTTP %d，%s", protocolLabel(protocol), status, hint)
}

// Each format has its own request and response adapter. No silent fallback that
// might send the same prompt to multiple endpoints or incur duplicate charges.
func (s *Server) testModel(w http.ResponseWriter, r *http.Request) {
	var model settings.Model
	for _, m := range s.Store.Read().Models {
		if m.ID == r.PathValue("id") {
			model = m
			break
		}
	}
	if model.ID == "" {
		fail(w, errors.New("请先保存模型配置"))
		return
	}
	var input struct {
		Protocol string `json:"protocol"`
	}
	if r.ContentLength > 0 {
		if err := decode(w, r, &input); err != nil {
			fail(w, err)
			return
		}
	}
	if input.Protocol != "" {
		model.Protocol = input.Protocol
	}
	s.writeModelTest(w, r, model)
}

func (s *Server) writeModelTest(w http.ResponseWriter, r *http.Request, model settings.Model) {
	protocol, e := modelProtocol(model.Protocol, model.Endpoint)
	if e != nil {
		fail(w, e)
		return
	}
	base, e := modelBaseURL(model.Endpoint)
	if e != nil {
		fail(w, e)
		return
	}
	payload := map[string]any{"model": model.Primary, "stream": false}
	path := "/chat/completions"
	switch protocol {
	case "responses":
		path = "/responses"
		payload["input"] = "Reply OK."
	case "anthropic":
		path = "/messages"
		payload["max_tokens"] = 256
		fallthrough
	default:
		payload["messages"] = []map[string]string{{"role": "user", "content": "Reply OK."}}
	}
	b, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(r.Context(), http.MethodPost, base+path, strings.NewReader(string(b)))
	if e != nil {
		fail(w, errors.New("无法创建模型请求"))
		return
	}
	modelHeaders(req, protocol, model.APIKey)
	req.Header.Set("Content-Type", "application/json")
	response, e := s.HTTP.Do(req)
	if e != nil {
		fail(w, errors.New("模型连接失败或超时，请检查接口地址及网络"))
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fail(w, upstreamModelError(response, protocol, model.APIKey))
		return
	}
	b, e = io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 || !validModelReply(protocol, b) {
		fail(w, fmt.Errorf("接口未返回有效的 %s 响应，请核对接口格式和模型", protocolLabel(protocol)))
		return
	}
	respond(w, map[string]string{"message": "连接成功，主模型可用", "protocol": protocol})
}

// Return a short, redacted upstream diagnostic rather than hiding every 400
// behind a generic Key/credits message. Never return raw bodies or URLs.
func upstreamModelError(response *http.Response, protocol, key string) error {
	summary := modelHTTPError(response.StatusCode, protocol).Error()
	b, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
	var envelope struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return errors.New(summary)
	}
	detail := envelope.Message
	if len(envelope.Error) > 0 {
		var object struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(envelope.Error, &object) == nil {
			detail = object.Message
		} else {
			var message string
			if json.Unmarshal(envelope.Error, &message) == nil {
				detail = message
			}
		}
	}
	if key != "" {
		detail = strings.ReplaceAll(detail, key, "[已隐藏]")
	}
	detail = regexp.MustCompile(`https?://[^\s]+`).ReplaceAllString(detail, "[接口地址]")
	detail = regexp.MustCompile(`(?i)(bearer\s+|(?:api[_-]?key|token|password|authorization)["']?\s*[:=]\s*["']?)[^\s"',}]+`).ReplaceAllString(detail, "${1}[已隐藏]")
	detail = strings.Join(strings.Fields(detail), " ")
	runes := []rune(detail)
	if len(runes) > 300 {
		detail = string(runes[:300]) + "…"
	}
	if detail != "" {
		summary += "；服务端说明：" + detail
	}
	return errors.New(summary)
}
func validModelReply(protocol string, b []byte) bool {
	var result struct {
		Error   json.RawMessage `json:"error"`
		Status  string          `json:"status"`
		Choices []struct {
			Message struct {
				Content   json.RawMessage `json:"content"`
				Reasoning string          `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(b, &result) != nil || (len(result.Error) > 0 && string(result.Error) != "null") {
		return false
	}
	switch protocol {
	case "responses":
		if result.Status != "" && result.Status != "completed" {
			return false
		}
		for _, item := range result.Output {
			if item.Type == "message" {
				for _, part := range item.Content {
					if part.Type == "output_text" && strings.TrimSpace(part.Text) != "" {
						return true
					}
				}
			}
		}
	case "anthropic":
		for _, part := range result.Content {
			if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
				return true
			}
		}
	default:
		for _, choice := range result.Choices {
			var text string
			if json.Unmarshal(choice.Message.Content, &text) == nil && strings.TrimSpace(text) != "" {
				return true
			}
			var parts []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(choice.Message.Content, &parts) == nil {
				for _, part := range parts {
					if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
						return true
					}
				}
			}
		}
	}
	return false
}
