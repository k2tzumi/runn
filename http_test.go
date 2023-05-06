package runn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/k1LoW/runn/testutil"
)

func TestHTTPRunnerRunUsingGitHubAPI(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("env GITHUB_TOKEN is not set")
	}
	endpoint := "https://api.github.com"
	tests := []struct {
		req                  *httpRequest
		useOpenApi3Validator bool
		want                 int
	}{
		{
			&httpRequest{
				path:      "/users/k1LoW",
				method:    http.MethodGet,
				mediaType: MediaTypeApplicationJSON,
				headers: map[string]string{
					"Authorization": fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")),
				},
			},
			true,
			http.StatusOK,
		},
		{
			&httpRequest{
				path:      "/invalid/endpoint",
				method:    http.MethodGet,
				mediaType: MediaTypeApplicationJSON,
				headers: map[string]string{
					"Authorization": fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")),
				},
			},
			false,
			http.StatusNotFound,
		},
	}

	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for i, tt := range tests {
		r, err := newHTTPRunner("req", endpoint)
		if err != nil {
			t.Fatal(err)
		}
		r.operator = o
		if tt.useOpenApi3Validator {
			c := &httpRunnerConfig{
				OpenApi3DocLocation:  "testdata/openapi3.yml",
				SkipValidateRequest:  false,
				SkipValidateResponse: false,
			}
			v, err := newHttpValidator(c)
			if err != nil {
				t.Fatal(err)
			}
			r.validator = v
		}
		if err := r.Run(ctx, tt.req); err != nil {
			t.Error(err)
			continue
		}
		if want := i + 1; len(r.operator.store.steps) != want {
			t.Errorf("got %v want %v", len(r.operator.store.steps), want)
			continue
		}
		res, ok := r.operator.store.steps[i]["res"].(map[string]interface{})
		if !ok {
			t.Fatalf("invalid steps res: %v", r.operator.store.steps[i]["res"])
		}
		got, ok := res["status"].(int)
		if !ok {
			t.Fatalf("invalid res status: %v", res["status"])
		}
		if got != tt.want {
			t.Errorf("got %v\nwant %v", got, tt.want)
		}
	}
}

func TestRequestBody(t *testing.T) {
	tests := []struct {
		in        string
		mediaType string
		want      string
	}{
		{
			`
data:
  one: ichi
  two: ni`,
			MediaTypeApplicationJSON,
			`{"data":{"one":"ichi","two":"ni"}}`,
		},
		{
			`
data:
  one: 1
  two: ni`,
			MediaTypeApplicationJSON,
			`{"data":{"one":1,"two":"ni"}}`,
		},
		{
			`text`,
			MediaTypeTextPlain,
			`text`,
		},
		{
			`
one: ichi
two: ni`,
			MediaTypeApplicationFormUrlencoded,
			`one=ichi&two=ni`,
		},
		{
			`
data:
  one: "i\nchi"
  two: "n\r\ni"`,
			MediaTypeApplicationJSON,
			`{"data":{"one":"i\nchi","two":"n\r\ni"}}`,
		},
	}

	for _, tt := range tests {
		var b interface{}
		if err := yaml.Unmarshal([]byte(tt.in), &b); err != nil {
			t.Fatal(err)
		}
		r := &httpRequest{
			mediaType: tt.mediaType,
			body:      b,
		}
		body, err := r.encodeBody()
		if err != nil {
			t.Fatal(err)
		}
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, body); err != nil {
			t.Fatal(err)
		}
		got := buf.String()
		if got != tt.want {
			t.Errorf("got %v\nwant %v", got, tt.want)
		}
	}
}

func TestRequestBodyForMultipart(t *testing.T) {
	dummy0, err := os.ReadFile("testdata/dummy.png")
	if err != nil {
		t.Fatal(err)
	}
	dummy1, err := os.ReadFile("testdata/dummy.jpg")
	if err != nil {
		t.Fatal(err)
	}

	multitests := []struct {
		in                     string
		mediaType              string
		wantContainRequestBody []string
		wantContentType        string
	}{
		{
			`
upload0: 'testdata/dummy.png'
upload1: 'testdata/dummy.jpg'
name: 'bob'`,
			MediaTypeMultipartFormData,
			[]string{
				"--123456789012345678901234567890abcdefghijklmnopqrstuvwxyz\r\n",
				"Content-Disposition: form-data; name=\"upload0\"; filename=\"dummy.png\"\r\nContent-Type: image/png\r\n\r\n" + string(dummy0),
				"Content-Disposition: form-data; name=\"upload1\"; filename=\"dummy.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n" + string(dummy1),
				"Content-Disposition: form-data; name=\"name\"\r\n\r\nbob",
			},
			"multipart/form-data; boundary=123456789012345678901234567890abcdefghijklmnopqrstuvwxyz",
		},
		{
			`
- upload0: 'testdata/dummy.png'
- upload1: 'testdata/dummy.jpg'
- name: 'bob'`,
			MediaTypeMultipartFormData,
			[]string{
				"--123456789012345678901234567890abcdefghijklmnopqrstuvwxyz\r\n",
				"Content-Disposition: form-data; name=\"upload0\"; filename=\"dummy.png\"\r\nContent-Type: image/png\r\n\r\n" + string(dummy0),
				"Content-Disposition: form-data; name=\"upload1\"; filename=\"dummy.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n" + string(dummy1),
				"Content-Disposition: form-data; name=\"name\"\r\n\r\nbob",
			},
			"multipart/form-data; boundary=123456789012345678901234567890abcdefghijklmnopqrstuvwxyz",
		},
		{
			`
file:
  - 'testdata/dummy.png'
  - 'testdata/dummy.jpg'`,
			MediaTypeMultipartFormData,
			[]string{
				"--123456789012345678901234567890abcdefghijklmnopqrstuvwxyz\r\n",
				"Content-Disposition: form-data; name=\"file\"; filename=\"dummy.png\"\r\nContent-Type: image/png\r\n\r\n" + string(dummy0),
				"Content-Disposition: form-data; name=\"file\"; filename=\"dummy.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n" + string(dummy1),
			},
			"multipart/form-data; boundary=123456789012345678901234567890abcdefghijklmnopqrstuvwxyz",
		},
	}

	for idx, tt := range multitests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			var b interface{}
			if err := yaml.Unmarshal([]byte(tt.in), &b); err != nil {
				t.Error(err)
				return
			}
			r := &httpRequest{
				mediaType:         tt.mediaType,
				body:              b,
				multipartBoundary: testutil.MultipartBoundary,
			}
			body, err := r.encodeBody()
			if err != nil {
				t.Error(err)
				return
			}
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, body); err != nil {
				t.Error(err)
				return
			}
			got := buf.String()
			for _, wb := range tt.wantContainRequestBody {
				if !strings.Contains(string(got), wb) {
					t.Errorf("got %v\nwant to contain %v", string(got), wb)
				}
			}
			contentType := r.multipartWriter.FormDataContentType()
			if contentType != tt.wantContentType {
				t.Errorf("got %v\nwant %v", got, tt.wantContentType)
			}
		})
	}
}

func TestRequestBodyForMultipart_onServer(t *testing.T) {
	dummy0, err := os.ReadFile("testdata/dummy.png")
	if err != nil {
		t.Fatal(err)
	}
	dummy1, err := os.ReadFile("testdata/dummy.jpg")
	if err != nil {
		t.Fatal(err)
	}

	req := &httpRequest{
		path:      "/upload",
		method:    http.MethodPost,
		mediaType: MediaTypeMultipartFormData,
		body: map[string]interface{}{
			"username": "bob",
			"upload0":  "testdata/dummy.png",
			"upload1":  "testdata/dummy.jpg",
		},
	}
	wantContainRequestBody := []string{
		"Content-Disposition: form-data; name=\"upload0\"; filename=\"dummy.png\"\r\nContent-Type: image/png\r\n\r\n" + string(dummy0),
		"Content-Disposition: form-data; name=\"upload1\"; filename=\"dummy.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n" + string(dummy1),
		"Content-Disposition: form-data; name=\"username\"\r\n\r\nbob",
	}

	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	hs, hr := testutil.HTTPServerAndRouter(t)

	r, err := newHTTPRunner("req", hs.URL)
	r.multipartBoundary = testutil.MultipartBoundary
	if err != nil {
		t.Error(err)
		return
	}
	r.operator = o
	if err := r.Run(ctx, req); err != nil {
		t.Error(err)
		return
	}
	rr := hr.Requests()[0]
	var save io.ReadCloser
	save, rr.Body, err = drainBody(rr.Body)
	if err != nil {
		t.Error(err)
		return
	}
	gotBody, err := io.ReadAll(save)
	if err != nil {
		t.Error(err)
		return
	}
	for _, wb := range wantContainRequestBody {
		if !strings.Contains(string(gotBody), wb) {
			t.Errorf("got %v\nwant to contain %v", string(gotBody), wb)
		}
	}

	f0, _, err := rr.FormFile("upload0")
	if err != nil {
		t.Error(err)
	}
	f1, _, err := rr.FormFile("upload1")
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(func() {
		_ = f0.Close()
		_ = f1.Close()
	})
	got0, err := io.ReadAll(f0)
	if err != nil {
		t.Error(err)
	}
	got1, err := io.ReadAll(f1)
	if err != nil {
		t.Error(err)
	}
	if diff := cmp.Diff(got0, dummy0, nil); diff != "" {
		t.Errorf("%s", diff)
	}
	if diff := cmp.Diff(got1, dummy1, nil); diff != "" {
		t.Errorf("%s", diff)
	}
}

func TestMergeURL(t *testing.T) {
	tests := []struct {
		endpoint string
		path     string
		want     string
	}{
		{"https://git.example.com/api/v3", "/orgs/octokit/repos", "https://git.example.com/api/v3/orgs/octokit/repos"},
		{"https://git.example.com/api/v3", "/repos/vmg/redcarpet/issues?state=closed", "https://git.example.com/api/v3/repos/vmg/redcarpet/issues?state=closed"},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		got, err := mergeURL(u, tt.path)
		if err != nil {
			t.Error(err)
			continue
		}
		if got.String() != tt.want {
			t.Errorf("got %v\nwant %v", got.String(), tt.want)
		}
	}
}

func TestHTTPRunnerWithHandler(t *testing.T) {
	tests := []struct {
		req         *httpRequest
		pattern     string
		handlerFunc func(w http.ResponseWriter, r *http.Request)
		want        int
	}{
		{
			&httpRequest{
				path:      "/users/k1LoW",
				method:    http.MethodGet,
				mediaType: MediaTypeApplicationJSON,
			},
			"/users/k1LoW",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("hello k1LoW!"))
			},
			http.StatusOK,
		},
		{
			&httpRequest{
				path:      "/users/k1LoW",
				method:    http.MethodGet,
				mediaType: MediaTypeApplicationJSON,
			},
			"/users/unknownuser",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("hello k1LoW!"))
			},
			http.StatusNotFound,
		},
		{
			&httpRequest{
				path:      "/users/newline",
				method:    http.MethodGet,
				mediaType: MediaTypeApplicationJSON,
				body:      map[string]interface{}{"key": "new\n", "value": "line\r\n"},
			},
			"/users/newline",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{ key: "hello\n", value: "world\r\n!" }`))
			},
			http.StatusOK,
		},
	}
	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for i, tt := range tests {
		s := http.NewServeMux()
		s.HandleFunc(tt.pattern, tt.handlerFunc)
		r, err := newHTTPRunnerWithHandler(t.Name(), s)
		if err != nil {
			t.Fatal(err)
		}
		r.operator = o
		if err := r.Run(ctx, tt.req); err != nil {
			t.Error(err)
			continue
		}
		res, ok := r.operator.store.steps[i]["res"].(map[string]interface{})
		if !ok {
			t.Fatalf("invalid steps res: %v", r.operator.store.steps[i]["res"])
		}
		got, ok := res["status"].(int)
		if !ok {
			t.Fatalf("invalid res status: %v", res["status"])
		}
		if got != tt.want {
			t.Errorf("got %v\nwant %v", got, tt.want)
		}
	}
}

func TestNotFollowRedirect(t *testing.T) {
	tests := []struct {
		req               *httpRequest
		notFollowRedirect bool
		want              int
	}{
		{
			&httpRequest{
				path:    "/redirect",
				method:  http.MethodGet,
				headers: map[string]string{},
			},
			false,
			http.StatusNotFound,
		},
		{
			&httpRequest{
				path:    "/redirect",
				method:  http.MethodGet,
				headers: map[string]string{},
			},
			true,
			http.StatusFound,
		},
	}
	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	hs := testutil.HTTPServer(t)
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.req), func(t *testing.T) {
			r, err := newHTTPRunner("req", hs.URL)
			if err != nil {
				t.Fatal(err)
			}
			r.operator = o
			if tt.notFollowRedirect {
				r.client.CheckRedirect = notFollowRedirectFn
			}
			if err := r.Run(ctx, tt.req); err != nil {
				t.Error(err)
				return
			}
			res, ok := r.operator.store.latest()["res"].(map[string]interface{})
			if !ok {
				t.Fatalf("invalid res: %#v", r.operator.store.latest()["res"])
			}
			got, ok := res["status"].(int)
			if !ok {
				t.Fatalf("invalid res status: %v", res["status"])
			}
			if got != tt.want {
				t.Errorf("got %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestHTTPCerts(t *testing.T) {
	tests := []struct {
		setCacert       bool
		setCertificates bool
		wantErr         bool
	}{
		{false, false, true},
		{true, false, true},
		{true, true, false},
	}
	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	hs := testutil.HTTPSServer(t)
	req := &httpRequest{
		path:    "/users/1",
		method:  http.MethodGet,
		headers: map[string]string{},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r, err := newHTTPRunner("req", hs.URL)
			if err != nil {
				t.Fatal(err)
			}
			r.operator = o
			if tt.setCacert {
				r.cacert = testutil.Cacert
			}
			if tt.setCertificates {
				r.cert = testutil.Cert
				r.key = testutil.Key
			}
			if err := r.Run(ctx, req); err != nil {
				if !tt.wantErr {
					t.Errorf("got %v", err)
				}
				return
			}
			if tt.wantErr {
				t.Error("want err")
			}
		})
	}
}

func TestHTTPRunnerInitializeWithCerts(t *testing.T) {
	tests := []struct {
		setCacert       bool
		setCertificates bool
		wantErr         bool
	}{
		{false, false, true},
		{true, false, true},
		{true, true, false},
	}
	ctx := context.Background()
	o, err := New()
	if err != nil {
		t.Fatal(err)
	}
	hs := testutil.HTTPSServer(t)
	req := &httpRequest{
		path:    "/users/1",
		method:  http.MethodGet,
		headers: map[string]string{},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			u, err := url.Parse(hs.URL)
			if err != nil {
				t.Fatal(err)
			}
			r := &httpRunner{
				name:      "req",
				endpoint:  u,
				client:    &http.Client{},
				validator: newNopValidator(),
				operator:  o,
			}
			if tt.setCacert {
				r.cacert = testutil.Cacert
			}
			if tt.setCertificates {
				r.cert = testutil.Cert
				r.key = testutil.Key
			}
			if err := r.Run(ctx, req); err != nil {
				if !tt.wantErr {
					t.Errorf("got %v", err)
				}
				return
			}
			if tt.wantErr {
				t.Error("want err")
			}
		})
	}
}
