package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/packages/param"
)

var _ datasource.DataSource = &NamespacesDataSource{}

func NewNamespacesDataSource() datasource.DataSource {
	return &NamespacesDataSource{}
}

type NamespacesDataSource struct {
	client *tpuf.Client
}

type NamespacesDataSourceModel struct {
	Prefix types.String   `tfsdk:"prefix"`
	Names  []types.String `tfsdk:"names"`
}

func (d *NamespacesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespaces"
}

func (d *NamespacesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists turbopuffer namespace names, optionally filtered by prefix.",
		Attributes: map[string]schema.Attribute{
			"prefix": schema.StringAttribute{
				Optional:    true,
				Description: "Only return namespaces whose name matches this prefix.",
			},
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Namespace names.",
			},
		},
	}
}

func (d *NamespacesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*tpuf.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("expected *tpuf.Client, got: %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *NamespacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NamespacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := tpuf.NamespacesParams{}
	if !data.Prefix.IsNull() {
		params.Prefix = param.NewOpt(data.Prefix.ValueString())
	}

	var names []types.String
	iter := d.client.NamespacesAutoPaging(ctx, params)
	for iter.Next() {
		names = append(names, types.StringValue(iter.Current().ID))
	}
	if err := iter.Err(); err != nil {
		resp.Diagnostics.AddError("Error listing namespaces", err.Error())
		return
	}

	data.Names = names
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
