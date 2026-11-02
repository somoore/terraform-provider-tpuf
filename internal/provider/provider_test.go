package provider

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateRegion(t *testing.T) {
	valid := []string{
		"", // unset: the provider falls back to TURBOPUFFER_REGION
		"a",
		"a1",
		"gcp-us-central1",
		"aws-us-east-1",
		"gcp-us-central1-2",
		strings.Repeat("a", 63),
	}
	for _, region := range valid {
		if err := validateRegion(region); err != nil {
			t.Errorf("validateRegion(%q) = %v, want nil", region, err)
		}
	}

	// Every one of these can rewrite the host of the SDK base URL (or is not a
	// region slug at all).
	invalid := []string{
		"attacker.example/x",
		"x@evil.example/",
		"evil.example/#",
		"127.0.0.1:58496/",
		"attacker.example:8443",
		"..",
		"a/b",
		"gcp-us-central1?x",
		"gcp-us-central1#",
		"gcp_us",
		"gcp.us",
		"GCP-US-CENTRAL1",
		" gcp-us-central1",
		"gcp-us-central1 ",
		"-gcp-us-central1",
		"gcp-us-central1-",
		strings.Repeat("a", 64),
	}
	for _, region := range invalid {
		if err := validateRegion(region); err == nil {
			t.Errorf("validateRegion(%q) = nil, want an error", region)
		}
	}
}

// TestRegionEnvFallbackIsValidated covers the path that bypasses schema
// validators: TURBOPUFFER_REGION is read in Configure.
func TestRegionEnvFallbackIsValidated(t *testing.T) {
	t.Setenv("TURBOPUFFER_REGION", "attacker.example/")
	if _, err := resolveRegion(""); err == nil {
		t.Fatal("invalid TURBOPUFFER_REGION must be rejected")
	}

	t.Setenv("TURBOPUFFER_REGION", "aws-us-east-1")
	got, err := resolveRegion("")
	if err != nil || got != "aws-us-east-1" {
		t.Fatalf("resolveRegion from env = (%q, %v), want (aws-us-east-1, nil)", got, err)
	}

	// An explicit provider attribute wins over the environment, but is still
	// validated on its own.
	t.Setenv("TURBOPUFFER_REGION", "attacker.example/")
	got, err = resolveRegion("gcp-us-central1")
	if err != nil || got != "gcp-us-central1" {
		t.Fatalf("resolveRegion(config) = (%q, %v), want (gcp-us-central1, nil)", got, err)
	}
	if _, err := resolveRegion("attacker.example/"); err == nil {
		t.Fatal("invalid configured region must be rejected")
	}
}

func TestRegionSchemaValidatorRejectsHostEscape(t *testing.T) {
	var resp provider.SchemaResponse
	New("test")().Schema(context.Background(), provider.SchemaRequest{}, &resp)

	attr, ok := resp.Schema.Attributes["region"].(providerschema.StringAttribute)
	if !ok {
		t.Fatalf("region attribute is %T, want schema.StringAttribute", resp.Schema.Attributes["region"])
	}
	if len(attr.Validators) == 0 {
		t.Fatal("region attribute has no validators; terraform validate would accept a host-escaping value")
	}

	ctx := context.Background()
	validate := func(region string) bool {
		var vresp validator.StringResponse
		for _, v := range attr.Validators {
			v.ValidateString(ctx, validator.StringRequest{
				Path:        path.Root("region"),
				ConfigValue: types.StringValue(region),
			}, &vresp)
		}
		return vresp.Diagnostics.HasError()
	}

	for _, bad := range []string{"attacker.example/", "x@evil.example/", "evil.example/#", "127.0.0.1:58496/"} {
		if !validate(bad) {
			t.Errorf("schema accepted host-escaping region %q", bad)
		}
	}
	for _, good := range []string{"gcp-us-central1", "aws-us-east-1"} {
		if validate(good) {
			t.Errorf("schema rejected valid region %q", good)
		}
	}
}

// TestValidRegionsKeepSDKHost is the sink-level assertion: the values the
// validator accepts still resolve to the intended turbopuffer host under the
// SDK's textual substitution.
func TestValidRegionsKeepSDKHost(t *testing.T) {
	const template = "https://REGION.turbopuffer.com/"
	for _, region := range []string{"gcp-us-central1", "aws-us-east-1", "gcp-us-central1-2"} {
		u, err := url.Parse(strings.ReplaceAll(template, "REGION", region))
		if err != nil {
			t.Fatalf("parse for region %q: %v", region, err)
		}
		if want := region + ".turbopuffer.com"; u.Host != want {
			t.Errorf("region %q resolved to host %q, want %q", region, u.Host, want)
		}
	}
}
