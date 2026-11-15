package server

import (
	"errors"
	"net/http"
	"strings"

	"tiantianxiangshang/internal/settings"
)

func (s *Server) probeModel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConfigID string `json:"config_id"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
		Primary  string `json:"primary"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	base, err := modelBaseURL(input.Endpoint)
	if err != nil {
		fail(w, err)
		return
	}
	protocol, err := modelProtocol(input.Protocol, input.Endpoint)
	if err != nil {
		fail(w, err)
		return
	}
	key := strings.TrimSpace(input.APIKey)
	if key == "" && input.ConfigID != "" {
		for _, model := range s.Store.Read().Models {
			if model.ID == input.ConfigID {
				previous, err := modelBaseURL(model.Endpoint)
				if err != nil || previous != base {
					fail(w, errors.New("接口地址已改变，请重新输入 API Key"))
					return
				}
				key = model.APIKey
				break
			}
		}
	}
	primary := strings.TrimSpace(input.Primary)
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n") || primary == "" || len(primary) > 256 || strings.ContainsAny(primary, "\r\n\x00") {
		fail(w, errors.New("请填写有效的 API Key 并选择主模型"))
		return
	}
	s.writeModelTest(w, r, settings.Model{Endpoint: base, Protocol: protocol, APIKey: key, Primary: primary})
}
