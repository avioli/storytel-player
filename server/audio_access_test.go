package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeTransport func(*http.Request) *http.Response

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

func fakeResponse(status int, body string, h http.Header) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

// withFakeStorytel answers audio requests with the given statuses in order
// and every legacy login with a fresh JWT.
func withFakeStorytel(t *testing.T, audio ...int) (audioCalls *int) {
	t.Helper()
	calls := 0
	old := httpClient.Transport
	httpClient.Transport = fakeTransport(func(r *http.Request) *http.Response {
		if strings.Contains(r.URL.Path, "login.action") {
			return fakeResponse(200, `{"accountInfo":{"jwt":"fresh","singleSignToken":"s"}}`, nil)
		}
		status := audio[min(calls, len(audio)-1)]
		calls++
		if status == 302 {
			return fakeResponse(302, `{}`, http.Header{"Location": {"https://example.com/a.mp3"}})
		}
		return fakeResponse(status, `{"message":"x"}`, nil)
	})
	t.Cleanup(func() { httpClient.Transport = old })
	tokenCacheMu.Lock()
	clear(legacyLoginCache)
	tokenCacheMu.Unlock()
	return &calls
}

func TestAudioStreamPreserves403(t *testing.T) {
	calls := withFakeStorytel(t, 403)
	c := &StorytelClient{jwt: "old"}
	_, err := c.GetAudioStreamURL("abc")
	if statusOf(err) != 403 || isUnauthorized(err) || *calls != 1 {
		t.Fatalf("got status %d unauthorized %v calls %d", statusOf(err), isUnauthorized(err), *calls)
	}
}

func TestAudioStreamRedirect(t *testing.T) {
	withFakeStorytel(t, 302)
	u, err := (&StorytelClient{jwt: "old"}).GetAudioStreamURL("abc")
	if err != nil || u != "https://example.com/a.mp3" {
		t.Fatalf("got %q, %v", u, err)
	}
}

func TestAudioStreamPreservesPostRenewal403(t *testing.T) {
	calls := withFakeStorytel(t, 401, 403)
	c := &StorytelClient{jwt: "old", email: "e", encPassword: "p"}
	_, err := c.GetAudioStreamURL("abc")
	if statusOf(err) != 403 || isUnauthorized(err) || *calls != 2 {
		t.Fatalf("got status %d unauthorized %v calls %d", statusOf(err), isUnauthorized(err), *calls)
	}
}

func TestAudioStreamExpiredWithoutCredentials(t *testing.T) {
	withFakeStorytel(t, 401)
	_, err := (&StorytelClient{jwt: "old"}).GetAudioStreamURL("abc")
	if !isUnauthorized(err) {
		t.Fatalf("want unauthorized, got %v", err)
	}
}
