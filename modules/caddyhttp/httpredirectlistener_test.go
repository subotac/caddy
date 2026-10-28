// Copyright 2026 The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddyhttp

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestHTTPRedirectRequestTargets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		method   string
		target   string
		host     string
		location string
	}{
		{
			name:     "origin form",
			method:   "GET",
			target:   "/path%2Fsegment?value=a%26b&empty=",
			host:     "example.com:8443",
			location: "https://example.com:8443/path%2Fsegment?value=a%26b&empty=",
		},
		{
			name:     "absolute form authority",
			method:   "HEAD",
			target:   "http://example.com:8443/path%2Fsegment?value=a%26b",
			host:     "different.example",
			location: "https://example.com:8443/path%2Fsegment?value=a%26b",
		},
		{
			name:     "absolute form query without path",
			method:   "POST",
			target:   "http://example.com:8443?value=a%26b",
			host:     "different.example",
			location: "https://example.com:8443/?value=a%26b",
		},
		{
			name:     "absolute form without path",
			method:   "HEAD",
			target:   "http://example.com:8443",
			host:     "different.example",
			location: "https://example.com:8443/",
		},
		{
			name:     "asterisk form",
			method:   "OPTIONS",
			target:   "*",
			host:     "example.com:8443",
			location: "https://example.com:8443/",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, client := net.Pipe()
			t.Cleanup(func() {
				server.Close()
				client.Close()
			})
			if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}

			redirected := make(chan error, 1)
			go func() {
				defer server.Close()
				conn := &httpRedirectConn{Conn: server, limit: 1024, r: bufio.NewReader(server)}
				_, err := conn.Read(make([]byte, 1))
				redirected <- err
			}()

			if _, err := fmt.Fprintf(client, "%s %s HTTP/1.1\r\nHost: %s\r\n\r\n", tc.method, tc.target, tc.host); err != nil {
				t.Fatal(err)
			}
			resp, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: tc.method})
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusPermanentRedirect {
				t.Errorf("expected status %d, got %d", http.StatusPermanentRedirect, resp.StatusCode)
			}
			if got := resp.Header.Get("Location"); got != tc.location {
				t.Errorf("expected Location %q, got %q", tc.location, got)
			}
			if err := <-redirected; err == nil {
				t.Error("expected the redirect to terminate the connection read")
			}
		})
	}
}
