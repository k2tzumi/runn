package runn

import (
	"net/http"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
)

func TestParseHTTPRequest(t *testing.T) {
	tests := []struct {
		in      string
		want    *httpRequest
		wantErr bool
	}{
		{
			`
/login:
  post:
    body:
      application/json:
        key: value
`,
			&httpRequest{
				path:      "/login",
				method:    http.MethodPost,
				mediaType: MediaTypeApplicationJSON,
				headers:   map[string]string{},
				body: map[string]interface{}{
					"key": "value",
				},
			},
			false,
		},
		{
			`
/login:
  post:
    body:
      application/json:
        password: "{{ vars.password }}"	
`,
			&httpRequest{
				path:      "/login",
				method:    http.MethodPost,
				mediaType: MediaTypeApplicationJSON,
				headers:   map[string]string{},
				body: map[string]interface{}{
					"key": "value",
				},
			},
			false,
		},
		{
			`
/users/k1LoW:
  get: 
    body: null
`,
			&httpRequest{
				path:      "/users/k1LoW",
				method:    http.MethodGet,
				mediaType: "",
				headers:   map[string]string{},
				body:      nil,
			},
			false,
		},
		{
			`
/users/k1LoW:
  get: null
`,
			nil,
			true,
		},
		{
			`
/users/k1LoW:
  post: 
    body: null
`,
			nil,
			true,
		},
	}

	for _, tt := range tests {
		var v map[string]interface{}
		if err := yaml.Unmarshal([]byte(tt.in), &v); err != nil {
			t.Fatal(err)
		}
		got, err := parseHTTPRequest(v)
		if err != nil {
			if !tt.wantErr {
				t.Error(err)
			}
			continue
		}
		if tt.wantErr {
			t.Error("want error")
		}
		opts := cmp.AllowUnexported(httpRequest{})
		if diff := cmp.Diff(got, tt.want, opts); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestParseDBQuery(t *testing.T) {
	tests := []struct {
		in      string
		want    *dbQuery
		wantErr bool
	}{
		{
			`
query: SELECT * FROM users;
`,
			&dbQuery{
				stmt: "SELECT * FROM users;",
			},
			false,
		},
		{
			`
query: |
  SELECT * FROM users;
`,
			&dbQuery{
				stmt: "SELECT * FROM users;",
			},
			false,
		},
	}

	for _, tt := range tests {
		var v map[string]interface{}
		if err := yaml.Unmarshal([]byte(tt.in), &v); err != nil {
			t.Fatal(err)
		}
		got, err := parseDBQuery(v)
		if err != nil {
			if !tt.wantErr {
				t.Error(err)
			}
			continue
		}
		if tt.wantErr {
			t.Error("want error")
		}
		opts := cmp.AllowUnexported(dbQuery{})
		if diff := cmp.Diff(got, tt.want, opts); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestParseExecCommand(t *testing.T) {
	tests := []struct {
		in      string
		want    *execCommand
		wantErr bool
	}{
		{
			`
command: echo hello > test.txt
`,
			&execCommand{
				command: "echo hello > test.txt",
			},
			false,
		},
		{
			`
command: echo hello > test.txt
stdin: |
  alice
  bob
  charlie
`,
			&execCommand{
				command: "echo hello > test.txt",
				stdin:   "alice\nbob\ncharlie\n",
			},
			false,
		},
		{
			`
stdin: |
  alice
  bob
  charlie
`,
			nil,
			true,
		},
	}

	for _, tt := range tests {
		var v map[string]interface{}
		if err := yaml.Unmarshal([]byte(tt.in), &v); err != nil {
			t.Fatal(err)
		}
		got, err := parseExecCommand(v)
		if err != nil {
			if !tt.wantErr {
				t.Error(err)
			}
			continue
		}
		if tt.wantErr {
			t.Error("want error")
		}
		opts := cmp.AllowUnexported(execCommand{})
		if diff := cmp.Diff(got, tt.want, opts); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}

func TestTrimDelimiter(t *testing.T) {
	tests := []struct {
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			map[string]interface{}{"k": `"Hello"`},
			map[string]interface{}{"k": "Hello"},
		},
		{
			map[string]interface{}{"k": `'Hello'`},
			map[string]interface{}{"k": "Hello"},
		},
		{
			map[string]interface{}{"k": `"'Hello'"`},
			map[string]interface{}{"k": "Hello"},
		},
		{
			map[string]interface{}{"k": `"'He\"llo'"`},
			map[string]interface{}{"k": "He\"llo"},
		},
		{
			map[string]interface{}{"k": `"\"Hello\""`},
			map[string]interface{}{"k": `Hello`},
		},
		{
			map[string]interface{}{"k": []interface{}{
				`"Hello"`,
				2,
			}},
			map[string]interface{}{"k": []interface{}{
				"Hello",
				2,
			}},
		},
	}
	for _, tt := range tests {
		got := trimDelimiter(tt.in)
		if diff := cmp.Diff(got, tt.want, nil); diff != "" {
			t.Errorf("%s", diff)
		}
	}
}
