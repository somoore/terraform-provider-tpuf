package provider

// These tests model the live-API behavior discovered on 2026-07-08 by probing
// production turbopuffer directly:
//
//   - POST /v2/namespaces/{ns} with no write operations -> 400 "no writes provided"
//   - a no-op delete on a nonexistent namespace is ignored (does NOT create it)
//
// which forced Create's shape: existence check, then placeholder upsert
// (carrying the schema), then placeholder delete. The mock server below
// enforces those exact semantics so a regression in Create's request sequence
// fails here without needing live credentials.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tpuf "github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/option"
)

// mockTpuf simulates the turbopuffer API's namespace-creation semantics.
type mockTpuf struct {
	exists   bool     // namespace pre-exists
	requests []string // "METHOD path" log
	upserted []string // ids currently in the namespace
	schema   map[string]any
}

func (m *mockTpuf) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/metadata"):
			if !m.exists {
				w.WriteHeader(404)
				w.Write([]byte(`{"error":"namespace was not found","status":"error"}`))
				return
			}
			w.Write([]byte(`{"schema":{"id":{"type":"string"}},"approx_row_count":0,"approx_logical_bytes":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","encryption":{"mode":"default"},"index":{"status":"up-to-date"}}`))

		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v2/namespaces/"):
			var body struct {
				UpsertRows []map[string]any          `json:"upsert_rows"`
				Deletes    []any                     `json:"deletes"`
				Schema     map[string]map[string]any `json:"schema"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("bad write body: %v", err)
			}

			// Live-API rule: a write with no operations is a 400.
			if len(body.UpsertRows) == 0 && len(body.Deletes) == 0 {
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"💔 no writes provided","status":"error"}`))
				return
			}

			// Live-API rule: no-op deletes on a nonexistent namespace are
			// ignored — the namespace is NOT created.
			if len(body.UpsertRows) == 0 && !m.exists && len(m.upserted) == 0 {
				w.Write([]byte(`{"status":"OK","message":"deleting from or patching empty namespace, ignoring request","rows_affected":0}`))
				return
			}

			for _, row := range body.UpsertRows {
				id, _ := row["id"].(string)
				m.upserted = append(m.upserted, id)
				m.exists = true
			}
			for _, d := range body.Deletes {
				id, _ := d.(string)
				for i, u := range m.upserted {
					if u == id {
						m.upserted = append(m.upserted[:i], m.upserted[i+1:]...)
						break
					}
				}
			}
			if body.Schema != nil {
				m.schema = map[string]any{}
				for k, v := range body.Schema {
					m.schema[k] = v
				}
			}
			w.Write([]byte(`{"status":"OK","message":"documents committed successfully","rows_affected":1}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}
}

func newMockClient(t *testing.T, m *mockTpuf) tpuf.Client {
	srv := httptest.NewServer(m.handler(t))
	t.Cleanup(srv.Close)
	return tpuf.NewClient(
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
}

func TestCreateEmptyNamespace_placeholderFlow(t *testing.T) {
	m := &mockTpuf{}
	client := newMockClient(t, m)

	schema := map[string]tpuf.AttributeSchemaConfigParam{
		"title": {Type: "string"},
	}
	err := createEmptyNamespace(context.Background(), client.Namespace("tf-test"), schema)
	if err != nil {
		t.Fatalf("createEmptyNamespace failed: %v", err)
	}

	// Exact request sequence: existence check, placeholder upsert, placeholder delete.
	want := []string{
		"GET /v1/namespaces/tf-test/metadata",
		"POST /v2/namespaces/tf-test",
		"POST /v2/namespaces/tf-test",
	}
	if len(m.requests) != len(want) {
		t.Fatalf("expected %d requests %v, got %v", len(want), want, m.requests)
	}
	for i := range want {
		if m.requests[i] != want[i] {
			t.Errorf("request %d: want %q, got %q", i, want[i], m.requests[i])
		}
	}

	// The namespace must end up empty (placeholder deleted) with the schema applied.
	if len(m.upserted) != 0 {
		t.Errorf("placeholder document was not cleaned up: %v", m.upserted)
	}
	if _, ok := m.schema["title"]; !ok {
		t.Errorf("schema was not sent with the creating write: %v", m.schema)
	}
}

func TestCreateEmptyNamespace_alreadyExists(t *testing.T) {
	m := &mockTpuf{exists: true}
	client := newMockClient(t, m)

	err := createEmptyNamespace(context.Background(), client.Namespace("tf-test"), nil)
	if !errors.Is(err, errNamespaceExists) {
		t.Fatalf("expected errNamespaceExists, got: %v", err)
	}

	// Must stop at the existence check — never write into existing data.
	if len(m.requests) != 1 || !strings.HasPrefix(m.requests[0], "GET ") {
		t.Errorf("expected only the metadata existence check, got: %v", m.requests)
	}
}

// TestCreateEmptyNamespace_noopDeleteDoesNotCreate documents WHY the
// placeholder upsert exists: a delete-only write on a nonexistent namespace is
// ignored by the API, so it can never be used to instantiate one.
func TestCreateEmptyNamespace_noopDeleteDoesNotCreate(t *testing.T) {
	m := &mockTpuf{}
	client := newMockClient(t, m)

	ns := client.Namespace("tf-test")
	if _, err := ns.Write(context.Background(), tpuf.NamespaceWriteParams{Deletes: []any{"nope"}}); err != nil {
		t.Fatalf("no-op delete should succeed (ignored), got: %v", err)
	}
	if m.exists {
		t.Fatal("no-op delete must not create the namespace")
	}

	// And an empty write is rejected outright.
	if _, err := ns.Write(context.Background(), tpuf.NamespaceWriteParams{}); err == nil {
		t.Fatal("empty write should be rejected with 400")
	}
}
