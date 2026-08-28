package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewImageSourceResource)
}

var (
	_ resource.Resource                   = (*imageSourceResource)(nil)
	_ resource.ResourceWithConfigure      = (*imageSourceResource)(nil)
	_ resource.ResourceWithImportState    = (*imageSourceResource)(nil)
	_ resource.ResourceWithValidateConfig = (*imageSourceResource)(nil)
)

// NewImageSourceResource returns the cycle_image_source resource.
func NewImageSourceResource() resource.Resource {
	return &imageSourceResource{}
}

type imageSourceResource struct {
	client *CycleClient
}

type imageSourceResourceModel struct {
	ID          types.String            `tfsdk:"id"`
	Identifier  types.String            `tfsdk:"identifier"`
	Name        types.String            `tfsdk:"name"`
	Type        types.String            `tfsdk:"type"`
	Description types.String            `tfsdk:"description"`
	Origin      *imageSourceOriginModel `tfsdk:"origin"`
	State       types.String            `tfsdk:"state"`
}

type imageSourceOriginModel struct {
	DockerHub      *imageSourceDockerHubModel      `tfsdk:"docker_hub"`
	DockerRegistry *imageSourceDockerRegistryModel `tfsdk:"docker_registry"`
	OciRegistry    *imageSourceOciRegistryModel    `tfsdk:"oci_registry"`
	DockerFile     *imageSourceDockerFileModel     `tfsdk:"docker_file"`
	CycleSource    *imageSourceCycleSourceModel    `tfsdk:"cycle_source"`
}

type imageSourceDockerHubModel struct {
	Target   types.String `tfsdk:"target"`
	Username types.String `tfsdk:"username"`
	Token    types.String `tfsdk:"token"`
}

type imageSourceDockerRegistryModel struct {
	Target   types.String `tfsdk:"target"`
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Token    types.String `tfsdk:"token"`
}

type imageSourceOciRegistryModel struct {
	Target   types.String `tfsdk:"target"`
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
	Token    types.String `tfsdk:"token"`
}

type imageSourceDockerFileModel struct {
	RepoURL    types.String `tfsdk:"repo_url"`
	Branch     types.String `tfsdk:"branch"`
	BuildFile  types.String `tfsdk:"build_file"`
	ContextDir types.String `tfsdk:"context_dir"`
	TargzURL   types.String `tfsdk:"targz_url"`
}

type imageSourceCycleSourceModel struct {
	SourceID types.String `tfsdk:"source_id"`
}

func (r *imageSourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image_source"
}

func (r *imageSourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle image source — the set of instructions telling Cycle where to find " +
			"and how to build container images. Exactly one origin (e.g. `docker_hub`, `docker_registry`, " +
			"`oci_registry`, `docker_file`, or `cycle_source`) must be set under `origin`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the image source.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "A human-readable slug identifier for this image source. Generated from the " +
					"name by the platform when not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A name for the image source.",
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The type of images in this source. One of `direct`, `stack-build`, or `bucket`. " +
					"Changing this forces a new image source to be created.",
				Validators: []validator.String{
					stringvalidator.OneOf("direct", "stack-build", "bucket"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A description of the image source (maps to the API's `about.description`).",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the image source (e.g. `live`).",
			},
			"origin": schema.SingleNestedAttribute{
				Required: true,
				MarkdownDescription: "Where the image comes from. Exactly one of the nested origins must be set, " +
					"mirroring the Cycle API's image origin union.",
				Attributes: map[string]schema.Attribute{
					"docker_hub": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Pull the image from Docker Hub.",
						Attributes: map[string]schema.Attribute{
							"target": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The Docker Hub target string, e.g. `mysql:5.7`.",
							},
							"username": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "A username for authenticating with Docker Hub.",
							},
							"token": schema.StringAttribute{
								Optional:            true,
								Sensitive:           true,
								MarkdownDescription: "A token for authenticating with Docker Hub.",
							},
						},
					},
					"docker_registry": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Pull the image from a private Docker registry.",
						Attributes: map[string]schema.Attribute{
							"target": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The image name on the registry, e.g. `myorg/myimage:latest`.",
							},
							"url": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The URL of the remote registry.",
							},
							"username": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "A username for authenticating with the registry.",
							},
							"password": schema.StringAttribute{
								Optional:            true,
								Sensitive:           true,
								MarkdownDescription: "A password for authenticating with the registry.",
							},
							"token": schema.StringAttribute{
								Optional:            true,
								Sensitive:           true,
								MarkdownDescription: "A token for authenticating with the registry.",
							},
						},
					},
					"oci_registry": schema.SingleNestedAttribute{
						Optional: true,
						MarkdownDescription: "Pull the image from an OCI-compatible registry (also used for " +
							"provider-native registries such as AWS ECR).",
						Attributes: map[string]schema.Attribute{
							"target": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The image name on the registry.",
							},
							"url": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The URL of the remote registry.",
							},
							"username": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "A username for user/token authentication with the registry.",
							},
							"token": schema.StringAttribute{
								Optional:            true,
								Sensitive:           true,
								MarkdownDescription: "A token for user/token authentication with the registry.",
							},
						},
					},
					"docker_file": schema.SingleNestedAttribute{
						Optional: true,
						MarkdownDescription: "Build the image from a Dockerfile located in a git repository or a " +
							"tar.gz archive. One of `repo_url` or `targz_url` should be set.",
						Attributes: map[string]schema.Attribute{
							"repo_url": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The URL of the git repository containing the Dockerfile.",
							},
							"branch": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The git branch to build from. Defaults to `master` on the platform.",
							},
							"build_file": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The path to the Dockerfile used for building the image.",
							},
							"context_dir": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The path of the directory used as the build context.",
							},
							"targz_url": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "A URL serving a .tar.gz archive of the repository, used instead of a git URL.",
							},
						},
					},
					"cycle_source": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Derive images from another existing Cycle image source.",
						Attributes: map[string]schema.Attribute{
							"source_id": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The ID of the image source images originate from.",
							},
						},
					},
				},
			},
		},
	}
}

func (r *imageSourceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

// ValidateConfig enforces that exactly one origin variant is configured.
func (r *imageSourceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var originObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("origin"), &originObj)...)
	if resp.Diagnostics.HasError() || originObj.IsNull() || originObj.IsUnknown() {
		return
	}

	variantNames := []string{"docker_hub", "docker_registry", "oci_registry", "docker_file", "cycle_source"}
	if !exactlyOneAttrSet(originObj, variantNames) {
		resp.Diagnostics.AddAttributeError(
			path.Root("origin"),
			"Invalid Image Source Origin",
			fmt.Sprintf("Exactly one origin must be set under `origin`: one of %s.", strings.Join(variantNames, ", ")),
		)
	}
}

func (r *imageSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan imageSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origin, err := expandImageSourceOrigin(plan.Origin)
	if err != nil {
		resp.Diagnostics.AddError("Error building image source origin", err.Error())
		return
	}

	name := plan.Name.ValueString()
	body := cycle.CreateImageSourceJSONRequestBody{
		Name:   &name,
		Type:   cycle.ImageSourceType(plan.Type.ValueString()),
		Origin: origin,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	if !plan.Description.IsNull() {
		description := plan.Description.ValueString()
		body.About = &struct {
			Description *string `json:"description"`
		}{Description: &description}
	}

	apiResp, err := r.client.Client.CreateImageSourceWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating image source", err.Error())
		return
	}
	if apiResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating image source", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(flattenImageSource(apiResp.JSON201.Data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *imageSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state imageSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.GetImageSourceWithResponse(ctx, state.ID.ValueString(), &cycle.GetImageSourceParams{})
	if err != nil {
		resp.Diagnostics.AddError("Error reading image source", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading image source", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	source := apiResp.JSON200.Data
	if source.State.Current == cycle.ImageSourceStateCurrentDeleted || source.State.Current == cycle.ImageSourceStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(flattenImageSource(source, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *imageSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan imageSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origin, err := expandImageSourceOrigin(plan.Origin)
	if err != nil {
		resp.Diagnostics.AddError("Error building image source origin", err.Error())
		return
	}

	name := plan.Name.ValueString()
	body := cycle.UpdateImageSourceJSONRequestBody{
		Name:   &name,
		Origin: &origin,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	about := &struct {
		Description *string `json:"description"`
	}{}
	if !plan.Description.IsNull() {
		description := plan.Description.ValueString()
		about.Description = &description
	}
	body.About = about

	apiResp, err := r.client.Client.UpdateImageSourceWithResponse(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating image source", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating image source", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(flattenImageSource(apiResp.JSON200.Data, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *imageSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state imageSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Client.DeleteImageSourceWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting image source", err.Error())
		return
	}
	if apiResp.StatusCode() == http.StatusNotFound {
		return
	}
	if apiResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting image source", apiResp.StatusCode(), apiResp.JSONDefault)
		return
	}

	// Image source deletion is asynchronous (images built from the source are
	// cleaned up too); wait so downstream destroys don't race it.
	if job := apiResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for image source deletion", err.Error())
		}
	}
}

func (r *imageSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// expandImageSourceOrigin converts the Terraform origin model into the API's
// ImageOrigin union. The From* helpers set the type discriminator.
func expandImageSourceOrigin(m *imageSourceOriginModel) (cycle.ImageOrigin, error) {
	var origin cycle.ImageOrigin
	if m == nil {
		return origin, fmt.Errorf("origin is required")
	}

	switch {
	case m.DockerHub != nil:
		var o cycle.DockerHubOrigin
		o.Details.Target = m.DockerHub.Target.ValueString()
		o.Details.Username = stringPointerIfSet(m.DockerHub.Username)
		o.Details.Token = stringPointerIfSet(m.DockerHub.Token)
		return origin, origin.FromDockerHubOrigin(o)
	case m.DockerRegistry != nil:
		var o cycle.DockerRegistryOrigin
		o.Details.Target = m.DockerRegistry.Target.ValueString()
		o.Details.Url = m.DockerRegistry.URL.ValueString()
		o.Details.Username = stringPointerIfSet(m.DockerRegistry.Username)
		o.Details.Password = stringPointerIfSet(m.DockerRegistry.Password)
		o.Details.Token = stringPointerIfSet(m.DockerRegistry.Token)
		return origin, origin.FromDockerRegistryOrigin(o)
	case m.OciRegistry != nil:
		var o cycle.OciRegistryOrigin
		o.Details.Target = m.OciRegistry.Target.ValueString()
		o.Details.Url = m.OciRegistry.URL.ValueString()
		if !m.OciRegistry.Username.IsNull() || !m.OciRegistry.Token.IsNull() {
			var user cycle.RegistryAuthUser
			user.Details.Username = stringPointerIfSet(m.OciRegistry.Username)
			user.Details.Token = stringPointerIfSet(m.OciRegistry.Token)
			var auth cycle.RegistryAuth
			if err := auth.FromRegistryAuthUser(user); err != nil {
				return origin, err
			}
			o.Details.Auth = &auth
		}
		return origin, origin.FromOciRegistryOrigin(o)
	case m.DockerFile != nil:
		var o cycle.DockerFileOrigin
		o.Details.BuildFile = stringPointerIfSet(m.DockerFile.BuildFile)
		o.Details.ContextDir = stringPointerIfSet(m.DockerFile.ContextDir)
		o.Details.TargzUrl = stringPointerIfSet(m.DockerFile.TargzURL)
		if !m.DockerFile.RepoURL.IsNull() {
			o.Details.Repo = &cycle.RepoType{
				Url:    m.DockerFile.RepoURL.ValueString(),
				Branch: stringPointerIfSet(m.DockerFile.Branch),
			}
		}
		return origin, origin.FromDockerFileOrigin(o)
	case m.CycleSource != nil:
		var o cycle.CycleSourceOrigin
		o.Details.SourceId = m.CycleSource.SourceID.ValueString()
		return origin, origin.FromCycleSourceOrigin(o)
	}

	return origin, fmt.Errorf("exactly one origin must be set under `origin`")
}

// flattenImageSource maps an API image source onto the Terraform model. The
// prior model's origin is consulted so that sensitive credential attributes
// are preserved when the API omits them from read responses.
func flattenImageSource(source cycle.ImageSource, m *imageSourceResourceModel) (diags diag.Diagnostics) {
	prior := m.Origin

	m.ID = types.StringValue(source.Id)
	m.Identifier = types.StringValue(source.Identifier)
	m.Name = types.StringValue(source.Name)
	m.Type = types.StringValue(string(source.Type))
	m.State = types.StringValue(string(source.State.Current))

	m.Description = types.StringNull()
	if source.About != nil && source.About.Description != nil && *source.About.Description != "" {
		m.Description = types.StringValue(*source.About.Description)
	}

	origin, err := flattenImageSourceOrigin(source.Origin, prior)
	if err != nil {
		diags.AddError("Error reading image source origin", err.Error())
		return diags
	}
	m.Origin = origin
	return diags
}

func flattenImageSourceOrigin(origin cycle.ImageOrigin, prior *imageSourceOriginModel) (*imageSourceOriginModel, error) {
	discriminator, err := origin.Discriminator()
	if err != nil {
		return nil, fmt.Errorf("determining origin type: %w", err)
	}

	m := &imageSourceOriginModel{}
	switch discriminator {
	case "docker-hub":
		o, err := origin.AsDockerHubOrigin()
		if err != nil {
			return nil, err
		}
		m.DockerHub = &imageSourceDockerHubModel{
			Target:   types.StringValue(o.Details.Target),
			Username: types.StringPointerValue(o.Details.Username),
			Token:    types.StringPointerValue(o.Details.Token),
		}
		if prior != nil && prior.DockerHub != nil && o.Details.Token == nil {
			m.DockerHub.Token = prior.DockerHub.Token
		}
	case "docker-registry":
		o, err := origin.AsDockerRegistryOrigin()
		if err != nil {
			return nil, err
		}
		m.DockerRegistry = &imageSourceDockerRegistryModel{
			Target:   types.StringValue(o.Details.Target),
			URL:      types.StringValue(o.Details.Url),
			Username: types.StringPointerValue(o.Details.Username),
			Password: types.StringPointerValue(o.Details.Password),
			Token:    types.StringPointerValue(o.Details.Token),
		}
		if prior != nil && prior.DockerRegistry != nil {
			if o.Details.Password == nil {
				m.DockerRegistry.Password = prior.DockerRegistry.Password
			}
			if o.Details.Token == nil {
				m.DockerRegistry.Token = prior.DockerRegistry.Token
			}
		}
	case "oci-registry":
		o, err := origin.AsOciRegistryOrigin()
		if err != nil {
			return nil, err
		}
		m.OciRegistry = &imageSourceOciRegistryModel{
			Target:   types.StringValue(o.Details.Target),
			URL:      types.StringValue(o.Details.Url),
			Username: types.StringNull(),
			Token:    types.StringNull(),
		}
		if o.Details.Auth != nil {
			if user, err := o.Details.Auth.AsRegistryAuthUser(); err == nil && user.Type == "user" {
				m.OciRegistry.Username = types.StringPointerValue(user.Details.Username)
				m.OciRegistry.Token = types.StringPointerValue(user.Details.Token)
			}
		}
		if prior != nil && prior.OciRegistry != nil && m.OciRegistry.Token.IsNull() {
			m.OciRegistry.Token = prior.OciRegistry.Token
		}
	case "docker-file":
		o, err := origin.AsDockerFileOrigin()
		if err != nil {
			return nil, err
		}
		m.DockerFile = &imageSourceDockerFileModel{
			BuildFile:  types.StringPointerValue(o.Details.BuildFile),
			ContextDir: types.StringPointerValue(o.Details.ContextDir),
			TargzURL:   types.StringPointerValue(o.Details.TargzUrl),
			RepoURL:    types.StringNull(),
			Branch:     types.StringNull(),
		}
		if o.Details.Repo != nil {
			m.DockerFile.RepoURL = types.StringValue(o.Details.Repo.Url)
			m.DockerFile.Branch = types.StringPointerValue(o.Details.Repo.Branch)
		}
	case "cycle-source":
		o, err := origin.AsCycleSourceOrigin()
		if err != nil {
			return nil, err
		}
		m.CycleSource = &imageSourceCycleSourceModel{
			SourceID: types.StringValue(o.Details.SourceId),
		}
	default:
		return nil, fmt.Errorf("the Cycle API returned an unsupported image origin type %q; "+
			"this origin type is not currently supported by the provider", discriminator)
	}

	return m, nil
}

// stringPointerIfSet returns a pointer to the string value, or nil when the
// value is null or unknown.
func stringPointerIfSet(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
