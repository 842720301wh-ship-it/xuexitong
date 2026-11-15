package learning

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPlatformNameParsing(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`<a aria-label="账号：张三">账号</a>`, "张三"},
		{`<a aria-label="账号: 李四">账号</a>`, "李四"},
		{`<input id="realName" value="王五">`, "王五"},
		{`<script>var realname = "\u8d75\u516d";</script>`, "赵六"},
		{`<script>const profile={"realName":"小明"}</script>`, "小明"},
		{`<script>var realname = '阿依古丽';</script>`, "阿依古丽"},
		{`<a class="user-info">李同学</a>`, "李同学"},
		{`<span class="user-name">真实姓名</span>`, "真实姓名"},
		{`<p class="color3">教师姓名</p><span>登录</span>`, ""},
		{`<input id="username" value="13811112222"><p>任意备注</p>`, ""},
		{`<input id="realName" value="&lt;script&gt;">`, ""},
	} {
		doc, e := goquery.NewDocumentFromReader(strings.NewReader(tc.body))
		if e != nil {
			t.Fatal(e)
		}
		if got := profileName(doc); got != tc.want {
			t.Errorf("profileName(%s)=%q want %q", tc.body, got, tc.want)
		}
	}
}
func TestAccountNameUsesAuthenticatedSession(t *testing.T) {
	c := NewCourseClient()
	profileRequested := false
	c.HTTP.Transport = courseTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/fanyalogin":
			body = `{"status":true}`
		case "/mooc2-ans/visit/courselistdata":
			body = `<div></div>`
		case "/mooc2-ans/visit/interaction":
			body = `<ul class="file-list"></ul>`
		case "/base":
			profileRequested = true
			body = `<a aria-label="账号：平台真实姓名">账号</a>`
		case "/apis/login/userLogin4Uname.do":
			body = `{"result":0}`
		default:
			t.Fatalf("unexpected %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	result, e := c.Account(context.Background(), "account", "password")
	if e != nil || result.Name != "平台真实姓名" || !profileRequested {
		t.Fatalf("profile not retrieved: %+v %v", result, e)
	}
}

func TestNativeProfileCookieAndName(t *testing.T) {
	c := NewCourseClient()
	c.HTTP.Transport = courseTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		headers := http.Header{}
		switch r.URL.Path {
		case "/fanyalogin":
			body = `{"status":true}`
			headers.Set("Set-Cookie", "_uid=own-account; Domain=.chaoxing.com; Path=/; Secure")
		case "/mooc2-ans/visit/courselistdata", "/mooc2-ans/visit/interaction":
			body = `<div></div>`
		case "/apis/login/userLogin4Uname.do":
			if !strings.Contains(r.Header.Get("Cookie"), "_uid=own-account") {
				t.Fatal("SSO request did not reuse authenticated session")
			}
			if r.Method != "GET" || r.URL.RawQuery != "" || r.Header.Get("X-Requested-With") != "com.chaoxing.mobile" {
				t.Fatal("incorrect native profile request")
			}
			body = `{"result":1,"msg":{"name":"平台实名"}}`
		default:
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	result, e := c.Account(context.Background(), "account", "password")
	if e != nil || result.Name != "平台实名" {
		t.Fatalf("native profile: %+v %v", result, e)
	}
}

func TestNativeProfileRejectsUnauthenticatedName(t *testing.T) {
	c := NewCourseClient()
	c.HTTP.Transport = courseTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"result":0,"msg":{"name":"不能采信的名字"}}`)), Request: r}, nil
	})
	if c.nativeName(context.Background()) != "" {
		t.Fatal("unauthenticated response used as account name")
	}
}
