package learning

// Protocol references: SuperStar/api/{base,cipher,config,decode}.py.
// Course lookup runs locally with an isolated, short-lived cookie jar.
import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
)

type Course struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Teacher     string `json:"teacher,omitempty"`
	ClassID     string `json:"class_id,omitempty"`
	Unavailable bool   `json:"unavailable,omitempty"`
}

type CourseClient struct {
	HTTP *http.Client
	Name string
}

func NewCourseClient() *CourseClient {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return &CourseClient{HTTP: &http.Client{Jar: jar, Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || r.URL.Scheme != "https" || (r.URL.Hostname() != "chaoxing.com" && !strings.HasSuffix(r.URL.Hostname(), ".chaoxing.com")) {
			return errors.New("学习平台重定向无效")
		}
		return nil
	}}}
}

func encryptCourseCredential(value string) string {
	key := []byte("u2oh6Vu^HWe4_AES")
	block, _ := aes.NewCipher(key)
	plain := []byte(value)
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	for i := 0; i < padding; i++ {
		plain = append(plain, byte(padding))
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key).CryptBlocks(out, plain)
	return base64.StdEncoding.EncodeToString(out)
}

func (c *CourseClient) page(ctx context.Context, endpoint string, form url.Values) ([]byte, error) {
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.New("无法创建课程请求")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if strings.Contains(endpoint, "mooc2-ans") {
		req.Header.Set("Referer", "https://mooc2-ans.chaoxing.com/mooc2-ans/visit/interaction?moocDomain=https://mooc1-1.chaoxing.com/mooc-ans")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("无法连接学习平台或请求超时，请检查网络后重试")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("学习平台暂时无法读取课程，请稍后重试")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil || len(b) > 4<<20 {
		return nil, errors.New("学习平台响应异常，请重试")
	}
	return b, nil
}

var numericCourseID = regexp.MustCompile(`^[0-9]+$`)

func courseDocument(b []byte) (*goquery.Document, error) {
	s := string(b)
	if strings.Contains(s, "fanyalogin") || strings.Contains(s, "验证码") || strings.Contains(s, "访问过于频繁") || regexp.MustCompile(`(?i)(?:location[^;\n]*|url\s*=\s*)https?://passport2\.chaoxing\.com`).MatchString(s) {
		return nil, errors.New("学习平台需要重新登录或验证码验证，请先在学习通完成验证后重试")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		return nil, errors.New("无法解析课程列表")
	}
	return doc, nil
}

func parseCourses(doc *goquery.Document) ([]Course, error) {
	courses := []Course{}
	invalid := false
	doc.Find("div.course").Each(func(_ int, item *goquery.Selection) {
		id := item.Find("input.courseId").AttrOr("value", "")
		name := item.Find("span.course-name")
		title := strings.TrimSpace(name.AttrOr("title", name.Text()))
		if !numericCourseID.MatchString(id) || title == "" {
			invalid = true
			return
		}
		teacher := item.Find("p.color3")
		courses = append(courses, Course{ID: id, Name: title, Teacher: strings.TrimSpace(teacher.AttrOr("title", teacher.Text())), ClassID: item.Find("input.clazzId").AttrOr("value", ""), Unavailable: item.Find(".not-open-tip").Length() > 0})
	})
	if invalid {
		return nil, errors.New("学习平台课程格式已变化，请手动填写课程 ID")
	}
	return courses, nil
}

func (c *CourseClient) Courses(ctx context.Context, username, password string) ([]Course, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("请先填写学习账号和登录密码")
	}
	for _, value := range []string{username, password} {
		if len(value) > 4096 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, errors.New("账号或密码格式无效")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	b, err := c.page(ctx, "https://passport2.chaoxing.com/fanyalogin", url.Values{"fid": {"-1"}, "uname": {encryptCourseCredential(username)}, "password": {encryptCourseCredential(password)}, "refer": {"https%3A%2F%2Fi.chaoxing.com"}, "t": {"true"}, "forbidotherlogin": {"0"}, "validate": {""}, "doubleFactorLogin": {"0"}, "independentId": {"0"}})
	if err != nil {
		return nil, err
	}
	var login struct {
		Status  bool   `json:"status"`
		Message string `json:"msg2"`
	}
	if json.Unmarshal(b, &login) != nil {
		return nil, errors.New("学习平台登录响应异常，请稍后重试")
	}
	if !login.Status {
		if strings.Contains(login.Message, "验证码") || strings.Contains(login.Message, "验证") {
			return nil, errors.New("学习平台要求验证码或安全验证，请先在学习通完成验证后重试")
		}
		return nil, errors.New("学习平台登录失败，请检查账号密码；如需安全验证，请先在学习通完成验证")
	}
	const base = "https://mooc2-ans.chaoxing.com/mooc2-ans/visit/"
	readFolder := func(id string) (*goquery.Document, error) {
		b, e := c.page(ctx, base+"courselistdata", url.Values{"courseType": {"1"}, "courseFolderId": {id}, "query": {""}, "superstarClass": {"0"}})
		if e != nil {
			return nil, e
		}
		return courseDocument(b)
	}
	doc, err := readFolder("0")
	if err != nil {
		return nil, err
	}
	courses, err := parseCourses(doc)
	if err != nil {
		return nil, err
	}
	b, err = c.page(ctx, base+"interaction", nil)
	if err != nil {
		return nil, err
	}
	doc, err = courseDocument(b)
	if err != nil {
		return nil, err
	}
	c.Name = profileName(doc)
	folders := []string{}
	seenFolders := map[string]bool{"0": true}
	doc.Find("ul.file-list > li[fileid]").Each(func(_ int, item *goquery.Selection) {
		id := item.AttrOr("fileid", "")
		if numericCourseID.MatchString(id) && !seenFolders[id] {
			folders = append(folders, id)
			seenFolders[id] = true
		}
	})
	if len(folders) > 100 {
		return nil, errors.New("课程文件夹过多，请手动填写课程 ID")
	}
	for _, id := range folders {
		doc, err = readFolder(id)
		if err != nil {
			return nil, err
		}
		items, e := parseCourses(doc)
		if e != nil {
			return nil, e
		}
		courses = append(courses, items...)
	}
	// The runner selects courseId, rather than the card's DOM id or clazzId.
	result := []Course{}
	seen := map[string]int{}
	for _, course := range courses {
		if i, ok := seen[course.ID]; ok {
			if result[i].Unavailable && !course.Unavailable {
				result[i] = course
			}
			continue
		}
		seen[course.ID] = len(result)
		result = append(result, course)
	}
	return result, nil
}
