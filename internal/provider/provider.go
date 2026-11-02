package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/option"
)

// regionPattern constrains the turbopuffer region slug. The SDK substitutes
// the region textually into "https://REGION.turbopuffer.com/", so anything
// outside this alphabet ("/", "@", "?", "#", ":", ".", whitespace, uppercase)
// can rewrite the request host and send the API key somewhere else.
var regionPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// validateRegion accepts an empty string (region unset) or a region slug of
// 1-63 characters: lowercase letters, digits and hyphens, starting and ending
// with a letter or digit.
func validateRegion(region string) error {
	if region == "" {
		return nil
	}
	if len(region) > 63 || !regionPattern.MatchString(region) {
		return fmt.Errorf("%q is not a valid turbopuffer region: expected 1-63 lowercase letters, digits or hyphens, starting and ending with a letter or digit (e.g. gcp-us-central1)", region)
	}
	return nil
}

// resolveRegion applies the documented provider-attribute/env-var precedence
// and validates the result. Schema validators only see the attribute, so the
// environment fallback must be validated here too.
func resolveRegion(configRegion string) (string, error) {
	region := configRegion
	if region == "" {
		region = os.Getenv("TURBOPUFFER_REGION")
	}
	if err := validateRegion(region); err != nil {
		return "", err
	}
	return region, nil
}

// regionValidator gives plan-time feedback for the region attribute. The
// Configure-time check remains authoritative because TURBOPUFFER_REGION
// bypasses schema validation.
type regionValidator struct{}

func (regionValidator) Description(context.Context) string {
	return "value must be a turbopuffer region slug (e.g. gcp-us-central1)"
}

func (v regionValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v regionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if err := validateRegion(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid region", err.Error())
	}
}

var _ provider.Provider = &TpufProvider{}

type TpufProvider struct {
	version string
}

type TpufProviderModel struct {
	APIKey types.String `tfsdk:"api_key"`
	Region types.String `tfsdk:"region"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TpufProvider{version: version}
	}
}

func (p *TpufProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tpuf"
	resp.Version = p.version
}

func (p *TpufProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with turbopuffer, a fast serverless vector database.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "turbopuffer API key. Defaults to the TURBOPUFFER_API_KEY environment variable.",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "turbopuffer region, e.g. gcp-us-central1 or aws-us-east-1. Defaults to the TURBOPUFFER_REGION environment variable.",
				Validators: []validator.String{
					regionValidator{},
				},
			},
		},
	}
}

func (p *TpufProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data TpufProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := data.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("TURBOPUFFER_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"Set the api_key provider attribute or the TURBOPUFFER_API_KEY environment variable.",
		)
		return
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}

	region, err := resolveRegion(data.Region.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid region",
			fmt.Sprintf("%s. The region controls the host the API key is sent to, so it must be a turbopuffer region slug.", err),
		)
		return
	}
	if region != "" {
		opts = append(opts, option.WithRegion(region))
	}

	client := tpuf.NewClient(opts...)
	resp.DataSourceData = &client
	resp.ResourceData = &client
}

func (p *TpufProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNamespaceResource,
	}
}

func (p *TpufProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNamespaceDataSource,
		NewNamespacesDataSource,
	}
}
