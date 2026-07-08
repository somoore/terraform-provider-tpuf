package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
)

func TestSchemaRoundTrip(t *testing.T) {
	ctx := context.Background()

	api := map[string]tpuf.AttributeSchemaConfig{
		"vector": {Type: "[3]f32", Filterable: false},
	}

	m, diags := schemaFromAPI(ctx, api)
	if diags.HasError() {
		t.Fatalf("schemaFromAPI errored: %v", diags)
	}
	if m.IsNull() {
		t.Fatal("expected non-null map")
	}

	param, diags := schemaToParam(ctx, m)
	if diags.HasError() {
		t.Fatalf("schemaToParam errored: %v", diags)
	}
	if param["vector"].Type != "[3]f32" {
		t.Fatalf("expected type [3]f32, got %q", param["vector"].Type)
	}
}

func TestSchemaFromAPIEmpty(t *testing.T) {
	m, diags := schemaFromAPI(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.IsNull() {
		t.Fatal("expected null map for empty schema")
	}
	_ = types.MapNull
}

func TestIsNotFoundErr(t *testing.T) {
	err := &tpuf.Error{StatusCode: 404}
	if !isNotFoundErr(err) {
		t.Fatal("expected 404 to be detected as not found")
	}
	err2 := &tpuf.Error{StatusCode: 500}
	if isNotFoundErr(err2) {
		t.Fatal("expected 500 to not be detected as not found")
	}
}
