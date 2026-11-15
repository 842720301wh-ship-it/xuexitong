package learning

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

type courseTransport func(*http.Request) (*http.Response, error)

func (f courseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCourseCredentialEncoding(t *testing.T) {
	for _, value := range []string{"中文密码abc", "1234567890123456"} {
		b, err := base64.StdEncoding.DecodeString(encryptCourseCredential(value))
		if err != nil {
			t.Fatal(err)
		}
		key := []byte("u2oh6Vu^HWe4_AES")
		block, _ := aes.NewCipher(key)
		cipher.NewCBCDecrypter(block, key).CryptBlocks(b, b)
		padding := int(b[len(b)-1])
		if string(b[:len(b)-padding]) != value {
			t.Fatal("credential encoding mismatch")
		}
	}
}

func TestCoursesLoginFoldersAndIDs(t *testing.T) {
	c := NewCourseClient()
	requests := 0
	c.HTTP.Transport = courseTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		body := ""
		switch r.URL.Path {
		case "/fanyalogin":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("uname") != encryptCourseCredential("user") || r.Form.Get("password") != encryptCourseCredential("private-password") {
				t.Fatal("incorrect login parameters")
			}
			body = `{"status":true}`
		case "/mooc2-ans/visit/interaction":
			body = `<ul class="file-list"><li fileid="5"></li><li fileid="5"></li></ul>`
		case "/mooc2-ans/visit/courselistdata":
			_ = r.ParseForm()
			if r.Form.Get("courseFolderId") == "0" {
				body = `<div class="course" id="999"><input class="courseId" value="123"><input class="clazzId" value="456"><span class="course-name" title="工程 &amp; 检测"></span><p class="color3" title="老师"></p></div>`
			} else {
				if r.Form.Get("courseFolderId") != "5" {
					t.Fatal("wrong folder")
				}
				body = `<div class="course"><input class="courseId" value="123"><span class="course-name">重复</span></div><div class="course"><input class="courseId" value="789"><span class="course-name">文件夹课程</span><a class="not-open-tip"></a></div>`
			}
		default:
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	courses, err := c.Courses(context.Background(), " user ", "private-password")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 4 || len(courses) != 2 || courses[0].ID != "123" || courses[0].Name != "工程 & 检测" || courses[0].ClassID != "456" || !courses[1].Unavailable {
		t.Fatalf("wrong courses: %+v (%d requests)", courses, requests)
	}
}

func TestCourseErrorsDoNotExposeCredentials(t *testing.T) {
	for _, response := range []string{`{"status":false,"msg2":"private-password"}`, `{"status":false,"msg2":"请输入验证码"}`, `<html>private-password</html>`} {
		c := NewCourseClient()
		c.HTTP.Transport = courseTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response)), Request: r}, nil
		})
		_, err := c.Courses(context.Background(), "user", "private-password")
		if err == nil || strings.Contains(err.Error(), "private-password") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
	if _, err := courseDocument([]byte(`<script>location.href="https://passport2.chaoxing.com/login"</script>`)); err == nil {
		t.Fatal("login redirect accepted as empty courses")
	}
}
