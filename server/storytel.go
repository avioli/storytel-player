package main

// Go port of server/storytelApi.ts: the Storytel API client.

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	storytelVersion   = "25.38.0"
	storytelUserAgent = "Storytel/25.38.0 (iOS 26.0; iPhone16,2) Release/924.1"
	firebaseTokenURL  = "https://securetoken.googleapis.com"
	firebaseReferer   = "https://www.storytel.com/"
	bookshelfURL      = "https://api.storytel.net/libraries/bookshelf"
	loginURL          = "https://www.storytel.com/api/login.action"
)

var pwdParam = regexp.MustCompile(`pwd=[^&]+`)

// APIError carries an HTTP status for the route layer. Unauthorized marks a
// Storytel 401 (or rejected Firebase refresh), which routes turn into 401.
type APIError struct {
	Status       int
	Message      string
	Unauthorized bool
	Location     string
	Body         any
}

func (e *APIError) Error() string { return e.Message }

func isUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Unauthorized
}

// wrap mirrors the TS pattern `if (isStorytelUnauthorized) throw; throw new Error(prefix + msg)`.
func wrap(err error, prefix string) error {
	if err == nil || isUnauthorized(err) {
		return err
	}
	return errors.New(prefix + err.Error())
}

func encryptPassword(password string) string {
	key := []byte("VQZBJ6TD8M9WBUWT")
	iv := []byte("joiwef08u23j341a")
	block, _ := aes.NewCipher(key)
	pad := aes.BlockSize - len(password)%aes.BlockSize
	data := append([]byte(password), bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, data)
	return strings.ToUpper(hex.EncodeToString(out))
}

func decodeJwtPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	return payload
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

func str(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	}
	return ""
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func arr(v any) []any {
	a, _ := v.([]any)
	return a
}

// firstNonEmpty returns the first argument that is truthy in the JS sense.
func firstNonEmpty(vals ...any) any {
	for _, v := range vals {
		switch t := v.(type) {
		case nil:
		case string:
			if t != "" {
				return t
			}
		case float64:
			if t != 0 {
				return t
			}
		case bool:
			if t {
				return t
			}
		default:
			return t
		}
	}
	return nil
}

func formatOfType(formats []any, typ string) map[string]any {
	for _, f := range formats {
		if m := obj(f); m != nil && m["type"] == typ {
			return m
		}
	}
	return nil
}

func joinNames(list any) string {
	var names []string
	for _, x := range arr(list) {
		if m := obj(x); m != nil {
			if n := str(m["name"]); n != "" {
				names = append(names, n)
			}
		} else if s := str(x); s != "" {
			names = append(names, s)
		}
	}
	return strings.Join(names, ", ")
}

type SsoSession struct {
	StorytelSession      string `json:"storytelSession"`
	FirebaseRefreshToken string `json:"firebaseRefreshToken"`
	FirebaseAPIKey       string `json:"firebaseApiKey"`
	Email                string `json:"email"`
	Cid                  string `json:"cid"`
}

type cachedToken struct {
	token, singleSignToken string
	expiresAt              int64
}

var (
	tokenCacheMu       sync.Mutex
	firebaseTokenCache = map[string]cachedToken{}
	legacyLoginCache   = map[string]cachedToken{}
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type StorytelClient struct {
	jwt, singleSignToken string
	sso                  *SsoSession
	email, encPassword   string
}

type reqOpts struct {
	method  string
	url     string
	body    any // nil, string (sent raw), or JSON-encodable value
	headers map[string]string
	retried bool
}

type response struct {
	status  int
	headers http.Header
	data    any
}

func cleanURL(u string) string {
	if strings.Contains(u, "login.action") {
		return pwdParam.ReplaceAllString(u, "pwd=***")
	}
	return u
}

// do mirrors the axios instance: iOS headers, ?version=, no redirects,
// status >= 400 is an error, and one transparent token refresh on 401.
func (c *StorytelClient) do(o reqOpts) (*response, error) {
	u, err := url.Parse(o.url)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("version", storytelVersion)
	u.RawQuery = q.Encode()

	var body io.Reader
	var logData any
	contentType := ""
	switch b := o.body.(type) {
	case nil:
	case string:
		body = strings.NewReader(b)
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, _ := json.Marshal(b)
		body = bytes.NewReader(buf)
		contentType = "application/json"
		logData = b
	}
	method := o.method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-storytel-terminal", "ios")
	req.Header.Set("user-agent", storytelUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	logURL := cleanURL(u.String())
	appLogger.Add(LogEntry{Type: "http_request", Message: fmt.Sprintf("[%s] %s", method, logURL), Method: method, URL: logURL, Data: logData})

	resp, err := httpClient.Do(req)
	if err != nil {
		appLogger.Add(LogEntry{Type: "error", Message: "[Error N/A] " + logURL, Method: method, URL: logURL, Data: err.Error()})
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var data any
	if json.Unmarshal(raw, &data) != nil {
		data = string(raw)
	}
	r := &response{status: resp.StatusCode, headers: resp.Header, data: data}

	if resp.StatusCode < 400 {
		appLogger.Add(LogEntry{Type: "http_response", Message: fmt.Sprintf("[%d] %s", resp.StatusCode, logURL), Status: resp.StatusCode, Method: method, URL: logURL, Data: data})
		return r, nil
	}

	appLogger.Add(LogEntry{Type: "error", Message: fmt.Sprintf("[Error %d] %s", resp.StatusCode, logURL), Status: resp.StatusCode, Method: method, URL: logURL, Data: data})
	isLogin := strings.Contains(o.url, "login.action")

	if resp.StatusCode == 401 && !isLogin && !o.retried {
		o.retried = true
		var bearer string
		var rerr error
		if c.sso != nil {
			tokenCacheMu.Lock()
			delete(firebaseTokenCache, c.sso.FirebaseAPIKey+":"+c.sso.FirebaseRefreshToken)
			tokenCacheMu.Unlock()
			bearer, rerr = c.ensureFirebaseIDToken()
		} else if c.email != "" && c.encPassword != "" {
			tokenCacheMu.Lock()
			delete(legacyLoginCache, c.email+":"+c.encPassword)
			tokenCacheMu.Unlock()
			rerr = c.refreshLegacyLogin()
			bearer = c.jwt
		} else {
			rerr = errors.New("no credentials")
		}
		if rerr == nil {
			h := map[string]string{}
			for k, v := range o.headers {
				h[k] = v
			}
			h["Authorization"] = "Bearer " + bearer
			o.headers = h
			if rr, err := c.do(o); err == nil {
				return rr, nil
			}
		}
	}

	if resp.StatusCode == 401 {
		msg := "Storytel session expired"
		if isLogin {
			msg = "Storytel login rejected"
		}
		return r, &APIError{Status: 401, Message: msg, Unauthorized: true, Body: data}
	}
	return r, &APIError{
		Status:   resp.StatusCode,
		Message:  fmt.Sprintf("Request failed with status code %d", resp.StatusCode),
		Location: resp.Header.Get("Location"),
		Body:     data,
	}
}

func (c *StorytelClient) SetCredentials(email, encPassword string) {
	c.email = strings.TrimSpace(email)
	c.encPassword = strings.TrimSpace(encPassword)
}

func jwtLifetime(jwt string) int64 {
	if p := decodeJwtPayload(jwt); p != nil {
		exp, iat := int64(num(p["exp"])), int64(num(p["iat"]))
		if exp != 0 && iat != 0 {
			return max(300, exp-iat)
		}
	}
	return 1800
}

func (c *StorytelClient) storeLogin(data any, cacheKey string) {
	info := obj(obj(data)["accountInfo"])
	c.jwt = str(info["jwt"])
	c.singleSignToken = str(info["singleSignToken"])
	tokenCacheMu.Lock()
	legacyLoginCache[cacheKey] = cachedToken{c.jwt, c.singleSignToken, time.Now().Unix() + jwtLifetime(c.jwt)}
	tokenCacheMu.Unlock()
}

func (c *StorytelClient) loginRequest() (*response, error) {
	v := url.Values{"m": {"1"}, "uid": {c.email}, "pwd": {c.encPassword}}
	return c.do(reqOpts{url: loginURL + "?" + v.Encode()})
}

func (c *StorytelClient) refreshLegacyLogin() error {
	if c.email == "" || c.encPassword == "" {
		return nil
	}
	key := c.email + ":" + c.encPassword
	tokenCacheMu.Lock()
	cached, ok := legacyLoginCache[key]
	tokenCacheMu.Unlock()
	if ok && cached.expiresAt-60 > time.Now().Unix() {
		c.jwt, c.singleSignToken = cached.token, cached.singleSignToken
		return nil
	}
	r, err := c.loginRequest()
	if err != nil {
		return err
	}
	if str(obj(obj(r.data)["accountInfo"])["jwt"]) != "" {
		c.storeLogin(r.data, key)
	}
	return nil
}

func (c *StorytelClient) Login(email, password string) error {
	c.SetCredentials(email, encryptPassword(strings.TrimSpace(password)))
	r, err := c.loginRequest()
	if err != nil {
		if isUnauthorized(err) {
			return err
		}
		return errors.New("Login failed: " + err.Error())
	}
	c.sso = nil
	c.storeLogin(r.data, c.email+":"+c.encPassword)
	return nil
}

func (c *StorytelClient) LoginViaSso(s SsoSession) {
	c.sso = &s
	c.jwt, c.singleSignToken = "", ""
}

func (c *StorytelClient) legacyActionToken() string {
	if c.sso != nil {
		return c.sso.StorytelSession
	}
	return c.singleSignToken
}

func (c *StorytelClient) ensureFirebaseIDToken() (string, error) {
	if c.sso == nil {
		return "", errors.New("ensureFirebaseIdToken called outside SSO mode")
	}
	now := time.Now().Unix()
	key := c.sso.FirebaseAPIKey + ":" + c.sso.FirebaseRefreshToken
	tokenCacheMu.Lock()
	cached, ok := firebaseTokenCache[key]
	tokenCacheMu.Unlock()
	if ok && cached.expiresAt-60 > now {
		return cached.token, nil
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {c.sso.FirebaseRefreshToken}}
	req, _ := http.NewRequest("POST", firebaseTokenURL+"/v1/token?key="+url.QueryEscape(c.sso.FirebaseAPIKey), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", firebaseReferer)
	req.Header.Set("Origin", "https://www.storytel.com")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("Firebase token refresh failed: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "", &APIError{Status: 401, Message: "Firebase refresh token rejected", Unauthorized: true}
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Firebase token refresh failed: Request failed with status code %d", resp.StatusCode)
	}
	var data map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&data)
	token := str(firstNonEmpty(data["id_token"], data["access_token"]))
	if token == "" {
		return "", errors.New("Firebase token refresh failed: Firebase token response did not include an ID token")
	}
	expiresIn := int64(num(data["expires_in"]))
	if expiresIn == 0 {
		expiresIn = 3600
	}
	tokenCacheMu.Lock()
	firebaseTokenCache[key] = cachedToken{token: token, expiresAt: now + expiresIn}
	tokenCacheMu.Unlock()
	return token, nil
}

func (c *StorytelClient) apiBearer() (string, error) {
	if c.sso != nil {
		return c.ensureFirebaseIDToken()
	}
	if c.jwt != "" {
		if p := decodeJwtPayload(c.jwt); p != nil {
			if exp := int64(num(p["exp"])); exp != 0 && exp-60 <= time.Now().Unix() && c.email != "" && c.encPassword != "" {
				if err := c.refreshLegacyLogin(); err != nil {
					return "", err
				}
			}
		}
		return c.jwt, nil
	}
	if c.email != "" && c.encPassword != "" {
		if err := c.refreshLegacyLogin(); err != nil {
			return "", err
		}
		return c.jwt, nil
	}
	return "", nil
}

// authed issues a request with the api.storytel.net bearer.
func (c *StorytelClient) authed(method, u string, body any, accept string) (*response, error) {
	bearer, err := c.apiBearer()
	if err != nil {
		return nil, err
	}
	h := map[string]string{"Authorization": "Bearer " + bearer}
	if accept != "" {
		h["Accept"] = accept
	}
	return c.do(reqOpts{method: method, url: u, body: body, headers: h})
}

var twoLetters = regexp.MustCompile(`^[a-z]{2}$`)

func countryFromJwt(token string) string {
	p := decodeJwtPayload(token)
	if p == nil {
		return ""
	}
	if store, ok := p["store"].(string); ok {
		parts := strings.Split(store, "-")
		code := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
		if twoLetters.MatchString(code) {
			return code
		}
	}
	for _, claim := range []any{p["country"], p["countryCode"]} {
		if s, ok := claim.(string); ok {
			s = strings.ToLower(strings.TrimSpace(s))
			if twoLetters.MatchString(s) {
				return s
			}
		}
	}
	return ""
}

func (c *StorytelClient) marketCountryCode() string {
	var candidates []string
	if b, err := c.apiBearer(); err == nil {
		candidates = append(candidates, b)
	}
	candidates = append(candidates, c.jwt)
	if c.sso != nil {
		candidates = append(candidates, c.sso.StorytelSession)
	}
	for _, t := range candidates {
		if code := countryFromJwt(t); code != "" {
			return code
		}
	}
	return ""
}

func (c *StorytelClient) GetBookmarkPositional(consumableID string) (any, error) {
	u := "https://api.storytel.net/bookmarks/positional?kidsMode=false&orderBy=updated&orderDirection=desc"
	if consumableID != "" {
		u += "&consumableIds=" + url.QueryEscape(consumableID)
	}
	r, err := c.authed("GET", u, nil, "")
	if err != nil {
		return nil, wrap(err, "Failed to get bookmark positional: ")
	}
	return obj(r.data)["bookmarks"], nil
}

func (c *StorytelClient) UpdateBookmarkPositional(consumableID string, position float64, deviceID string) (any, error) {
	r, err := c.authed("POST", "https://api.storytel.net/bookmarks/positional", map[string]any{
		"deviceId": deviceID, "action": "player_paused", "secondsSinceCreated": 0,
		"position": position, "type": "abook", "kidsMode": false, "consumableId": consumableID,
	}, "")
	if err != nil {
		return nil, wrap(err, "Failed to get bookmark positional: ")
	}
	return r.data, nil
}

// fetchBookshelfSnapshot sends an empty form body, which the endpoint reads as
// "send me everything". A JSON body such as {"items": []} is rejected with 400.
func (c *StorytelClient) fetchBookshelfSnapshot() (map[string]any, error) {
	r, err := c.authed("POST", bookshelfURL, "", "*/*")
	if err != nil {
		return nil, err
	}
	return obj(r.data), nil
}

func (c *StorytelClient) postBookshelfDelta(body map[string]any) (map[string]any, error) {
	r, err := c.authed("POST", bookshelfURL, body, "*/*")
	if err != nil {
		return nil, err
	}
	return obj(r.data), nil
}

var stateToStatus = map[string]int{
	"WILL_CONSUME": 1, "CONSUMING": 2, "CONSUMED": 3, "FINISHED": 3,
	"COMPLETED": 3, "READING": 2, "TO_READ": 1,
}

// GetBookshelf remaps the bookshelf onto the legacy BookShelfEntity shape the
// frontend reads (client/src/interfaces/books.ts).
func (c *StorytelClient) GetBookshelf() (map[string]any, error) {
	data, err := c.fetchBookshelfSnapshot()
	if err != nil {
		return nil, wrap(err, "Failed to get bookshelf: ")
	}
	books := []any{}
	for _, entry := range obj(data["items"]) {
		model := obj(obj(entry)["model"])
		if model == nil {
			continue
		}
		formats := arr(model["formats"])
		abook := formatOfType(formats, "abook")
		ebook := formatOfType(formats, "ebook")
		cover := str(firstNonEmpty(obj(obj(abook)["cover"])["url"], obj(obj(ebook)["cover"])["url"]))

		status := stateToStatus[strings.ToUpper(str(model["state"]))]
		if status == 0 {
			status = 1
			if num(obj(abook["position"])["position"]) > 0 {
				status = 2
			}
		}
		entity := map[string]any{
			"id":                  model["id"],
			"status":              status,
			"stateUpdateTime":     model["stateUpdateTime"],
			"positionUpdatedTime": obj(abook["position"])["updatedTime"],
			"book": map[string]any{
				"name":            model["title"],
				"authorsAsString": joinNames(model["authors"]),
				"consumableId":    str(model["id"]),
				"largeCover":      cover,
				"largeCoverE":     "",
				"category":        map[string]any{"title": str(obj(model["category"])["name"])},
				"language":        map[string]any{"localizedName": ""},
			},
			"abook":     nil,
			"abookMark": nil,
			"ebook":     nil,
		}
		if abook != nil {
			// Legacy frontend expects microseconds; the API gives ms.
			entity["abook"] = map[string]any{
				"id": abook["id"], "narratorAsString": joinNames(model["narrators"]),
				"time": num(abook["durationInMilliseconds"]) * 1000, "description": "",
			}
			if pos := obj(abook["position"]); pos != nil {
				entity["abookMark"] = map[string]any{"pos": num(pos["position"]) * 1000}
			}
		}
		if ebook != nil {
			entity["ebook"] = ebook
		}
		books = append(books, entity)
	}
	return map[string]any{"books": books}, nil
}

func extractLanguageInfo(model, raw, item map[string]any) (string, string) {
	formats := arr(model["formats"])
	if formats == nil {
		formats = arr(raw["formats"])
	}
	var formatLang any
	for _, f := range formats {
		if l := obj(f)["language"]; l != nil {
			formatLang = l
			break
		}
	}
	var firstOf = func(v any) any {
		if a := arr(v); len(a) > 0 {
			return a[0]
		}
		return nil
	}
	lang := firstNonEmpty(model["language"], firstOf(model["languages"]), formatLang,
		raw["language"], firstOf(raw["languages"]), item["language"], obj(item["book"])["language"])
	switch l := lang.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(l)), ""
	case map[string]any:
		iso := strings.ToLower(strings.TrimSpace(str(firstNonEmpty(l["isoValue"], l["iso"], l["code"], l["id"]))))
		return iso, str(firstNonEmpty(l["localizedName"], l["name"], l["title"]))
	}
	return "", ""
}

func searchResult(id, title, authors, narrators, cover, category string, durationMs float64, description string, hasEbook bool, lang, langName string) map[string]any {
	r := map[string]any{
		"id": id, "title": title, "authors": authors, "narrators": narrators,
		"coverUrl": cover, "category": category, "durationMs": durationMs,
		"description": description, "hasAbook": true, "hasEbook": hasEbook,
	}
	if lang != "" {
		r["language"] = lang
	}
	if langName != "" {
		r["languageName"] = langName
	}
	return r
}

func webSearchBaseURL(country string) string {
	if country == "dk" {
		return "https://mofibo.com/dk/da"
	}
	return fmt.Sprintf("https://www.storytel.com/%s/%s", country, country)
}

func (c *StorytelClient) SearchCatalog(query string) ([]any, error) {
	trimmed := strings.TrimSpace(query)
	results := []any{}
	if trimmed == "" {
		return results, nil
	}
	seen := map[string]bool{}
	q := url.QueryEscape(trimmed)

	// 1. Authenticated api.storytel.net search, scoped to the account's market.
	endpoints := []string{
		"https://api.storytel.net/search/v2?query=" + q + "&configVariant=default",
		"https://api.storytel.net/search/page?query=" + q + "&kidsMode=false",
		"https://api.storytel.net/search?query=" + q + "&kidsMode=false",
	}
	for _, u := range endpoints {
		r, err := c.authed("GET", u, nil, "*/*")
		if err != nil {
			continue
		}
		d := obj(r.data)
		items := arr(d["items"])
		if items == nil {
			items = arr(d["results"])
		}
		for _, it := range items {
			raw := obj(it)
			model := obj(firstNonEmpty(raw["model"], raw["item"], raw["book"]))
			if model == nil {
				model = raw
			}
			id := str(firstNonEmpty(model["id"], model["consumableId"], raw["id"], raw["consumableId"]))
			title := str(firstNonEmpty(model["title"], model["name"], raw["title"], raw["name"]))
			if id == "" || title == "" || seen[id] {
				continue
			}
			formats := arr(model["formats"])
			abook := formatOfType(formats, "abook")
			ebook := formatOfType(formats, "ebook")
			// Exclude ebook-only items: this is an audiobook player.
			if abook == nil {
				continue
			}
			seen[id] = true
			cover := str(firstNonEmpty(obj(abook["cover"])["url"], obj(ebook["cover"])["url"], obj(model["cover"])["url"]))
			lang, langName := extractLanguageInfo(model, raw, nil)
			category := str(firstNonEmpty(obj(model["category"])["name"], obj(model["category"])["title"]))
			results = append(results, searchResult(id, title, joinNames(model["authors"]), joinNames(model["narrators"]),
				cover, category, num(abook["durationInMilliseconds"]), str(model["description"]), ebook != nil, lang, langName))
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// 2. Fallback to the storytel.com web search, scoped by market country.
	country := c.marketCountryCode()
	if country == "" {
		return results, nil
	}
	r, err := c.do(reqOpts{url: webSearchBaseURL(country) + "/api/search.action?q=" + q})
	if err != nil {
		return results, nil
	}
	for _, it := range arr(obj(r.data)["books"]) {
		item := obj(it)
		book, abook, ebook := obj(item["book"]), obj(item["abook"]), obj(item["ebook"])
		if book == nil {
			book = map[string]any{}
		}
		if str(abook["id"]) == "" && item["abook"] == nil {
			continue
		}
		id := str(firstNonEmpty(book["consumableId"], book["id"], abook["id"], item["id"]))
		title := str(firstNonEmpty(book["name"], book["title"]))
		if id == "" || title == "" || seen[id] {
			continue
		}
		seen[id] = true
		cover := str(firstNonEmpty(book["largeCover"], book["largeCoverE"], book["cover"], book["coverE"], book["smallCover"], abook["cover"]))
		authors := str(book["authorsAsString"])
		if authors == "" {
			authors = joinNames(book["authors"])
		}
		narrators := str(abook["narratorAsString"])
		if narrators == "" {
			narrators = joinNames(abook["narrators"])
		}
		var duration float64
		if l := num(abook["length"]); l != 0 {
			duration = l
		} else if t := num(abook["time"]); t != 0 {
			duration = float64(int64(t / 1000000))
		}
		lang, langName := extractLanguageInfo(book, item, item)
		category := str(firstNonEmpty(obj(book["category"])["title"], obj(book["category"])["name"]))
		description := str(firstNonEmpty(abook["description"], ebook["description"]))
		hasEbook := str(ebook["id"]) != "" || item["ebook"] != nil
		results = append(results, searchResult(id, title, authors, narrators, cover, category, duration, description, hasEbook, lang, langName))
	}
	return results, nil
}

func deltaContainsConsumable(data map[string]any, id string) bool {
	for key, entry := range obj(data["items"]) {
		if key == id || str(obj(obj(entry)["model"])["id"]) == id {
			return true
		}
	}
	return false
}

func (c *StorytelClient) isOnBookshelf(id string) (bool, error) {
	shelf, err := c.GetBookshelf()
	if err != nil {
		if isUnauthorized(err) {
			return false, err
		}
		return false, nil
	}
	for _, b := range shelf["books"].([]any) {
		e := b.(map[string]any)
		if str(e["id"]) == id || str(obj(e["book"])["consumableId"]) == id {
			return true, nil
		}
	}
	return false, nil
}

// buildBookshelfModel mirrors the shape the endpoint returns for shelf
// entries, enriched from book-details when that call succeeds.
func (c *StorytelClient) buildBookshelfModel(id, state, stateUpdateTime string) (map[string]any, error) {
	model := map[string]any{
		"id": id, "state": state, "stateUpdateTime": stateUpdateTime,
		"deepLink":   "storytel://book-details-page/book-details/consumables/" + id,
		"resultType": "book",
	}
	details, err := c.GetBookDetails(id)
	if err != nil {
		if isUnauthorized(err) {
			return nil, err
		}
		model["kidsBook"] = false
		return model, nil
	}
	d := obj(details)
	orEmpty := func(v any) []any {
		if a := arr(v); a != nil {
			return a
		}
		return []any{}
	}
	model["title"] = str(d["title"])
	model["shareUrl"] = str(d["shareUrl"])
	model["kidsBook"] = d["kidsBook"] == true
	model["authors"] = orEmpty(d["authors"])
	model["narrators"] = orEmpty(d["narrators"])
	model["formats"] = orEmpty(d["formats"])
	model["duration"] = d["duration"]
	model["category"] = d["category"]
	model["language"] = d["language"]
	return model, nil
}

func statusOf(err error) int {
	var ae *APIError
	if errors.As(err, &ae) && ae.Status != 0 {
		return ae.Status
	}
	return 500
}

func (c *StorytelClient) AddToBookshelf(id string) (bool, string, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	current, err := c.fetchBookshelfSnapshot()
	if err != nil {
		if isUnauthorized(err) {
			return false, "", err
		}
		return false, "", &APIError{Status: statusOf(err), Message: "Failed to add book to bookshelf: " + err.Error()}
	}
	if deltaContainsConsumable(current, id) {
		return true, "already-on-bookshelf", nil
	}
	model, err := c.buildBookshelfModel(id, "WILL_CONSUME", now)
	if err != nil {
		return false, "", err
	}
	body := map[string]any{"items": map[string]any{id: map[string]any{"action": "SET", "model": model}}}
	if rv := current["resourceVersion"]; rv != nil && rv != "" {
		body["resourceVersion"] = rv
	}
	data, err := c.postBookshelfDelta(body)
	if err != nil {
		if isUnauthorized(err) {
			return false, "", err
		}
		detail := err.Error()
		var ae *APIError
		if errors.As(err, &ae) {
			if b := obj(ae.Body); b != nil {
				if m := str(firstNonEmpty(b["message"], b["error"])); m != "" {
					detail = m
				}
			}
		}
		return false, "", &APIError{Status: statusOf(err), Message: "Failed to add book to bookshelf: " + detail}
	}
	if deltaContainsConsumable(data, id) {
		return true, "sync-items-map", nil
	}
	if ok, err := c.isOnBookshelf(id); err != nil {
		return false, "", err
	} else if ok {
		return true, "sync-verified", nil
	}
	return false, "", nil
}

// RemoveFromBookshelf tries the plausible delete verbs (the protocol is not
// documented) and only reports success once the shelf no longer has the book.
func (c *StorytelClient) RemoveFromBookshelf(id string) (bool, string, error) {
	current, err := c.fetchBookshelfSnapshot()
	if err != nil {
		if isUnauthorized(err) {
			return false, "", err
		}
		return false, "", &APIError{Status: statusOf(err), Message: "Failed to remove book from bookshelf: " + err.Error()}
	}
	if !deltaContainsConsumable(current, id) {
		return true, "not-on-bookshelf", nil
	}
	resourceVersion := current["resourceVersion"]
	var lastErr error

	type attempt struct {
		strategy string
		build    func() (map[string]any, error)
	}
	var attempts []attempt
	for _, action := range []string{"DELETE", "REMOVE", "UNSET"} {
		action := action
		attempts = append(attempts, attempt{strings.ToLower(action), func() (map[string]any, error) {
			return map[string]any{"items": map[string]any{id: map[string]any{"action": action}}}, nil
		}})
	}
	attempts = append(attempts, attempt{"delete-with-model", func() (map[string]any, error) {
		model, err := c.buildBookshelfModel(id, "WILL_CONSUME", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": map[string]any{id: map[string]any{"action": "DELETE", "model": model}}}, nil
	}})

	for _, a := range attempts {
		body, err := a.build()
		if err != nil {
			if isUnauthorized(err) {
				return false, "", err
			}
			lastErr = err
			continue
		}
		if resourceVersion != nil && resourceVersion != "" {
			body["resourceVersion"] = resourceVersion
		}
		if _, err := c.postBookshelfDelta(body); err != nil {
			if isUnauthorized(err) {
				return false, "", err
			}
			lastErr = err
			continue
		}
		on, err := c.isOnBookshelf(id)
		if err != nil {
			return false, "", err
		}
		if !on {
			return true, "sync-" + a.strategy, nil
		}
		if snap, err := c.fetchBookshelfSnapshot(); err == nil {
			resourceVersion = snap["resourceVersion"]
		}
	}
	var ae *APIError
	if lastErr != nil && !errors.As(lastErr, &ae) {
		return false, "", &APIError{Status: 500, Message: "Failed to remove book from bookshelf: " + lastErr.Error()}
	}
	return false, "", nil
}

func (c *StorytelClient) GetBookDetails(id string) (any, error) {
	r, err := c.authed("GET", "https://api.storytel.net/book-details/consumables/"+url.PathEscape(id)+"?kidsMode=false&configVariant=default", nil, "*/*")
	if err != nil {
		return nil, wrap(err, "Failed to get book details: ")
	}
	return r.data, nil
}

func hasChapters(data any) bool {
	d := obj(data)
	if d == nil {
		return false
	}
	nonEmpty := func(v any) bool { return len(arr(v)) > 0 }
	if nonEmpty(d["chapters"]) || nonEmpty(d["tracks"]) || nonEmpty(d["items"]) || nonEmpty(obj(d["abook"])["chapters"]) {
		return true
	}
	abook := formatOfType(arr(d["formats"]), "abook")
	return nonEmpty(abook["chapters"]) || nonEmpty(abook["tracks"])
}

func (c *StorytelClient) GetPlayBookMetaData(id string) (any, error) {
	pid := url.PathEscape(id)
	candidates := []string{
		"https://api.storytel.net/playback-metadata/consumables/" + pid,
		"https://api.storytel.net/playback-metadata/consumable/" + pid,
		"https://api.storytel.net/playback-metadata/v2/consumables/" + pid,
		"https://api.storytel.net/assets/v2/consumables/" + pid + "/abook",
		"https://api.storytel.net/consumables/" + pid + "/tracks",
		"https://api.storytel.net/consumables/" + pid + "/chapters",
	}
	var fallback any
	var lastErr error
	for _, u := range candidates {
		r, err := c.authed("GET", u, nil, "application/json, text/plain, */*")
		if err != nil {
			if isUnauthorized(err) {
				return nil, err
			}
			lastErr = err
			continue
		}
		if r.data != nil && r.data != "" {
			if hasChapters(r.data) {
				return r.data, nil
			}
			if fallback == nil {
				fallback = r.data
			}
		}
	}
	if token := c.legacyActionToken(); token != "" {
		q := "?token=" + url.QueryEscape(token) + "&programId=" + url.QueryEscape(id)
		for _, action := range []string{"getBookInfo", "getABookTracks", "getChapters"} {
			r, err := c.do(reqOpts{url: "https://www.storytel.com/api/" + action + ".action" + q})
			if err != nil || r.data == nil || r.data == "" {
				continue
			}
			if hasChapters(r.data) {
				return r.data, nil
			}
			if fallback == nil {
				fallback = r.data
			}
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	details, err := c.GetBookDetails(id)
	if err == nil && details != nil {
		return details, nil
	}
	if err != nil {
		if isUnauthorized(err) {
			return nil, err
		}
		lastErr = err
	}
	msg := "Unknown error"
	if lastErr != nil {
		msg = lastErr.Error()
	}
	return nil, errors.New("Failed to get bookinfo: " + msg)
}

// redirectTarget reads the Location of a 3xx (redirects are never followed).
func redirectTarget(r *response, err error, prefix string) (string, error) {
	if err != nil {
		if isUnauthorized(err) {
			return "", err
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Location != "" {
			return ae.Location, nil
		}
		return "", errors.New(prefix + err.Error())
	}
	return r.headers.Get("Location"), nil
}

// GetAudioStreamURL resolves the 302 to the signed CDN mp3 URL.
func (c *StorytelClient) GetAudioStreamURL(consumableID string) (string, error) {
	r, err := c.authed("GET", "https://api.storytel.net/assets/v2/consumables/"+url.PathEscape(consumableID)+"/abook", nil, "*/*")
	return redirectTarget(r, err, "Failed to get audio stream URL: ")
}

func (c *StorytelClient) GetStreamURL(bookID string) (string, error) {
	r, err := c.do(reqOpts{url: "https://www.storytel.com/mp3streamRangeReq?startposition=0&programId=" + url.QueryEscape(bookID) + "&token=" + url.QueryEscape(c.legacyActionToken())})
	return redirectTarget(r, err, "Failed to get stream URL: ")
}

const bookmarkAccept = "application/vnd.storytel.bookmark+json;v=2.0"

func (c *StorytelClient) GetBookmark(consumableID string) (any, error) {
	r, err := c.authed("GET", "https://api.storytel.net/bookmarks/manual?type=abook&consumableId="+url.QueryEscape(consumableID), nil, bookmarkAccept)
	if err != nil {
		return nil, wrap(err, "Failed to get bookmark: ")
	}
	return r.data, nil
}

func (c *StorytelClient) SetBookmark(consumableID string, position any, note any) error {
	_, err := c.authed("POST", "https://api.storytel.net/bookmarks/manual", map[string]any{
		"position": position, "consumableId": consumableID, "note": note, "type": "abook",
	}, bookmarkAccept)
	return wrap(err, "Failed to set bookmark: ")
}

func (c *StorytelClient) ensureBookmarkExists(consumableID, bookmarkID string) error {
	data, err := c.GetBookmark(consumableID)
	if err != nil {
		return err
	}
	for _, b := range arr(obj(data)["bookmarks"]) {
		if str(obj(b)["id"]) == bookmarkID {
			return nil
		}
	}
	return errors.New("Failed to remove bookmark: bookmark does not exists!")
}

func (c *StorytelClient) UpdateBookmark(consumableID, bookmarkID string, data any) error {
	if err := c.ensureBookmarkExists(consumableID, bookmarkID); err != nil {
		return err
	}
	id := url.PathEscape(bookmarkID)
	_, err := c.authed("PUT", "https://api.storytel.net/bookmarks/manual/"+id+"?id="+id, data, bookmarkAccept)
	return wrap(err, "Failed to update bookmark: ")
}

func (c *StorytelClient) DeleteBookmark(consumableID, bookmarkID string) error {
	if err := c.ensureBookmarkExists(consumableID, bookmarkID); err != nil {
		return err
	}
	id := url.PathEscape(bookmarkID)
	_, err := c.authed("DELETE", "https://api.storytel.net/bookmarks/manual/"+id+"?id="+id, nil, bookmarkAccept)
	return wrap(err, "Failed to delete bookmark: ")
}
