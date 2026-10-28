package main

// Port of server/fastify-common.ts: the /api/* routes, served in-process
// through the Wails asset server so the React app talks to them same-origin.

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Same default secret as the Fastify server, so tokens issued by the
// Electron build stay valid after migrating config.json.
func jwtSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("your-super-secret-jwt-key-change-this-in-production")
}

type JWTUser struct {
	Email                   string `json:"email"`
	StorytelToken           string `json:"storytelToken"`
	JWT                     string `json:"jwt"`
	EncryptedPassword       string `json:"encryptedPassword,omitempty"`
	SsoStorytelSession      string `json:"ssoStorytelSession,omitempty"`
	SsoFirebaseRefreshToken string `json:"ssoFirebaseRefreshToken,omitempty"`
	SsoFirebaseAPIKey       string `json:"ssoFirebaseApiKey,omitempty"`
	SsoCid                  string `json:"ssoCid,omitempty"`
	Iat                     int64  `json:"iat,omitempty"`
}

var b64 = base64.RawURLEncoding

func signJWT(u JWTUser) string {
	u.Iat = time.Now().Unix()
	header := b64.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(u)
	signing := header + "." + b64.EncodeToString(payload)
	mac := hmac.New(sha256.New, jwtSecret())
	mac.Write([]byte(signing))
	return signing + "." + b64.EncodeToString(mac.Sum(nil))
}

func verifyJWT(token string) (*JWTUser, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	mac := hmac.New(sha256.New, jwtSecret())
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("bad signature")
	}
	raw, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var u JWTUser
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, err
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	_ = json.Unmarshal(raw, &claims)
	if claims.Exp != 0 && claims.Exp < time.Now().Unix() {
		return nil, errors.New("token expired")
	}
	return &u, nil
}

func hydrateClient(u *JWTUser) *StorytelClient {
	c := &StorytelClient{}
	if u.SsoStorytelSession != "" && u.SsoFirebaseRefreshToken != "" && u.SsoFirebaseAPIKey != "" {
		c.LoginViaSso(SsoSession{u.SsoStorytelSession, u.SsoFirebaseRefreshToken, u.SsoFirebaseAPIKey, u.Email, u.SsoCid})
	} else {
		c.singleSignToken, c.jwt = u.StorytelToken, u.JWT
		if u.Email != "" && u.EncryptedPassword != "" {
			c.SetCredentials(u.Email, u.EncryptedPassword)
		}
	}
	return c
}

func sendJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func sendError(w http.ResponseWriter, err error) {
	if isUnauthorized(err) {
		sendJSON(w, 401, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, statusOf(err), map[string]any{"error": err.Error()})
}

func readBody(r *http.Request, v any) {
	_ = json.NewDecoder(r.Body).Decode(v)
}

var bookIDPattern = regexp.MustCompile(`^\d+$`)
var unsafeTitleChars = regexp.MustCompile(`[<>:"/\\|?*#%\x00-\x1f]`)
var whitespace = regexp.MustCompile(`\s+`)
var trailingDotsSpaces = regexp.MustCompile(`[. ]+$`)

const maxTitleLength = 120

func sanitizeFileTitle(title string) string {
	t := strings.TrimSpace(whitespace.ReplaceAllString(unsafeTitleChars.ReplaceAllString(title, ""), " "))
	if r := []rune(t); len(r) > maxTitleLength {
		t = string(r[:maxTitleLength])
	}
	return trailingDotsSpaces.ReplaceAllString(t, "") // Windows rejects trailing dots and spaces
}

type download struct {
	cancel   func()
	filePath string
}

type API struct {
	downloadsDir string
	mu           sync.Mutex
	downloads    map[string]*download
}

func NewAPI(downloadsDir string) *API {
	_ = os.MkdirAll(downloadsDir, 0o755)
	return &API{downloadsDir: downloadsDir, downloads: map[string]*download{}}
}

// Downloaded files are named "<title> [<bookId>].mp3"; older ones "<bookId>.mp3".
func (a *API) buildDownloadPath(bookID, title string) string {
	if safe := sanitizeFileTitle(title); safe != "" {
		return filepath.Join(a.downloadsDir, fmt.Sprintf("%s [%s].mp3", safe, bookID))
	}
	return filepath.Join(a.downloadsDir, bookID+".mp3")
}

func (a *API) findDownloadedFile(bookID string) string {
	if !bookIDPattern.MatchString(bookID) {
		return ""
	}
	legacy := filepath.Join(a.downloadsDir, bookID+".mp3")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	entries, _ := os.ReadDir(a.downloadsDir)
	suffix := " [" + bookID + "].mp3"
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			return filepath.Join(a.downloadsDir, e.Name())
		}
	}
	return ""
}

type authedHandler func(w http.ResponseWriter, r *http.Request, u *JWTUser)

func authenticate(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		u, err := verifyJWT(token)
		if err != nil {
			sendJSON(w, 401, map[string]any{"error": "Not authenticated"})
			return
		}
		h(w, r, u)
	}
}

func newDeviceID() string {
	if id := os.Getenv("DEVICE_ID"); id != "" {
		return id
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Email, Password string }
		readBody(r, &body)
		if body.Email == "" || body.Password == "" {
			sendJSON(w, 400, map[string]any{"error": "Email and password required"})
			return
		}
		c := &StorytelClient{}
		if err := c.Login(body.Email, body.Password); err != nil {
			sendError(w, err)
			return
		}
		token := signJWT(JWTUser{
			StorytelToken: c.singleSignToken, JWT: c.jwt, Email: body.Email,
			EncryptedPassword: encryptPassword(strings.TrimSpace(body.Password)),
		})
		sendJSON(w, 200, map[string]any{"success": true, "message": "Login successful", "token": token})
	})

	mux.HandleFunc("POST /api/sso-login", func(w http.ResponseWriter, r *http.Request) {
		var b SsoSession
		readBody(r, &b)
		if b.StorytelSession == "" || b.FirebaseRefreshToken == "" || b.FirebaseAPIKey == "" {
			sendJSON(w, 400, map[string]any{"error": "Missing SSO credentials"})
			return
		}
		token := signJWT(JWTUser{
			Email: b.Email, SsoStorytelSession: b.StorytelSession,
			SsoFirebaseRefreshToken: b.FirebaseRefreshToken, SsoFirebaseAPIKey: b.FirebaseAPIKey, SsoCid: b.Cid,
		})
		sendJSON(w, 200, map[string]any{"success": true, "message": "SSO login successful", "token": token})
	})

	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, map[string]any{"success": true, "message": "Logged out successfully"})
	})

	mux.HandleFunc("GET /api/bookshelf", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		shelf, err := hydrateClient(u).GetBookshelf()
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, shelf)
	}))

	mux.HandleFunc("GET /api/search", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		q := r.URL.Query().Get("q")
		if q == "" {
			q = r.URL.Query().Get("query")
		}
		results, err := hydrateClient(u).SearchCatalog(q)
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"results": results})
	}))

	mux.HandleFunc("POST /api/bookshelf/add", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct{ ConsumableID any `json:"consumableId"` }
		readBody(r, &b)
		id := str(b.ConsumableID)
		if id == "" {
			sendJSON(w, 400, map[string]any{"error": "Missing consumableId"})
			return
		}
		added, strategy, err := hydrateClient(u).AddToBookshelf(id)
		if err != nil {
			sendError(w, err)
			return
		}
		if !added {
			sendJSON(w, 502, map[string]any{"code": "ADD_TO_BOOKSHELF_NOT_CONFIRMED", "error": "Storytel accepted the request but the book is not on the bookshelf"})
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "result": map[string]any{"added": true, "strategy": strategy}})
	}))

	mux.HandleFunc("POST /api/bookshelf/remove", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct{ ConsumableID any `json:"consumableId"` }
		readBody(r, &b)
		id := str(b.ConsumableID)
		if id == "" {
			sendJSON(w, 400, map[string]any{"error": "Missing consumableId"})
			return
		}
		removed, strategy, err := hydrateClient(u).RemoveFromBookshelf(id)
		if err != nil {
			sendError(w, err)
			return
		}
		if !removed {
			sendJSON(w, 502, map[string]any{"code": "REMOVE_FROM_BOOKSHELF_NOT_CONFIRMED", "error": "Storytel accepted the request but the book is still on the bookshelf"})
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "result": map[string]any{"removed": true, "strategy": strategy}})
	}))

	mux.HandleFunc("POST /api/stream", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct {
			BookID       any `json:"bookId"`
			ConsumableID any `json:"consumableId"`
		}
		readBody(r, &b)
		bookID, consumableID := str(b.BookID), str(b.ConsumableID)
		if a.findDownloadedFile(bookID) != "" {
			// Served same-origin by /api/local-stream, with Range support.
			sendJSON(w, 200, map[string]any{"streamUrl": "/api/local-stream/" + bookID, "remote": false})
			return
		}
		c := hydrateClient(u)
		var streamURL string
		var err error
		if consumableID != "" {
			streamURL, err = c.GetAudioStreamURL(consumableID)
		} else {
			streamURL, err = c.GetStreamURL(bookID)
		}
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"streamUrl": streamURL, "remote": true})
	}))

	mux.HandleFunc("POST /api/bookmarks/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct{ Position, Note any }
		readBody(r, &b)
		if err := hydrateClient(u).SetBookmark(r.PathValue("consumableId"), b.Position, b.Note); err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "message": "Bookmark saved"})
	}))

	mux.HandleFunc("DELETE /api/bookmarks/{consumableId}/{bookmarkId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		if err := hydrateClient(u).DeleteBookmark(r.PathValue("consumableId"), r.PathValue("bookmarkId")); err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "message": "Bookmark removed"})
	}))

	mux.HandleFunc("PUT /api/bookmarks/{consumableId}/{bookmarkId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var body any
		readBody(r, &body)
		if err := hydrateClient(u).UpdateBookmark(r.PathValue("consumableId"), r.PathValue("bookmarkId"), body); err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "message": "Bookmark removed"})
	}))

	mux.HandleFunc("GET /api/bookmark-positional", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		data, err := hydrateClient(u).GetBookmarkPositional("")
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("GET /api/bookmark-positional/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		data, err := hydrateClient(u).GetBookmarkPositional(r.PathValue("consumableId"))
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("PUT /api/bookmark-positional/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct{ Position float64 }
		readBody(r, &b)
		data, err := hydrateClient(u).UpdateBookmarkPositional(r.PathValue("consumableId"), b.Position, newDeviceID())
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("GET /api/bookmarks/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		data, err := hydrateClient(u).GetBookmark(r.PathValue("consumableId"))
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("GET /api/bookmetadata/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		data, err := hydrateClient(u).GetPlayBookMetaData(r.PathValue("consumableId"))
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("GET /api/book-details/{consumableId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		data, err := hydrateClient(u).GetBookDetails(r.PathValue("consumableId"))
		if err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, data)
	}))

	mux.HandleFunc("GET /api/auth/status", func(w http.ResponseWriter, r *http.Request) {
		_, err := verifyJWT(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		sendJSON(w, 200, map[string]any{"authenticated": err == nil})
	})

	mux.HandleFunc("GET /api/account", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		sendJSON(w, 200, map[string]any{"email": u.Email})
	}))

	mux.HandleFunc("GET /api/translations", func(w http.ResponseWriter, r *http.Request) {
		if code := matchLanguage(r.URL.Query().Get("lang")); code != "" {
			w.Header().Set("Content-Language", code)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write(localeJSON(code))
			return
		}
		all := map[string]json.RawMessage{}
		for _, code := range supportedLanguages {
			all[code] = localeJSON(code)
		}
		sendJSON(w, 200, all)
	})

	mux.HandleFunc("POST /api/download", authenticate(a.handleDownload))

	mux.HandleFunc("DELETE /api/download/{bookId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		bookID := r.PathValue("bookId")
		a.mu.Lock()
		d := a.downloads[bookID]
		a.mu.Unlock()
		if d == nil {
			sendJSON(w, 200, map[string]any{"error": "No active download found"})
			return
		}
		d.cancel()
		sendJSON(w, 200, map[string]any{"success": true, "message": "Download cancelled"})
	}))

	serveLocal := func(w http.ResponseWriter, r *http.Request) {
		path := a.findDownloadedFile(r.PathValue("bookId"))
		if path == "" {
			sendJSON(w, 404, map[string]any{"error": "File not found"})
			return
		}
		f, err := os.Open(path)
		if err != nil {
			sendError(w, err)
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(w, r, "", st.ModTime(), f) // handles Range / 206 / 416
	}
	mux.HandleFunc("GET /api/local-stream/{bookId}", serveLocal)
	mux.HandleFunc("GET /api/download/{bookId}", authenticate(func(w http.ResponseWriter, r *http.Request, _ *JWTUser) { serveLocal(w, r) }))

	mux.HandleFunc("GET /api/download-status/{bookId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		sendJSON(w, 200, map[string]any{"downloaded": a.findDownloadedFile(r.PathValue("bookId")) != ""})
	}))

	mux.HandleFunc("DELETE /api/downloaded-file/{bookId}", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		path := a.findDownloadedFile(r.PathValue("bookId"))
		if path == "" {
			sendJSON(w, 404, map[string]any{"error": "File not found"})
			return
		}
		if err := os.Remove(path); err != nil {
			sendError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"success": true, "message": "File deleted successfully"})
	}))

	mux.HandleFunc("GET /api/logs", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		sendJSON(w, 200, appLogger.Logs())
	}))
	mux.HandleFunc("DELETE /api/logs", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		appLogger.Clear()
		sendJSON(w, 200, map[string]any{"success": true})
	}))
	mux.HandleFunc("POST /api/logs/action", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		var b struct {
			Message string
			Data    any
		}
		readBody(r, &b)
		appLogger.Add(LogEntry{Type: "action", Message: b.Message, Data: b.Data})
		sendJSON(w, 200, map[string]any{"success": true})
	}))
	mux.HandleFunc("GET /api/logs/export", authenticate(func(w http.ResponseWriter, r *http.Request, u *JWTUser) {
		raw, _ := json.MarshalIndent(appLogger.Logs(), "", "  ")
		w.Header().Set("Content-Disposition", `attachment; filename="storytel-logs.json"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))

	return mux
}

// handleDownload streams to "<path>.part" and renames on completion, so a
// partial file is never picked up by findDownloadedFile. The request stays
// open until the download finishes, as in the Fastify version.
func (a *API) handleDownload(w http.ResponseWriter, r *http.Request, u *JWTUser) {
	var b struct {
		BookID       any `json:"bookId"`
		ConsumableID any `json:"consumableId"`
		Book         struct {
			Book struct {
				Name string `json:"name"`
			} `json:"book"`
		} `json:"book"`
	}
	readBody(r, &b)
	bookID, consumableID := str(b.BookID), str(b.ConsumableID)
	if !bookIDPattern.MatchString(bookID) {
		sendJSON(w, 400, map[string]any{"error": "Invalid bookId"})
		return
	}
	if a.findDownloadedFile(bookID) != "" {
		sendJSON(w, 200, map[string]any{"success": true, "message": "File already downloaded", "percentage": 100})
		return
	}
	a.mu.Lock()
	if a.downloads[bookID] != nil {
		a.mu.Unlock()
		sendJSON(w, 409, map[string]any{"error": "Download already in progress"})
		return
	}
	finalPath := a.buildDownloadPath(bookID, b.Book.Book.Name)
	partPath := finalPath + ".part"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.downloads[bookID] = &download{cancel: cancel, filePath: partPath}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.downloads, bookID)
		a.mu.Unlock()
	}()

	c := hydrateClient(u)
	var streamURL string
	var err error
	if consumableID != "" {
		streamURL, err = c.GetAudioStreamURL(consumableID)
	} else {
		streamURL, err = c.GetStreamURL(bookID)
	}
	if err != nil {
		sendError(w, err)
		return
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			sendJSON(w, 499, map[string]any{"error": "Download cancelled"})
		} else {
			sendError(w, err)
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		sendError(w, &APIError{Status: resp.StatusCode, Message: fmt.Sprintf("Request failed with status code %d", resp.StatusCode)})
		return
	}

	f, err := os.Create(partPath)
	if err != nil {
		sendError(w, err)
		return
	}
	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		_ = os.Remove(partPath)
		if ctx.Err() != nil {
			sendJSON(w, 499, map[string]any{"error": "Download cancelled"})
		} else {
			sendError(w, err)
		}
		return
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		sendError(w, err)
		return
	}
	sendJSON(w, 200, map[string]any{"success": true, "message": "Download completed", "percentage": 100})
}
