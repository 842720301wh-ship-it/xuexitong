package learning

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// The authenticated personal-space page exposes the account name in aria-label;
// older pages expose realname or a user-name element. Never use a teacher's name,
// arbitrary page text, or a caller-supplied label as the account's real name.
// References: DanMo661/Xuexitong-mcp (base-page account label),
// SJYssr/CX_Kitty/src/core/auth.js (interaction and personal-space profile).
type AccountData struct {
	Name    string   `json:"name"`
	Courses []Course `json:"courses"`
}

func (c *CourseClient) Account(ctx context.Context, username, password string) (AccountData, error) {
	ctx, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	courses, e := c.Courses(ctx, username, password)
	if e != nil {
		return AccountData{}, e
	}
	if c.Name == "" {
		c.Name = c.nativeName(ctx)
	}
	if c.Name == "" {
		profileCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		if b, e := c.page(profileCtx, "https://i.chaoxing.com/base", nil); e == nil {
			if doc, e := courseDocument(b); e == nil {
				c.Name = profileName(doc)
			}
		}
	}
	return AccountData{Name: c.Name, Courses: courses}, nil
}

// This endpoint is present in the supplied Android APK. It uses the current
// account's authenticated cookie jar; no other user's identifiers are requested.
const nativeProfileEndpoint = "https://sso.chaoxing.com/apis/login/userLogin4Uname.do"
const mobileProfileAgent = "Dalvik/2.1.0 (Linux; U; Android 11) com.chaoxing.mobile/ChaoXingStudy_3_5.1.4_android_phone_614_74"

func (c *CourseClient) nativeName(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, nativeProfileEndpoint, nil)
	if e != nil {
		return ""
	}
	req.Header.Set("User-Agent", mobileProfileAgent)
	req.Header.Set("X-Requested-With", "com.chaoxing.mobile")
	res, e := c.HTTP.Do(req)
	if e != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return ""
	}
	var profile struct {
		Result  int `json:"result"`
		Message struct {
			Name     string `json:"name"`
			RealName string `json:"realname"`
		} `json:"msg"`
	}
	if json.Unmarshal(b, &profile) != nil || profile.Result != 1 {
		return ""
	}
	if name := cleanName(profile.Message.RealName); name != "" {
		return name
	}
	return cleanName(profile.Message.Name)
}

var realNamePattern = regexp.MustCompile(`(?i)\brealname["']?\s*[:=]\s*(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)')`)

func cleanName(value string) string {
	value = strings.TrimSpace(html.UnescapeString(value))
	if value == "" || len([]rune(value)) > 100 || strings.ContainsAny(value, "<>\r\n\x00") {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return ""
		}
	}
	switch value {
	case "未登录", "登录", "学习通用户", "姓名未获取", "超星用户":
		return ""
	}
	return value
}
func profileName(doc *goquery.Document) string {
	var name string
	doc.Find(`[aria-label^="账号："], [aria-label^="账号:"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		value := s.AttrOr("aria-label", "")
		value = strings.TrimPrefix(strings.TrimPrefix(value, "账号："), "账号:")
		name = cleanName(value)
		return name == ""
	})
	if name != "" {
		return name
	}
	doc.Find(`input#realName, input[name="realname"], input[name="realName"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool { name = cleanName(s.AttrOr("value", "")); return name == "" })
	if name != "" {
		return name
	}
	doc.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if m := realNamePattern.FindStringSubmatch(s.Text()); len(m) > 0 {
			raw := m[1]
			if raw == "" {
				raw = m[2]
			}
			decoded := ""
			if json.Unmarshal([]byte(`"`+raw+`"`), &decoded) == nil {
				raw = decoded
			}
			name = cleanName(raw)
		}
		return name == ""
	})
	if name != "" {
		return name
	}
	for _, selector := range []string{"a.user-info", "p.user-name", "span.user-name", "p.username", "span.username"} {
		doc.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) bool { name = cleanName(s.Text()); return name == "" })
		if name != "" {
			return name
		}
	}
	return ""
}
