package oilpriceapi

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"testing"
	"time"
)

func TestTimeoutDoesNotMutateSharedHTTPClient(t *testing.T) {
	for _, timeout := range []time.Duration{time.Second, 0, -time.Second} {
		for _, timeoutFirst := range []bool{false, true} {
			name := timeout.String() + "/http-client-first"
			if timeoutFirst {
				name = timeout.String() + "/timeout-first"
			}
			t.Run(name, func(t *testing.T) {
				jar, err := cookiejar.New(nil)
				if err != nil {
					t.Fatal(err)
				}
				redirectErr := errors.New("redirect disabled")
				shared := &http.Client{
					Timeout:   7 * time.Second,
					Transport: &http.Transport{},
					Jar:       jar,
					CheckRedirect: func(*http.Request, []*http.Request) error {
						return redirectErr
					},
				}
				opts := []ClientOption{WithHTTPClient(shared), WithTimeout(timeout)}
				if timeoutFirst {
					opts[0], opts[1] = opts[1], opts[0]
				}
				client := NewClient("test-key", opts...)
				if shared.Timeout != 7*time.Second {
					t.Errorf("caller-owned timeout changed to %v", shared.Timeout)
				}
				if client.httpClient.Timeout != timeout {
					t.Errorf("SDK timeout = %v, want %v", client.httpClient.Timeout, timeout)
				}
				if client.httpClient == shared {
					t.Error("explicit timeout reused the caller-owned client")
				}
				if client.httpClient.Transport != shared.Transport || client.httpClient.Jar != shared.Jar {
					t.Error("transport or cookie jar was not preserved")
				}
				if client.httpClient.CheckRedirect == nil || client.httpClient.CheckRedirect(nil, nil) != redirectErr {
					t.Error("redirect policy was not preserved")
				}
				other := NewClient("other-test-key", WithHTTPClient(shared))
				if other.httpClient != shared || other.httpClient.Timeout != 7*time.Second {
					t.Error("client without a timeout override was changed")
				}
			})
		}
	}
}

func TestLastTimeoutOptionWins(t *testing.T) {
	shared := &http.Client{Timeout: 7 * time.Second}
	client := NewClient("test-key", WithTimeout(time.Second), WithHTTPClient(shared), WithTimeout(0))
	if client.httpClient.Timeout != 0 || shared.Timeout != 7*time.Second {
		t.Fatal("last timeout override must win without modifying the shared client")
	}
}

func TestConcurrentTimeoutOptionsWithSharedHTTPClient(t *testing.T) {
	shared := &http.Client{Timeout: 7 * time.Second}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for i := 0; i < 100; i++ {
				client := NewClient("test-key", WithHTTPClient(shared), WithTimeout(time.Duration(i)*time.Second))
				if client.httpClient.Timeout != time.Duration(i)*time.Second {
					t.Error("another client changed the timeout")
				}
			}
		}()
	}
	close(start)
	workers.Wait()
	if shared.Timeout != 7*time.Second {
		t.Errorf("shared timeout changed to %v", shared.Timeout)
	}
}
