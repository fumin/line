package line

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"text/template"

	"github.com/pkg/errors"

	"github.com/fumin/line/config"
	"github.com/fumin/line/util"
)

//go:embed static
var staticFS embed.FS

const (
	PathLineWebhook  = "/LineWebhook"
	PathAbout        = "/About"
	PathViewData     = "/ViewData"
	PathServeContent = "/ServeContent"
)

type Server struct {
	C config.Config

	Root *os.Root

	Webhook *WebhookHandler

	ServeMux *http.ServeMux
	Server   http.Server
}

func NewServer(cfg config.Config) (*Server, error) {
	s := &Server{C: cfg}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, errors.Wrap(err, "")
	}
	var err error
	s.Root, err = os.OpenRoot(s.C.Dir)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	s.Webhook = &WebhookHandler{
		ChannelSecret: cfg.Secret.LineChannelSecret,
		Names:         NewNameCache(NewClient(cfg.Secret.LineChannelAccessToken)),
		Writer:        NewLogWriter(cfg.Dir),
	}

	s.ServeMux = http.NewServeMux()
	s.Server.Addr = s.C.Addr
	s.Server.Handler = s.ServeMux
	s.ServeMux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	s.ServeMux.Handle(PathLineWebhook, s.Webhook)
	handleFunc(s, "/Login", Login)
	handleFunc(s, PathAbout, About)
	handleFunc(s, PathViewData, ViewData)
	handleFunc(s, PathServeContent, ServeContent)
	handleFunc(s, "/", Index)

	return s, nil
}

func (s *Server) Close() error {
	s.Root.Close()
	return nil
}

func Login(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.FormValue(s.C.Secret.Password) == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sessionID",
		Value:    "admin",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

//go:embed tmpl/viewdata.html
var viewdataHTML string
var viewdataTmpl = template.Must(commonTmpl().Parse(viewdataHTML))

func requireSession(w http.ResponseWriter, r *http.Request) bool {
	if _, err := r.Cookie("sessionID"); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func ViewData(s *Server, w http.ResponseWriter, r *http.Request) {
	if !requireSession(w, r) {
		return
	}

	pwd := r.FormValue("p")
	if pwd == "" {
		pwd = "."
	}

	info, err := s.Root.Stat(pwd)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !info.IsDir() {
		http.Error(w, "not a directory", http.StatusBadRequest)
		return
	}

	f, err := s.Root.Open(pwd)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	sort.Slice(dirEntries, func(i, j int) bool { return dirEntries[i].Name() < dirEntries[j].Name() })

	type viewDataEntry struct {
		Name    string
		Href    string
		IsDir   bool
		SizeStr string
		ModTime string
	}
	entries := make([]viewDataEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		child := path.Join(pwd, de.Name())

		entry := viewDataEntry{Name: html.EscapeString(de.Name()), IsDir: de.IsDir()}
		if de.IsDir() {
			entry.Name += "/"
			entry.Href = viewDataURL(child)
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

	page := struct {
		Navbar     navbar
		PWD        string
		HasParent  bool
		ParentHref string
		Entries    []viewDataEntry
	}{}
	page.Navbar = s.navbar()
	if pwd == "." {
		page.PWD = "/"
	} else {
		page.PWD = html.EscapeString("/" + pwd)
		page.HasParent = true
		page.ParentHref = viewDataURL(path.Dir(pwd))
	}
	page.Entries = entries

	if err := viewdataTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}

func viewDataURL(pwd string) string {
	urlStr := PathViewData
	if pwd != "." && pwd != "" {
		vals := url.Values{}
		vals.Set("p", pwd)
		urlStr += "?" + vals.Encode()
	}
	return urlStr
}

func ServeContent(s *Server, w http.ResponseWriter, r *http.Request) {
	if !requireSession(w, r) {
		return
	}

	fpath := r.FormValue("p")
	info, err := s.Root.Stat(fpath)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	f, err := s.Root.Open(fpath)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
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
	About string
}

func (s *Server) navbar() navbar {
	bar := navbar{}
	bar.About = PathAbout
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
	page.Navbar = s.navbar()
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
		Navbar navbar
	}{}
	page.Navbar = s.navbar()
	if err := indexTmpl.Execute(w, page); err != nil {
		log.Printf("%+v", err)
	}
}
