package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/option"
)

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

	region := data.Region.ValueString()
	if region == "" {
		region = os.Getenv("TURBOPUFFER_REGION")
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
