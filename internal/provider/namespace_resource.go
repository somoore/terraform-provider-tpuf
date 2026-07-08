package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tpuf "github.com/turbopuffer/turbopuffer-go"
	"github.com/turbopuffer/turbopuffer-go/packages/param"
)

var (
	_ resource.Resource                = &NamespaceResource{}
	_ resource.ResourceWithImportState = &NamespaceResource{}
)

func NewNamespaceResource() resource.Resource {
	return &NamespaceResource{}
}

type NamespaceResource struct {
	client *tpuf.Client
}

type NamespaceResourceModel struct {
	Name            types.String `tfsdk:"name"`
	Schema          types.Map    `tfsdk:"schema"`
	PinningReplicas types.Int64  `tfsdk:"pinning_replicas"`
	ApproxRowCount  types.Int64  `tfsdk:"approx_row_count"`
}

type attributeSchemaModel struct {
	Type           types.String `tfsdk:"type"`
	Filterable     types.Bool   `tfsdk:"filterable"`
	FullTextSearch types.Bool   `tfsdk:"full_text_search"`
	Glob           types.Bool   `tfsdk:"glob"`
	Regex          types.Bool   `tfsdk:"regex"`
	Ann            types.Bool   `tfsdk:"ann"`
}

func attributeSchemaObjectType() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required:    true,
			Description: "Attribute data type, e.g. string, int, bool, [1536]f32.",
		},
		"filterable": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Whether the attribute can be used in filters.",
		},
		"full_text_search": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Whether the attribute is indexed for BM25 full-text search.",
		},
		"glob": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Whether glob filters are enabled on the attribute.",
		},
		"regex": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Whether regex filters are enabled on the attribute.",
		},
		"ann": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Whether an approximate nearest neighbor index is built for the attribute.",
		},
	}
}

func (r *NamespaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (r *NamespaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a turbopuffer namespace: its existence, attribute schema, and pinning configuration.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The namespace name. Changing this forces a new namespace.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
			},
			"schema": schema.MapNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Attribute schema for documents in this namespace, keyed by attribute name. The built-in `id` attribute is managed by turbopuffer and excluded.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: attributeSchemaObjectType(),
				},
			},
			"pinning_replicas": schema.Int64Attribute{
				Optional:    true,
				Description: "Number of warm read replicas to pin for this namespace. Omit to disable pinning.",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"approx_row_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Approximate number of rows currently in the namespace.",
			},
		},
	}
}

func (r *NamespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*tpuf.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("expected *tpuf.Client, got: %T", req.ProviderData))
		return
	}
	r.client = client
}

func schemaToParam(ctx context.Context, m types.Map) (map[string]tpuf.AttributeSchemaConfigParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}

	attrs := make(map[string]attributeSchemaModel, len(m.Elements()))
	diags.Append(m.ElementsAs(ctx, &attrs, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make(map[string]tpuf.AttributeSchemaConfigParam, len(attrs))
	for name, a := range attrs {
		p := tpuf.AttributeSchemaConfigParam{
			Type: a.Type.ValueString(),
		}
		if !a.Filterable.IsNull() && !a.Filterable.IsUnknown() {
			p.Filterable = param.NewOpt(a.Filterable.ValueBool())
		}
		if !a.Glob.IsNull() && !a.Glob.IsUnknown() {
			p.Glob = param.NewOpt(a.Glob.ValueBool())
		}
		if !a.Regex.IsNull() && !a.Regex.IsUnknown() {
			p.Regex = param.NewOpt(a.Regex.ValueBool())
		}
		if !a.FullTextSearch.IsNull() && !a.FullTextSearch.IsUnknown() && a.FullTextSearch.ValueBool() {
			p.FullTextSearch = &tpuf.FullTextSearchConfigParam{}
		}
		if !a.Ann.IsNull() && !a.Ann.IsUnknown() && a.Ann.ValueBool() {
			p.Ann = tpuf.AttributeSchemaConfigAnnParam{}
		}
		out[name] = p
	}
	return out, diags
}

func schemaFromAPI(ctx context.Context, s map[string]tpuf.AttributeSchemaConfig) (types.Map, diag.Diagnostics) {
	objType := schemaObjectAttrType()

	elems := make(map[string]attributeSchemaModel, len(s))
	for name, cfg := range s {
		// turbopuffer injects a built-in `id` attribute into every namespace
		// schema; it isn't user-manageable, so keep it out of state.
		if name == "id" {
			continue
		}
		elems[name] = attributeSchemaModel{
			Type:           types.StringValue(cfg.Type),
			Filterable:     types.BoolValue(cfg.Filterable),
			FullTextSearch: types.BoolValue(cfg.JSON.FullTextSearch.Valid()),
			Glob:           types.BoolValue(cfg.Glob),
			Regex:          types.BoolValue(cfg.Regex),
			Ann:            types.BoolValue(cfg.JSON.Ann.Valid()),
		}
	}

	if len(elems) == 0 {
		return types.MapNull(objType), nil
	}

	return types.MapValueFrom(ctx, objType, elems)
}

func schemaObjectAttrType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"type":             types.StringType,
		"filterable":       types.BoolType,
		"full_text_search": types.BoolType,
		"glob":             types.BoolType,
		"regex":            types.BoolType,
		"ann":              types.BoolType,
	}}
}

// errNamespaceExists signals that Create found the namespace already present.
var errNamespaceExists = errors.New("namespace already exists")

// placeholderID is the throwaway document id used to instantiate a namespace.
const placeholderID = "tf-provider-tpuf-placeholder"

// createEmptyNamespace brings a namespace into existence with the given
// schema and zero documents.
//
// Live-API facts (verified 2026-07-08) that dictate this shape:
//   - A write with no operations is rejected: 400 "no writes provided".
//   - A no-op delete against a nonexistent namespace is ignored and does NOT
//     create it.
//
// So the only path is: upsert a placeholder row carrying the schema, then
// delete it — the namespace persists empty with the schema intact. An
// existence check runs first so Create never writes into existing data.
//
// ponytail: placeholder id is a string, so Terraform-created namespaces get a
// string id type; expose an id_type attribute if uint/uuid ids are needed.
func createEmptyNamespace(ctx context.Context, ns tpuf.Namespace, schemaParam map[string]tpuf.AttributeSchemaConfigParam) error {
	// Terraform contract: Create must not silently adopt an existing resource.
	if _, err := ns.Metadata(ctx, tpuf.NamespaceMetadataParams{}); err == nil {
		return errNamespaceExists
	} else if !isNotFoundErr(err) {
		return fmt.Errorf("checking for existing namespace: %w", err)
	}

	writeParams := tpuf.NamespaceWriteParams{
		UpsertRows: []tpuf.RowParam{{"id": placeholderID}},
	}
	if schemaParam != nil {
		writeParams.Schema = schemaParam
	}
	if _, err := ns.Write(ctx, writeParams); err != nil {
		return fmt.Errorf("creating namespace: %w", err)
	}
	if _, err := ns.Write(ctx, tpuf.NamespaceWriteParams{Deletes: []any{placeholderID}}); err != nil {
		return fmt.Errorf("removing placeholder document from new namespace: %w", err)
	}
	return nil
}

func (r *NamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	schemaParam, diags := schemaToParam(ctx, data.Schema)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := r.client.Namespace(data.Name.ValueString())

	if err := createEmptyNamespace(ctx, ns, schemaParam); err != nil {
		if errors.Is(err, errNamespaceExists) {
			resp.Diagnostics.AddError(
				"Namespace already exists",
				fmt.Sprintf("Namespace %q already exists. Use `terraform import` to manage it.", data.Name.ValueString()),
			)
		} else {
			resp.Diagnostics.AddError("Error creating namespace", err.Error())
		}
		return
	}

	if !data.PinningReplicas.IsNull() {
		_, err := ns.UpdateMetadata(ctx, tpuf.NamespaceUpdateMetadataParams{
			NamespaceMetadataPatch: tpuf.NamespaceMetadataPatchParam{
				Pinning: tpuf.PinningConfigParam{Replicas: param.NewOpt(data.PinningReplicas.ValueInt64())},
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("Error setting namespace pinning", err.Error())
			return
		}
	}

	notFound, diags := r.readInto(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || notFound {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NamespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	notFound, diags := r.readInto(ctx, &data)
	if notFound {
		resp.Diagnostics.AddWarning(
			"Namespace not found",
			fmt.Sprintf("Namespace %q no longer exists and will be removed from state.", data.Name.ValueString()),
		)
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// readInto populates data from the API. The first return value is true if the
// namespace no longer exists.
func (r *NamespaceResource) readInto(ctx context.Context, data *NamespaceResourceModel) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	ns := r.client.Namespace(data.Name.ValueString())
	meta, err := ns.Metadata(ctx, tpuf.NamespaceMetadataParams{})
	if err != nil {
		if isNotFoundErr(err) {
			return true, diags
		}
		diags.AddError("Error reading namespace", err.Error())
		return false, diags
	}

	data.ApproxRowCount = types.Int64Value(meta.ApproxRowCount)

	schemaMap, sDiags := schemaFromAPI(ctx, meta.Schema)
	diags.Append(sDiags...)
	data.Schema = schemaMap

	if meta.Pinning.JSON.Status.Valid() || meta.Pinning.Replicas != 0 {
		data.PinningReplicas = types.Int64Value(meta.Pinning.Replicas)
	} else {
		data.PinningReplicas = types.Int64Null()
	}

	return false, diags
}

func (r *NamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state NamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := r.client.Namespace(plan.Name.ValueString())

	if !plan.Schema.Equal(state.Schema) && !plan.Schema.IsNull() && !plan.Schema.IsUnknown() {
		schemaParam, diags := schemaToParam(ctx, plan.Schema)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if _, err := ns.UpdateSchema(ctx, tpuf.NamespaceUpdateSchemaParams{Schema: schemaParam}); err != nil {
			resp.Diagnostics.AddError("Error updating namespace schema", err.Error())
			return
		}
	}

	if !plan.PinningReplicas.Equal(state.PinningReplicas) {
		pinning := param.NullStruct[tpuf.PinningConfigParam]() // explicit null removes pinning
		if !plan.PinningReplicas.IsNull() {
			pinning = tpuf.PinningConfigParam{Replicas: param.NewOpt(plan.PinningReplicas.ValueInt64())}
		}
		_, err := ns.UpdateMetadata(ctx, tpuf.NamespaceUpdateMetadataParams{
			NamespaceMetadataPatch: tpuf.NamespaceMetadataPatchParam{Pinning: pinning},
		})
		if err != nil {
			resp.Diagnostics.AddError("Error updating namespace pinning", err.Error())
			return
		}
	}

	_, diags := r.readInto(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := r.client.Namespace(data.Name.ValueString())
	if _, err := ns.DeleteAll(ctx, tpuf.NamespaceDeleteAllParams{}); err != nil && !isNotFoundErr(err) {
		resp.Diagnostics.AddError("Error deleting namespace", err.Error())
	}
}

func (r *NamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
