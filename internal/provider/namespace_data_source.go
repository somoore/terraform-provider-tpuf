package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
)

var _ datasource.DataSource = &NamespaceDataSource{}

func NewNamespaceDataSource() datasource.DataSource {
	return &NamespaceDataSource{}
}

type NamespaceDataSource struct {
	client *tpuf.Client
}

func (d *NamespaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *NamespaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing turbopuffer namespace's schema and metadata.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The namespace name.",
			},
			"schema": schema.MapNestedAttribute{
				Computed:    true,
				Description: "Attribute schema for documents in this namespace, keyed by attribute name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Attribute data type, e.g. string, int, bool, [1536]f32.",
						},
						"filterable": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the attribute can be used in filters.",
						},
						"full_text_search": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the attribute is indexed for BM25 full-text search.",
						},
						"glob": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether glob filters are enabled on the attribute.",
						},
						"regex": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether regex filters are enabled on the attribute.",
						},
						"ann": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether an approximate nearest neighbor index is built for the attribute.",
						},
					},
				},
			},
			"pinning_replicas": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of warm read replicas pinned for this namespace.",
			},
			"approx_row_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Approximate number of rows currently in the namespace.",
			},
		},
	}
}

func (d *NamespaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NamespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := d.client.Namespace(data.Name.ValueString())
	meta, err := ns.Metadata(ctx, tpuf.NamespaceMetadataParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading namespace", err.Error())
		return
	}

	data.ApproxRowCount = types.Int64Value(meta.ApproxRowCount)

	schemaMap, diags := schemaFromAPI(ctx, meta.Schema)
	resp.Diagnostics.Append(diags...)
	data.Schema = schemaMap

	if meta.Pinning.JSON.Status.Valid() || meta.Pinning.Replicas != 0 {
		data.PinningReplicas = types.Int64Value(meta.Pinning.Replicas)
	} else {
		data.PinningReplicas = types.Int64Null()
	}

	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
