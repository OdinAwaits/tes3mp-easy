package panel

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const panelTimeout = 20 * time.Second

var (
	listenPattern = regexp.MustCompile(`(?m)^.*\b(http|https) listening on ([^\s]+)$`)
	csrfPattern   = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)
)

type panelProc struct {
	cmd      *exec.Cmd
	root     string
	logPath  string
	modePath string
	logFile  *os.File

	httpAddr  string
	httpsAddr string

	done     chan struct{}
	waitErr  error
	stopOnce sync.Once
}

type panelResponse struct {
	Status  int
	Header  http.Header
	Body    string
	Cookies []*http.Cookie
}

type panelSession struct {
	Client           *http.Client
	CSRF             string
	NewPassword      string
	LoginResponse    panelResponse
	PasswordResponse panelResponse
}

// startPanel launches the real panel binary on ephemeral loopback ports and
// waits until both listeners report their addresses in the log.
func startPanel(t *testing.T, extraEnv map[string]string) *panelProc {
	t.Helper()

	root := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "panel.log")
	modePath := filepath.Join(t.TempDir(), "ops-mode")
	if err := os.WriteFile(modePath, []byte("ok\n"), 0o600); err != nil {
		t.Fatalf("write fake mode: %v", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("create panel log: %v", err)
	}

	httpPort, httpsPort := reservePorts(t)
	cmd := exec.Command(testPanelBinary,
		"--root", root,
		"--http-addr", fmt.Sprintf("127.0.0.1:%d", httpPort),
		"--https-addr", fmt.Sprintf("127.0.0.1:%d", httpsPort),
		"--public-address", fmt.Sprintf("127.0.0.1:%d", httpsPort),
	)
	cmd.Env = panelEnvironment(map[string]string{
		"FAKE_OPS_LOG":       filepath.Join(t.TempDir(), "ops.log"),
		"FAKE_OPS_MODE_FILE": modePath,
	}, extraEnv)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start panel: %v", err)
	}

	p := &panelProc{
		cmd:      cmd,
		root:     root,
		logPath:  logPath,
		modePath: modePath,
		logFile:  logFile,
		done:     make(chan struct{}),
	}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.done)
	}()
	t.Cleanup(func() {
		p.stop()
		_ = p.logFile.Close()
	})

	p.httpAddr, p.httpsAddr = p.waitForListeners(t)
	return p
}

func panelEnvironment(extra ...map[string]string) []string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	values["GOMAXPROCS"] = "1"
	values["TZ"] = "UTC"
	if path := values["PATH"]; path != "" {
		values["PATH"] = testFakeBin + string(os.PathListSeparator) + path
	} else {
		values["PATH"] = testFakeBin
	}
	for _, set := range extra {
		for key, value := range set {
			values[key] = value
		}
	}

	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func reservePorts(t *testing.T) (int, int) {
	t.Helper()

	first, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP port: %v", err)
	}
	second, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = first.Close()
		t.Fatalf("reserve HTTPS port: %v", err)
	}
	httpPort := first.Addr().(*net.TCPAddr).Port
	httpsPort := second.Addr().(*net.TCPAddr).Port
	if err := first.Close(); err != nil {
		_ = second.Close()
		t.Fatalf("release HTTP port: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("release HTTPS port: %v", err)
	}
	return httpPort, httpsPort
}

func (p *panelProc) waitForListeners(t *testing.T) (httpAddr, httpsAddr string) {
	t.Helper()

	deadline := time.Now().Add(panelTimeout)
	for {
		data, _ := os.ReadFile(p.logPath)
		for _, match := range listenPattern.FindAllStringSubmatch(string(data), -1) {
			switch match[1] {
			case "http":
				httpAddr = match[2]
			case "https":
				httpsAddr = match[2]
			}
		}
		if httpAddr != "" && httpsAddr != "" {
			return httpAddr, httpsAddr
		}
		select {
		case <-p.done:
			t.Fatalf("panel exited before listening: %v\n%s", p.waitErr, data)
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("panel did not start listening within %s\n%s",
				panelTimeout, data)
		}
	}
}

func (p *panelProc) stop() {
	p.stopOnce.Do(func() {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-p.done:
		case <-time.After(panelTimeout):
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
}

func (p *panelProc) logContents() string {
	data, _ := os.ReadFile(p.logPath)
	return string(data)
}

func (p *panelProc) opsLogPath() string {
	for _, entry := range p.cmd.Env {
		if value, ok := strings.CutPrefix(entry, "FAKE_OPS_LOG="); ok {
			return value
		}
	}
	return ""
}

func (p *panelProc) httpURL(path string) string {
	return "http://" + p.httpAddr + path
}

func (p *panelProc) httpsURL(path string) string {
	return "https://" + p.httpsAddr + path
}

func (p *panelProc) newClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	transport := &http.Transport{
		// The panel creates a self-signed certificate on first start.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402
	}
	return &http.Client{
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (p *panelProc) request(t *testing.T, client *http.Client, method, path string,
	form url.Values, header http.Header,
) panelResponse {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, p.httpsURL(path), body)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	return panelResponse{
		Status:  resp.StatusCode,
		Header:  resp.Header.Clone(),
		Body:    string(data),
		Cookies: resp.Cookies(),
	}
}

func readLogFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(data)
}

func (p *panelProc) initialPassword(t *testing.T) string {
	t.Helper()

	path := filepath.Join(p.root, "panel-data", "initial-password.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initial password: %v", err)
	}
	password := strings.TrimSpace(string(data))
	if password == "" {
		t.Fatal("initial password is empty")
	}
	return password
}

func requireStatus(t *testing.T, resp panelResponse, want int, context string) {
	t.Helper()
	if resp.Status != want {
		t.Fatalf("%s = %d, want %d; body=%s", context, resp.Status, want, resp.Body)
	}
}

func requireLocation(t *testing.T, resp panelResponse, want string) {
	t.Helper()
	if got := resp.Header.Get("Location"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func formToken(t *testing.T, body string) string {
	t.Helper()
	match := csrfPattern.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("page has no CSRF token:\n%s", body)
	}
	return match[1]
}

// loginPanel performs the forced first-login password change and returns an
// authenticated client with the rotated session CSRF token.
func loginPanel(t *testing.T, p *panelProc, newPassword string) *panelSession {
	t.Helper()

	client := p.newClient()
	resp := p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusSeeOther, "GET /admin before login")
	requireLocation(t, resp, "/admin/login")

	resp = p.request(t, client, http.MethodGet, "/admin/login", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/login")
	loginCSRF := formToken(t, resp.Body)

	resp = p.request(t, client, http.MethodPost, "/admin/login", url.Values{
		"_csrf":    {loginCSRF},
		"password": {p.initialPassword(t)},
	}, nil)
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/login")
	requireLocation(t, resp, "/admin/password")
	loginResp := resp

	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusSeeOther, "GET /admin before password change")
	requireLocation(t, resp, "/admin/password")

	resp = p.request(t, client, http.MethodGet, "/admin/password", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin/password")
	passwordCSRF := formToken(t, resp.Body)

	resp = p.request(t, client, http.MethodPost, "/admin/password", url.Values{
		"_csrf":            {passwordCSRF},
		"current_password": {p.initialPassword(t)},
		"new_password":     {newPassword},
		"confirm_password": {newPassword},
	}, nil)
	requireStatus(t, resp, http.StatusSeeOther, "POST /admin/password")
	requireLocation(t, resp, "/admin")
	passwordResp := resp

	resp = p.request(t, client, http.MethodGet, "/admin", nil, nil)
	requireStatus(t, resp, http.StatusOK, "GET /admin after password change")

	return &panelSession{
		Client:           client,
		CSRF:             formToken(t, resp.Body),
		NewPassword:      newPassword,
		LoginResponse:    loginResp,
		PasswordResponse: passwordResp,
	}
}

func expectJSON(t *testing.T, resp panelResponse, status int) {
	t.Helper()
	requireStatus(t, resp, status, "JSON response")
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(
		contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if cache := resp.Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cache)
	}
}

func requireContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("body does not contain %q:\n%s", want, body)
	}
}

func requireNotContains(t *testing.T, body, want string) {
	t.Helper()
	if strings.Contains(body, want) {
		t.Fatalf("body unexpectedly contains %q:\n%s", want, body)
	}
}

func requireBody(t *testing.T, resp panelResponse, want string) {
	t.Helper()
	if resp.Body != want {
		t.Fatalf("body = %q, want %q", resp.Body, want)
	}
}
