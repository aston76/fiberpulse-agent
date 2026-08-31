package accountsync

import (
	"context"
	"net/http"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (fn roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestNewRejectsInsecureOrigin(t *testing.T) {
	if _, err := New("http://testspeednow.com", nil); err == nil {
		t.Fatal("expected insecure account origin to be rejected")
	}
}

func TestStartRejectsIncompleteResponse(t *testing.T) {
	client, err := New("https://testspeednow.com", roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 201, Body: http.NoBody, Header: make(http.Header)}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Start(context.Background(), "Mac"); err == nil {
		t.Fatal("expected incomplete response to fail")
	}
}
