package line

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/fumin/line/config"
	"github.com/fumin/line/util"
)

//go:embed static
var staticFS embed.FS

const (
	PathLineWebhook  = "/LineWebhook"
	PathAbout        = "/About"
	PathReadDir      = "/ReadDir"
	PathServeContent = "/ServeContent"
	PathLogin        = "/Login"
	PathDoLogin      = "/DoLogin"
	PathLogout       = "/Logout"

	cookieSessionID = "sessionID"
)

type Server struct {
	C config.Config

	Root *os.Root

	sessionMu    sync.RWMutex
	sessionToken string

	Webhook *WebhookHandler

	ServeMux *http.ServeMux
	Server   http.Server
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Wrap(err, "")
	}
	return hex.EncodeToString(b), nil
}

// currentSessionToken returns the token that a valid "sessionID" cookie
// must currently match.
func (s *Server) currentSessionToken() string {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	return s.sessionToken
}

// rotateSessionToken replaces the session token with a fresh random one,
// invalidating every outstanding "sessionID" cookie (used by Logout, and
// at startup).
func (s *Server) rotateSessionToken() error {
	token, err := newSessionToken()
	if err != nil {
		return errors.Wrap(err, "")
	}
	s.sessionMu.Lock()
	s.sessionToken = token
	s.sessionMu.Unlock()
	return nil
}

func NewServer(cfg config.Config) (*Server, error) {
	s := &Server{C: cfg}

	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, errors.Wrap(err, "")
	}
	var err error
	s.Root, err = os.OpenRoot(s.C.Dir)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	if err := s.rotateSessionToken(); err != nil {
		return nil, errors.Wrap(err, "")
	}

	client := NewClient(cfg.Secret.LineChannelAccessToken)
	s.Webhook = &WebhookHandler{
		ChannelSecret: cfg.Secret.LineChannelSecret,
		Client:        client,
		Names:         NewNameCache(client),
		Writer:        NewLogWriter(s.Root),
	}

	s.ServeMux = http.NewServeMux()
	s.Server.Addr = s.C.Addr
	s.Server.Handler = s.ServeMux
	s.Server.ReadHeaderTimeout = 10 * time.Second
	s.Server.ReadTimeout = 30 * time.Second
	s.Server.WriteTimeout = 30 * time.Second
	s.Server.IdleTimeout = 120 * time.Second

	s.ServeMux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	s.ServeMux.Handle(PathLineWebhook, s.Webhook)
	handleFunc(s, PathLogin, Login)
	handleFunc(s, PathDoLogin, DoLogin)
	handleFunc(s, PathLogout, Logout)
	handleFunc(s, PathAbout, About)
	handleFunc(s, PathReadDir, ReadDir)
	handleFunc(s, PathServeContent, ServeContent)
	handleFunc(s, "/", Index)

	return s, nil
}

func (s *Server) Close() error {
	s.Root.Close()
	return nil
}

//go:embed tmpl/login.html
var loginHTML string
var loginTmpl = template.Must(commonTmpl().Parse(loginHTML))

func Login(s *Server, w http.ResponseWriter, r *http.Request) {
	page := struct {
		Navbar      navbar
		PathDoLogin string
	}{
		Navbar:      s.navbar(r),
		PathDoLogin: PathDoLogin,
	}
	if err := loginTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}

func DoLogin(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host != r.Host {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	password := r.FormValue("password")
	if s.C.Secret.Password == "" || subtle.ConstantTimeCompare([]byte(password), []byte(s.C.Secret.Password)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieSessionID,
		Value:    s.currentSessionToken(),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().AddDate(3, 0, 0),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Logout(s *Server, w http.ResponseWriter, r *http.Request) {
	if err := s.rotateSessionToken(); err != nil {
		log.Printf("%+v", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   cookieSessionID,
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// isLoggedIn reports whether r carries a "sessionID" cookie matching the
// current session token.
func (s *Server) isLoggedIn(r *http.Request) bool {
	cookie, err := r.Cookie(cookieSessionID)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.currentSessionToken())) == 1
}

func requireSession(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if !s.isLoggedIn(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

//go:embed tmpl/readdir.html
var readdirHTML string
var readdirTmpl = template.Must(commonTmpl().Parse(readdirHTML))

func ReadDir(s *Server, w http.ResponseWriter, r *http.Request) {
	if !requireSession(s, w, r) {
		return
	}
	pwd := r.FormValue("p")
	if pwd == "" {
		pwd = "."
	}

	dirEntries, err := s.Root.FS().(fs.ReadDirFS).ReadDir(pwd)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	type readDirEntry struct {
		Name    string
		Href    string
		IsDir   bool
		SizeStr string
		ModTime string
	}
	entries := make([]readDirEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		child := path.Join(pwd, de.Name())

		entry := readDirEntry{Name: de.Name(), IsDir: de.IsDir()}
		if de.IsDir() {
			entry.Name += "/"
			entry.Href = readDirURL(child)
			entry.SizeStr = "-"
		} else {
			entry.Href = serveContentURL(child)
			fi, err := de.Info()
			if err != nil {
				continue
			}
			entry.SizeStr = strconv.FormatInt(fi.Size(), 10)
			entry.ModTime = fi.ModTime().In(util.TaipeiTZ).Format("2006-01-02 15:04:05")
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b readDirEntry) int { return -cmp.Compare(a.Name, b.Name) })

	page := struct {
		Navbar     navbar
		PWD        string
		HasParent  bool
		ParentHref string
		Entries    []readDirEntry
	}{}
	page.Navbar = s.navbar(r)
	if pwd == "." {
		page.PWD = "/"
	} else {
		page.PWD = "/" + pwd
		page.HasParent = true
		page.ParentHref = readDirURL(path.Dir(pwd))
	}
	page.Entries = entries

	w.Header().Set("Cache-Control", "no-store")
	if err := readdirTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}

func readDirURL(pwd string) string {
	urlStr := PathReadDir
	if pwd != "." && pwd != "" {
		vals := url.Values{}
		vals.Set("p", pwd)
		urlStr += "?" + vals.Encode()
	}
	return urlStr
}

func markupLog(data []byte) []byte {
	events := make([][]byte, 0)
	ev := make([]byte, 0)
	lines := bytes.SplitSeq(data, []byte{'\n'})
	for l := range lines {
		if hasEventPrefix(l) {
			events = append(events, ev)
			ev = make([]byte, 0)
		}

		ev = append(ev, l...)
		ev = append(ev, []byte("<br>")...)
	}
	events = append(events, ev)

	htmlB := bytes.NewBuffer([]byte("<!DOCTYPE html><html><head><style>body{font-size: xxx-large;}</style></head><body><ul>"))
	for _, ev := range slices.Backward(events) {
		if len(bytes.TrimSpace(ev)) == 0 {
			continue
		}
		htmlB.Write([]byte("<li>"))
		htmlB.Write(util.MakeLinks(ev))
		htmlB.Write([]byte("</li>"))
	}
	htmlB.Write([]byte("</ul></body></html>"))

	return htmlB.Bytes()
}

func ServeContent(s *Server, w http.ResponseWriter, r *http.Request) {
	if !requireSession(s, w, r) {
		return
	}

	fpath := r.FormValue("p")
	info, err := s.Root.Stat(fpath)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "error", http.StatusBadRequest)
		return
	}
	data, err := s.Root.ReadFile(fpath)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	if path.Ext(info.Name()) == ".html" {
		// Enforce CSP since LINE messages originate externally.
		w.Header().Set("Content-Security-Policy", "script-src 'none'; object-src 'none'; frame-src 'none';")
		// Force no-cache, since chat logs are often updated.
		w.Header().Set("Cache-Control", "no-cache")
		data = markupLog(data)
	} else {
		// Images can be cached long term.
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), bytes.NewReader(data))
}

func serveContentURL(fpath string) string {
	vals := url.Values{}
	vals.Set("p", fpath)
	urlStr := PathServeContent + "?" + vals.Encode()
	return urlStr
}

func handleFunc(s *Server, httpPath string, fn func(*Server, http.ResponseWriter, *http.Request)) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fn(s, w, r)
	}
	s.ServeMux.HandleFunc(httpPath, handler)
}

func handleJSON(s *Server, httpPath string, fn func(*Server, http.ResponseWriter, *http.Request) (any, error)) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		res, err := fn(s, w, r)
		if err != nil {
			msg := struct {
				Error struct {
					Msg string
				}
			}{}
			msg.Error.Msg = fmt.Sprintf("%+v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(msg)
			return
		}
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("%+v", err)
			return
		}
	}
	s.ServeMux.HandleFunc(httpPath, handler)
}

//go:embed tmpl/navbar.html
var navbarHTML string
var navbarTmpl = template.Must(template.New("").Parse(navbarHTML))

type navbar struct {
	LoggedIn   bool
	PathLogin  string
	PathLogout string
}

func (s *Server) navbar(r *http.Request) navbar {
	bar := navbar{}
	bar.LoggedIn = s.isLoggedIn(r)
	bar.PathLogin = PathLogin
	bar.PathLogout = PathLogout
	return bar
}

func commonTmpl() *template.Template {
	t := template.Must(navbarTmpl.Clone())
	return t
}

//go:embed tmpl/about.html
var aboutHTML string
var aboutTmpl = template.Must(commonTmpl().Parse(aboutHTML))

func About(s *Server, w http.ResponseWriter, r *http.Request) {
	page := struct {
		Navbar navbar
		Email  string
	}{}
	page.Navbar = s.navbar(r)
	page.Email = "awaw@nandalu.idv.tw"
	if err := aboutTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}

//go:embed tmpl/index.html
var indexHTML string
var indexTmpl = template.Must(commonTmpl().Parse(indexHTML))

func Index(s *Server, w http.ResponseWriter, r *http.Request) {
	page := struct {
		Navbar  navbar
		ReadDir string
		About   string
	}{
		Navbar:  s.navbar(r),
		ReadDir: PathReadDir,
		About:   PathAbout,
	}
	if err := indexTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}
