package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed content.json templates/*.html static/*
var assets embed.FS

const price = 20000

type FW struct{ N, D string }
type Lesson struct{ ID, Title, Explain, Code, Out string }
type Q struct {
	Q    string
	Opts []string
	A    int
	Why  string
}
type Task struct {
	ID, Title, Company, Problem, Task, Starter, Hint string
	Must                                             []string
}
type Link struct{ Label, URL string }
type Lang struct {
	ID, Name, Icon, Color, Tagline, Focus string
	Frameworks, Tools                     []FW
	Roles, Work                           []string
	Lessons                               []Lesson
	Theory                                []Q
	Live, Cases                           []Task
	Jobs                                  []Link
}

type User struct {
	Email, Name, Salt, Hash string
	Prog                    map[string]map[string]int
	Subs                    map[string]int64
}
type Order struct {
	ID, Email, Lang, Status, QR, Img string
	Amount                           int
}
type Store struct {
	mu     sync.Mutex
	Users  map[string]*User
	Sess   map[string]string
	Orders map[string]*Order
}
type Pend struct {
	Mode, Email, Name, Salt, Hash, Code string
	Exp                                 int64
	Tries                               int
}

var (
	db      = &Store{}
	pends   = map[string]*Pend{}
	langs   []Lang
	byID    = map[string]*Lang{}
	tpl     *template.Template
	baseURL = env("BASE_URL", "http://localhost:8080")
	httpc   = &http.Client{Timeout: 20 * time.Second}
	errs    = map[string]string{"google": "Login Google belum diatur di server.", "apple": "Login Apple belum diatur di server.", "state": "Sesi login kedaluwarsa, coba lagi.", "fail": "Login gagal, coba lagi."}
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func tok() string { b := make([]byte, 24); rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) save() {
	b, _ := json.MarshalIndent(s, "", " ")
	os.WriteFile("data.json", b, 0600)
}

var bypass = os.Getenv("BYPASS_LOGIN") == "1"

func L(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		db.mu.Lock()
		defer db.mu.Unlock()
		if bypass && cur(r) == nil {
			e := "tamu-" + tok()[:10] + "@guest.local"
			db.Users[e] = &User{Email: e, Name: env("GUEST_NAME", "Kamu")}
			t := tok()
			db.Sess[t] = e
			db.save()
			setC(w, "sid", t, 86400*365)
			r.AddCookie(&http.Cookie{Name: "sid", Value: t})
		}
		h(w, r)
	}
}
func with(f func()) { db.mu.Lock(); defer db.mu.Unlock(); f() }

func pbkdf2(pw, salt []byte, iter int) []byte {
	mac := hmac.New(sha256.New, pw)
	mac.Write(salt)
	mac.Write([]byte{0, 0, 0, 1})
	u := mac.Sum(nil)
	t := append([]byte{}, u...)
	for i := 1; i < iter; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range t {
			t[j] ^= u[j]
		}
	}
	return t
}
func hashPw(pw, salt string) string {
	return hex.EncodeToString(pbkdf2([]byte(pw), []byte(salt), 60000))
}

func main() {
	var c struct{ Langs []Lang }
	b, _ := assets.ReadFile("content.json")
	if err := json.Unmarshal(b, &c); err != nil {
		log.Fatal(err)
	}
	langs = c.Langs
	for i := range langs {
		byID[langs[i].ID] = &langs[i]
	}
	if b, err := os.ReadFile("data.json"); err == nil {
		json.Unmarshal(b, db)
	}
	if db.Users == nil {
		db.Users = map[string]*User{}
	}
	if db.Sess == nil {
		db.Sess = map[string]string{}
	}
	if db.Orders == nil {
		db.Orders = map[string]*Order{}
	}
	tpl = template.Must(template.New("").Funcs(template.FuncMap{
		"inc":  func(i int) int { return i + 1 },
		"join": strings.Join,
		"date": func(ts int64) string { return time.Unix(ts, 0).Format("02 Jan 2006") },
	}).ParseFS(assets, "templates/*.html"))

	m := http.NewServeMux()
	m.Handle("GET /static/", http.FileServerFS(assets))
	m.HandleFunc("GET /{$}", L(home))
	m.HandleFunc("GET /login", L(loginPage))
	m.HandleFunc("POST /login", L(loginPost))
	m.HandleFunc("GET /register", L(regPage))
	m.HandleFunc("POST /register", L(regPost))
	m.HandleFunc("GET /otp", L(otpPage))
	m.HandleFunc("POST /otp", L(otpPost))
	m.HandleFunc("GET /logout", L(logout))
	m.HandleFunc("GET /auth/google", gStart)
	m.HandleFunc("GET /auth/google/callback", gCB)
	m.HandleFunc("GET /auth/apple", aStart)
	m.HandleFunc("POST /auth/apple/callback", aCB)
	m.HandleFunc("GET /account", L(account))
	m.HandleFunc("GET /l/{lang}", L(langPage))
	m.HandleFunc("GET /l/{lang}/lesson/{id}", L(lessonPage))
	m.HandleFunc("GET /l/{lang}/quiz", L(quizPage))
	m.HandleFunc("GET /l/{lang}/live/{id}", L(taskPage("live")))
	m.HandleFunc("GET /l/{lang}/case/{id}", L(taskPage("case")))
	m.HandleFunc("GET /jobs", L(jobsPage))
	m.HandleFunc("GET /jobs/{lang}", L(jobsLang))
	m.HandleFunc("GET /pay/{lang}", L(payPage))
	m.HandleFunc("POST /api/lesson", L(apiLesson))
	m.HandleFunc("POST /api/quiz", L(apiQuiz))
	m.HandleFunc("POST /api/check", L(apiCheck))
	m.HandleFunc("POST /api/ask", ask)
	m.HandleFunc("POST /api/pay/create", payCreate)
	m.HandleFunc("GET /api/pay/status/{id}", payStatus)
	m.HandleFunc("POST /api/pay/dev", payDev)
	m.HandleFunc("POST /pay/notify", payNotify)
	addr := ":" + env("PORT", "8080")
	log.Println("KodeKu jalan di", baseURL, "(port", addr+")")
	log.Fatal(http.ListenAndServe(addr, m))
}

// ---------- helpers ----------
func cur(r *http.Request) *User {
	for _, c := range r.Cookies() {
		if c.Name == "sid" {
			if u := db.Users[db.Sess[c.Value]]; u != nil {
				return u
			}
		}
	}
	return nil
}
func auth(w http.ResponseWriter, r *http.Request) *User {
	u := cur(r)
	if u == nil {
		http.Redirect(w, r, "/login", 302)
	}
	return u
}
func render(w http.ResponseWriter, r *http.Request, name string, d map[string]any) {
	if d == nil {
		d = map[string]any{}
	}
	d["U"] = cur(r)
	d["Ad"] = template.HTML(env("ADS_HTML", `<div class="adph">Ruang iklan · isi kode iklan lewat env ADS_HTML</div>`))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, name, d); err != nil {
		log.Println(err)
	}
}
func jout(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func jerr(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	jout(w, map[string]any{"ok": false, "msg": msg})
}
func setC(w http.ResponseWriter, n, v string, age int) {
	http.SetCookie(w, &http.Cookie{Name: n, Value: v, Path: "/", MaxAge: age, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(baseURL, "https")})
}
func paid(u *User, lang string) bool { return u.Subs[lang] > time.Now().Unix() }
func setProg(u *User, lang, id string, v int) {
	if u.Prog == nil {
		u.Prog = map[string]map[string]int{}
	}
	if u.Prog[lang] == nil {
		u.Prog[lang] = map[string]int{}
	}
	if v > u.Prog[lang][id] {
		u.Prog[lang][id] = v
		db.save()
	}
}
func ready(u *User, l *Lang) int {
	ids := []string{"theory"}
	for _, x := range l.Lessons {
		ids = append(ids, x.ID)
	}
	for _, x := range l.Live {
		ids = append(ids, x.ID)
	}
	for _, x := range l.Cases {
		ids = append(ids, x.ID)
	}
	s := 0
	for _, id := range ids {
		s += u.Prog[l.ID][id]
	}
	return s / len(ids)
}
func norm(s string) string {
	s = strings.ReplaceAll(s, "'", `"`)
	return strings.Join(strings.Fields(s), "")
}
func startSession(w http.ResponseWriter, email string) {
	t := tok()
	db.Sess[t] = email
	db.save()
	setC(w, "sid", t, 86400*30)
}
func upsert(email, name string) *User {
	email = strings.ToLower(strings.TrimSpace(email))
	u := db.Users[email]
	if u == nil {
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		u = &User{Email: email, Name: name}
		db.Users[email] = u
		db.save()
	}
	return u
}

// ---------- auth ----------
func loginPage(w http.ResponseWriter, r *http.Request) {
	if bypass {
		http.Redirect(w, r, "/", 302)
		return
	}
	render(w, r, "login.html", map[string]any{"Err": errs[r.URL.Query().Get("e")]})
}
func regPage(w http.ResponseWriter, r *http.Request) {
	if bypass {
		http.Redirect(w, r, "/", 302)
		return
	}
	render(w, r, "register.html", nil)
}
func otpCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	n := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return fmt.Sprintf("%06d", n%1000000)
}
func sendOTP(email, code string) {
	log.Printf("OTP %s: %s", email, code)
	h := os.Getenv("SMTP_HOST")
	if h == "" {
		return
	}
	a := smtp.PlainAuth("", os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"), h)
	from := env("SMTP_FROM", os.Getenv("SMTP_USER"))
	msg := "From: KodeKu <" + from + ">\r\nTo: " + email + "\r\nSubject: Kode OTP KodeKu\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nKode OTP kamu: " + code + " (berlaku 10 menit)"
	if err := smtp.SendMail(h+":"+env("SMTP_PORT", "587"), a, from, []string{email}, []byte(msg)); err != nil {
		log.Println("smtp:", err)
	}
}
func startPend(w http.ResponseWriter, p *Pend) {
	now := time.Now().Unix()
	for k, v := range pends {
		if v.Exp < now {
			delete(pends, k)
		}
	}
	p.Code, p.Exp = otpCode(), now+600
	t := tok()
	pends[t] = p
	setC(w, "pend", t, 600)
	go sendOTP(p.Email, p.Code)
}
func regPost(w http.ResponseWriter, r *http.Request) {
	name, email, pw := strings.TrimSpace(r.FormValue("name")), strings.ToLower(strings.TrimSpace(r.FormValue("email"))), r.FormValue("password")
	fail := func(m string) { render(w, r, "register.html", map[string]any{"Err": m}) }
	switch {
	case name == "" || !strings.Contains(email, "@"):
		fail("Isi nama dan email yang valid.")
	case len(pw) < 8:
		fail("Password minimal 8 karakter.")
	case db.Users[email] != nil && db.Users[email].Hash != "":
		fail("Email sudah terdaftar, silakan masuk.")
	default:
		salt := tok()
		startPend(w, &Pend{Mode: "reg", Email: email, Name: name, Salt: salt, Hash: hashPw(pw, salt)})
		http.Redirect(w, r, "/otp", 302)
	}
}
func loginPost(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	u := db.Users[email]
	if u == nil || u.Hash == "" || subtle.ConstantTimeCompare([]byte(hashPw(r.FormValue("password"), u.Salt)), []byte(u.Hash)) != 1 {
		render(w, r, "login.html", map[string]any{"Err": "Email atau password salah."})
		return
	}
	startPend(w, &Pend{Mode: "login", Email: email})
	http.Redirect(w, r, "/otp", 302)
}
func pendOf(r *http.Request) *Pend {
	c, err := r.Cookie("pend")
	if err != nil {
		return nil
	}
	p := pends[c.Value]
	if p == nil || p.Exp < time.Now().Unix() {
		return nil
	}
	return p
}
func otpPage(w http.ResponseWriter, r *http.Request) {
	p := pendOf(r)
	if p == nil {
		http.Redirect(w, r, "/login", 302)
		return
	}
	d := map[string]any{"Email": p.Email}
	if os.Getenv("SMTP_HOST") == "" {
		d["Dev"] = p.Code
	}
	render(w, r, "otp.html", d)
}
func otpPost(w http.ResponseWriter, r *http.Request) {
	p := pendOf(r)
	if p == nil {
		http.Redirect(w, r, "/login", 302)
		return
	}
	p.Tries++
	if p.Tries > 5 || strings.TrimSpace(r.FormValue("code")) != p.Code {
		d := map[string]any{"Email": p.Email, "Err": "Kode salah atau kedaluwarsa."}
		if p.Tries > 5 {
			c, _ := r.Cookie("pend")
			delete(pends, c.Value)
			http.Redirect(w, r, "/login?e=fail", 302)
			return
		}
		render(w, r, "otp.html", d)
		return
	}
	if p.Mode == "reg" {
		db.Users[p.Email] = &User{Email: p.Email, Name: p.Name, Salt: p.Salt, Hash: p.Hash}
	}
	c, _ := r.Cookie("pend")
	delete(pends, c.Value)
	startSession(w, p.Email)
	http.Redirect(w, r, "/", 302)
}
func logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("sid"); err == nil {
		delete(db.Sess, c.Value)
		db.save()
	}
	setC(w, "sid", "", -1)
	http.Redirect(w, r, "/login", 302)
}

func gStart(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("GOOGLE_CLIENT_ID") == "" {
		http.Redirect(w, r, "/login?e=google", 302)
		return
	}
	st := tok()
	setC(w, "gst", st, 600)
	q := url.Values{"client_id": {os.Getenv("GOOGLE_CLIENT_ID")}, "redirect_uri": {baseURL + "/auth/google/callback"}, "response_type": {"code"}, "scope": {"openid email profile"}, "state": {st}}
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+q.Encode(), 302)
}
func gCB(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("gst")
	if err != nil || c.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, "/login?e=state", 302)
		return
	}
	resp, err := httpc.PostForm("https://oauth2.googleapis.com/token", url.Values{"code": {r.URL.Query().Get("code")}, "client_id": {os.Getenv("GOOGLE_CLIENT_ID")}, "client_secret": {os.Getenv("GOOGLE_CLIENT_SECRET")}, "redirect_uri": {baseURL + "/auth/google/callback"}, "grant_type": {"authorization_code"}})
	if err != nil {
		http.Redirect(w, r, "/login?e=fail", 302)
		return
	}
	var t struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&t)
	resp.Body.Close()
	req, _ := http.NewRequest("GET", "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+t.AccessToken)
	resp, err = httpc.Do(req)
	if err != nil {
		http.Redirect(w, r, "/login?e=fail", 302)
		return
	}
	var ui struct{ Email, Name string }
	json.NewDecoder(resp.Body).Decode(&ui)
	resp.Body.Close()
	if ui.Email == "" {
		http.Redirect(w, r, "/login?e=fail", 302)
		return
	}
	with(func() { startSession(w, upsert(ui.Email, ui.Name).Email) })
	http.Redirect(w, r, "/", 302)
}

func appleSecret() (string, error) {
	pk := strings.ReplaceAll(os.Getenv("APPLE_PRIVATE_KEY"), `\n`, "\n")
	blk, _ := pem.Decode([]byte(pk))
	if blk == nil {
		return "", errors.New("apple key")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return "", err
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return "", errors.New("apple key type")
	}
	e := base64.RawURLEncoding
	hb, _ := json.Marshal(map[string]any{"alg": "ES256", "kid": os.Getenv("APPLE_KEY_ID")})
	now := time.Now().Unix()
	cb, _ := json.Marshal(map[string]any{"iss": os.Getenv("APPLE_TEAM_ID"), "iat": now, "exp": now + 300, "aud": "https://appleid.apple.com", "sub": os.Getenv("APPLE_CLIENT_ID")})
	in := e.EncodeToString(hb) + "." + e.EncodeToString(cb)
	h := sha256.Sum256([]byte(in))
	rr, ss, err := ecdsa.Sign(rand.Reader, ec, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	rr.FillBytes(sig[:32])
	ss.FillBytes(sig[32:])
	return in + "." + e.EncodeToString(sig), nil
}
func aStart(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("APPLE_CLIENT_ID") == "" || os.Getenv("APPLE_PRIVATE_KEY") == "" {
		http.Redirect(w, r, "/login?e=apple", 302)
		return
	}
	st := tok()
	http.SetCookie(w, &http.Cookie{Name: "ast", Value: st, Path: "/", MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode})
	q := url.Values{"client_id": {os.Getenv("APPLE_CLIENT_ID")}, "redirect_uri": {baseURL + "/auth/apple/callback"}, "response_type": {"code"}, "response_mode": {"form_post"}, "scope": {"name email"}, "state": {st}}
	http.Redirect(w, r, "https://appleid.apple.com/auth/authorize?"+q.Encode(), 302)
}
func aCB(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("ast")
	if err != nil || c.Value != r.FormValue("state") {
		http.Redirect(w, r, "/login?e=state", 303)
		return
	}
	sec, err := appleSecret()
	if err != nil {
		http.Redirect(w, r, "/login?e=apple", 303)
		return
	}
	resp, err := httpc.PostForm("https://appleid.apple.com/auth/token", url.Values{"client_id": {os.Getenv("APPLE_CLIENT_ID")}, "client_secret": {sec}, "code": {r.FormValue("code")}, "grant_type": {"authorization_code"}, "redirect_uri": {baseURL + "/auth/apple/callback"}})
	if err != nil {
		http.Redirect(w, r, "/login?e=fail", 303)
		return
	}
	var t struct {
		IDToken string `json:"id_token"`
	}
	json.NewDecoder(resp.Body).Decode(&t)
	resp.Body.Close()
	parts := strings.Split(t.IDToken, ".")
	var cl struct{ Email string }
	if len(parts) == 3 {
		if pb, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			json.Unmarshal(pb, &cl)
		}
	}
	if cl.Email == "" {
		http.Redirect(w, r, "/login?e=fail", 303)
		return
	}
	var un struct {
		Name struct{ FirstName, LastName string }
	}
	json.Unmarshal([]byte(r.FormValue("user")), &un)
	name := strings.TrimSpace(un.Name.FirstName + " " + un.Name.LastName)
	with(func() { startSession(w, upsert(cl.Email, name).Email) })
	http.Redirect(w, r, "/", 303)
}

// ---------- pages ----------
func home(w http.ResponseWriter, r *http.Request) {
	u := auth(w, r)
	if u == nil {
		return
	}
	type C struct {
		L    *Lang
		R    int
		Paid bool
	}
	var cs []C
	var top *Lang
	best := 0
	for i := range langs {
		l := &langs[i]
		rd := ready(u, l)
		cs = append(cs, C{l, rd, paid(u, l.ID)})
		if rd > best {
			best, top = rd, l
		}
	}
	render(w, r, "home.html", map[string]any{"Cs": cs, "Top": top, "Best": best, "Tab": "home"})
}
func account(w http.ResponseWriter, r *http.Request) {
	if auth(w, r) == nil {
		return
	}
	render(w, r, "account.html", map[string]any{"Langs": langs, "Tab": "acc", "Now": time.Now().Unix()})
}
func getLang(w http.ResponseWriter, r *http.Request) (*User, *Lang) {
	u := auth(w, r)
	if u == nil {
		return nil, nil
	}
	l := byID[r.PathValue("lang")]
	if l == nil {
		http.NotFound(w, r)
	}
	return u, l
}
func langPage(w http.ResponseWriter, r *http.Request) {
	u, l := getLang(w, r)
	if l == nil {
		return
	}
	render(w, r, "lang.html", map[string]any{"L": l, "P": u.Prog[l.ID], "Paid": paid(u, l.ID), "R": ready(u, l), "Tab": "home"})
}
func lessonPage(w http.ResponseWriter, r *http.Request) {
	u, l := getLang(w, r)
	if l == nil {
		return
	}
	for i := range l.Lessons {
		if l.Lessons[i].ID == r.PathValue("id") {
			if i > 0 && !paid(u, l.ID) {
				http.Redirect(w, r, "/pay/"+l.ID, 302)
				return
			}
			d := map[string]any{"L": l, "Les": l.Lessons[i], "Tab": "home"}
			if i+1 < len(l.Lessons) {
				d["Next"] = l.Lessons[i+1]
			}
			render(w, r, "lesson.html", d)
			return
		}
	}
	http.NotFound(w, r)
}
func quizPage(w http.ResponseWriter, r *http.Request) {
	_, l := getLang(w, r)
	if l == nil {
		return
	}
	render(w, r, "quiz.html", map[string]any{"L": l, "Tab": "home"})
}
func findTask(l *Lang, id string) (*Task, string) {
	for i := range l.Live {
		if l.Live[i].ID == id {
			return &l.Live[i], "live"
		}
	}
	for i := range l.Cases {
		if l.Cases[i].ID == id {
			return &l.Cases[i], "case"
		}
	}
	return nil, ""
}
func taskPage(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, l := getLang(w, r)
		if l == nil {
			return
		}
		t, k := findTask(l, r.PathValue("id"))
		if t == nil || k != kind {
			http.NotFound(w, r)
			return
		}
		if !paid(u, l.ID) {
			http.Redirect(w, r, "/pay/"+l.ID, 302)
			return
		}
		render(w, r, "task.html", map[string]any{"L": l, "T": t, "Kind": kind, "Tab": "home"})
	}
}
func jobsPage(w http.ResponseWriter, r *http.Request) {
	u := auth(w, r)
	if u == nil {
		return
	}
	m := map[string]bool{}
	for _, l := range langs {
		m[l.ID] = paid(u, l.ID)
	}
	render(w, r, "jobs.html", map[string]any{"Langs": langs, "PaidMap": m, "Tab": "jobs"})
}
func jobsLang(w http.ResponseWriter, r *http.Request) {
	u, l := getLang(w, r)
	if l == nil {
		return
	}
	render(w, r, "jobs_lang.html", map[string]any{"L": l, "Paid": paid(u, l.ID), "Tab": "jobs"})
}
func payPage(w http.ResponseWriter, r *http.Request) {
	u, l := getLang(w, r)
	if l == nil {
		return
	}
	render(w, r, "pay.html", map[string]any{"L": l, "Paid": paid(u, l.ID), "Exp": u.Subs[l.ID], "Price": price, "Dev": os.Getenv("MIDTRANS_SERVER_KEY") == "", "Tab": "home"})
}

// ---------- api ----------
func body(r *http.Request, v any) { json.NewDecoder(io.LimitReader(r.Body, 1<<17)).Decode(v) }

func apiLesson(w http.ResponseWriter, r *http.Request) {
	u := cur(r)
	var in struct{ Lang, ID, Code string }
	body(r, &in)
	l := byID[in.Lang]
	if u == nil || l == nil {
		jerr(w, 400, "Permintaan tidak valid")
		return
	}
	for i, x := range l.Lessons {
		if x.ID != in.ID {
			continue
		}
		if i > 0 && !paid(u, l.ID) {
			jerr(w, 402, "Materi ini berbayar")
			return
		}
		a, b := norm(in.Code), norm(x.Code)
		if a == b {
			setProg(u, l.ID, x.ID, 100)
			jout(w, map[string]any{"ok": true, "out": x.Out})
			return
		}
		k := 0
		for k < len(a) && k < len(b) && a[k] == b[k] {
			k++
		}
		end := k + 14
		if end > len(b) {
			end = len(b)
		}
		jout(w, map[string]any{"ok": false, "msg": "Belum pas. Mulai beda sekitar: \"" + b[k:end] + "\" (spasi diabaikan). Cek huruf besar/kecil & tanda baca."})
		return
	}
	jerr(w, 404, "Materi tidak ada")
}
func apiQuiz(w http.ResponseWriter, r *http.Request) {
	u := cur(r)
	var in struct {
		Lang    string
		Answers []int
	}
	body(r, &in)
	l := byID[in.Lang]
	if u == nil || l == nil {
		jerr(w, 400, "Permintaan tidak valid")
		return
	}
	type R struct {
		OK  bool   `json:"ok"`
		Why string `json:"why"`
	}
	var res []R
	ok := 0
	for i, q := range l.Theory {
		c := i < len(in.Answers) && in.Answers[i] == q.A
		if c {
			ok++
		}
		res = append(res, R{c, q.Why})
	}
	sc := ok * 100 / len(l.Theory)
	setProg(u, l.ID, "theory", sc)
	jout(w, map[string]any{"ok": true, "score": sc, "res": res})
}
func apiCheck(w http.ResponseWriter, r *http.Request) {
	u := cur(r)
	var in struct{ Lang, ID, Code string }
	body(r, &in)
	l := byID[in.Lang]
	if u == nil || l == nil {
		jerr(w, 400, "Permintaan tidak valid")
		return
	}
	t, _ := findTask(l, in.ID)
	if t == nil {
		jerr(w, 404, "Soal tidak ada")
		return
	}
	if !paid(u, l.ID) {
		jerr(w, 402, "Soal ini berbayar")
		return
	}
	code := strings.ToLower(norm(in.Code))
	got := 0
	for _, m := range t.Must {
		for _, alt := range strings.Split(m, "|") {
			if strings.Contains(code, strings.ToLower(alt)) {
				got++
				break
			}
		}
	}
	pct := got * 100 / len(t.Must)
	setProg(u, l.ID, t.ID, pct)
	if got == len(t.Must) {
		jout(w, map[string]any{"ok": true, "msg": "Mantap! Solusimu memenuhi semua syarat studi kasus."})
		return
	}
	jout(w, map[string]any{"ok": false, "msg": fmt.Sprintf("Baru %d dari %d syarat terpenuhi. Petunjuk: %s", got, len(t.Must), t.Hint)})
}

func ask(w http.ResponseWriter, r *http.Request) {
	var in struct{ Lang, Topic, Q string }
	body(r, &in)
	var name string
	with(func() {
		if u := cur(r); u != nil {
			name = u.Name
		}
	})
	if name == "" {
		jerr(w, 401, "Masuk dulu ya")
		return
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		jout(w, map[string]any{"ok": true, "a": "AI belum aktif. Admin perlu mengisi ANTHROPIC_API_KEY di server."})
		return
	}
	cut := func(s string, n int) string {
		if rs := []rune(s); len(rs) > n {
			return string(rs[:n])
		}
		return s
	}
	sys := fmt.Sprintf("Kamu tutor ngoding ramah di aplikasi KodeKu. Panggil pengguna dengan nama %q. Jawab dalam bahasa Indonesia santai, maksimal 5 kalimat, jelaskan istilah dengan contoh kecil bila perlu. Konteks materi: bahasa %s, topik %s.", cut(name, 40), cut(in.Lang, 20), cut(in.Topic, 80))
	b, _ := json.Marshal(map[string]any{"model": env("ANTHROPIC_MODEL", "claude-sonnet-5-5"), "max_tokens": 500, "system": sys, "messages": []map[string]string{{"role": "user", "content": cut(in.Q, 800)}}})
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		jerr(w, 502, "AI sedang tidak bisa dihubungi")
		return
	}
	defer resp.Body.Close()
	var out struct {
		Content []struct{ Text string }
	}
	json.NewDecoder(resp.Body).Decode(&out)
	a := ""
	for _, c := range out.Content {
		a += c.Text
	}
	if a == "" {
		a = "Maaf, AI belum bisa menjawab sekarang. Coba lagi ya."
	}
	jout(w, map[string]any{"ok": true, "a": a})
}

// ---------- payment (Midtrans QRIS) ----------
func midReq(method, path string, body any) (map[string]any, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, env("MIDTRANS_BASE", "https://api.sandbox.midtrans.com")+path, rd)
	req.SetBasicAuth(os.Getenv("MIDTRANS_SERVER_KEY"), "")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var m map[string]any
	err = json.NewDecoder(resp.Body).Decode(&m)
	return m, err
}
func activate(o *Order) {
	if o == nil || o.Status == "paid" {
		return
	}
	o.Status = "paid"
	if u := db.Users[o.Email]; u != nil {
		if u.Subs == nil {
			u.Subs = map[string]int64{}
		}
		base := time.Now().Unix()
		if u.Subs[o.Lang] > base {
			base = u.Subs[o.Lang]
		}
		u.Subs[o.Lang] = base + 30*86400
	}
	db.save()
}
func payCreate(w http.ResponseWriter, r *http.Request) {
	var in struct{ Lang string }
	body(r, &in)
	var email string
	with(func() {
		if u := cur(r); u != nil {
			email = u.Email
		}
	})
	if email == "" || byID[in.Lang] == nil {
		jerr(w, 400, "Permintaan tidak valid")
		return
	}
	o := &Order{ID: "KK-" + tok()[:14], Email: email, Lang: in.Lang, Status: "pending", Amount: price}
	live := os.Getenv("MIDTRANS_SERVER_KEY") != ""
	if live {
		m, err := midReq("POST", "/v2/charge", map[string]any{"payment_type": "qris", "transaction_details": map[string]any{"order_id": o.ID, "gross_amount": price}, "qris": map[string]any{"acquirer": "gopay"}})
		if err != nil || fmt.Sprint(m["status_code"]) != "201" {
			jerr(w, 502, "Payment gateway bermasalah")
			return
		}
		o.QR, _ = m["qr_string"].(string)
		if as, ok := m["actions"].([]any); ok {
			for _, a := range as {
				if am, ok := a.(map[string]any); ok && am["name"] == "generate-qr-code" {
					o.Img, _ = am["url"].(string)
				}
			}
		}
	} else {
		o.QR = "KODEKU-DEV-" + o.ID
	}
	with(func() { db.Orders[o.ID] = o; db.save() })
	jout(w, map[string]any{"ok": true, "id": o.ID, "qr": o.QR, "img": o.Img, "dev": !live})
}
func payStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var email, status string
	with(func() {
		if u := cur(r); u != nil {
			email = u.Email
		}
		if o := db.Orders[id]; o != nil && o.Email == email {
			status = o.Status
		}
	})
	if status == "" {
		jerr(w, 404, "Order tidak ada")
		return
	}
	if status == "pending" && os.Getenv("MIDTRANS_SERVER_KEY") != "" {
		if m, err := midReq("GET", "/v2/"+id+"/status", nil); err == nil {
			if ts := fmt.Sprint(m["transaction_status"]); ts == "settlement" || ts == "capture" {
				with(func() { activate(db.Orders[id]) })
				status = "paid"
			}
		}
	}
	jout(w, map[string]any{"ok": true, "status": status})
}
func payDev(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID string }
	body(r, &in)
	if os.Getenv("MIDTRANS_SERVER_KEY") != "" {
		jerr(w, 403, "Hanya mode dev")
		return
	}
	with(func() {
		u := cur(r)
		if o := db.Orders[in.ID]; u != nil && o != nil && o.Email == u.Email {
			activate(o)
		}
	})
	jout(w, map[string]any{"ok": true})
}
func payNotify(w http.ResponseWriter, r *http.Request) {
	var n struct {
		OrderID string `json:"order_id"`
		Code    string `json:"status_code"`
		Gross   string `json:"gross_amount"`
		Sig     string `json:"signature_key"`
		Status  string `json:"transaction_status"`
	}
	body(r, &n)
	key := os.Getenv("MIDTRANS_SERVER_KEY")
	s := sha512.Sum512([]byte(n.OrderID + n.Code + n.Gross + key))
	if key == "" || subtle.ConstantTimeCompare([]byte(hex.EncodeToString(s[:])), []byte(n.Sig)) != 1 {
		http.Error(w, "bad signature", 403)
		return
	}
	if n.Status == "settlement" || n.Status == "capture" {
		with(func() { activate(db.Orders[n.OrderID]) })
	}
	w.WriteHeader(200)
}
