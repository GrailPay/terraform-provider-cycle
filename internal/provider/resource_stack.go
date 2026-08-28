package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewStackResource)
}

var (
	_ resource.Resource                   = (*stackResource)(nil)
	_ resource.ResourceWithConfigure      = (*stackResource)(nil)
	_ resource.ResourceWithImportState    = (*stackResource)(nil)
	_ resource.ResourceWithValidateConfig = (*stackResource)(nil)
)

// NewStackResource returns the cycle_stack resource.
func NewStackResource() resource.Resource {
	return &stackResource{}
}

type stackResource struct {
	client *CycleClient
}

type stackResourceModel struct {
	ID         types.String      `tfsdk:"id"`
	Name       types.String      `tfsdk:"name"`
	Identifier types.String      `tfsdk:"identifier"`
	Variables  types.Map         `tfsdk:"variables"`
	Source     *stackSourceModel `tfsdk:"source"`
	HubID      types.String      `tfsdk:"hub_id"`
	State      types.String      `tfsdk:"state"`
}

type stackSourceModel struct {
	GitRepo *stackGitRepoModel   `tfsdk:"git_repo"`
	Raw     jsontypes.Normalized `tfsdk:"raw"`
}

type stackGitRepoModel struct {
	URL       types.String       `tfsdk:"url"`
	Branch    types.String       `tfsdk:"branch"`
	RefType   types.String       `tfsdk:"ref_type"`
	RefValue  types.String       `tfsdk:"ref_value"`
	StackFile types.String       `tfsdk:"stack_file"`
	Auth      *stackGitAuthModel `tfsdk:"auth"`
}

type stackGitAuthModel struct {
	HTTP *stackGitHTTPAuthModel `tfsdk:"http"`
	SSH  *stackGitSSHAuthModel  `tfsdk:"ssh"`
}

type stackGitHTTPAuthModel struct {
	Username types.String `tfsdk:"username"`
	Token    types.String `tfsdk:"token"`
}

type stackGitSSHAuthModel struct {
	Username   types.String `tfsdk:"username"`
	PrivateKey types.String `tfsdk:"private_key"`
	Passphrase types.String `tfsdk:"passphrase"`
}

func (r *stackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack"
}

func (r *stackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle stack (`/v1/stacks`). Stacks describe an environment as code and " +
			"are sourced from a git repository or an inline stack spec. Exactly one of `source.git_repo` or " +
			"`source.raw` must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the stack.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A user-defined name for the stack.",
			},
			"identifier": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable slugged identifier for the stack. Automatically generated from the name if not provided. The update API does not accept identifier, so changing this forces a new stack to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"variables": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Default variable values used when building this stack. Anywhere a stack uses `{{var}}`, the matching key from this map is substituted.",
			},
			"source": stackSourceSchema(false),
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this stack belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the stack (e.g. `live`).",
			},
		},
	}
}

func (r *stackResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var sourceObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("source"), &sourceObj)...)
	if resp.Diagnostics.HasError() || sourceObj.IsNull() || sourceObj.IsUnknown() {
		return
	}

	if !exactlyOneAttrSet(sourceObj, []string{"git_repo", "raw"}) {
		resp.Diagnostics.AddAttributeError(
			path.Root("source"),
			"Invalid Stack Source",
			"Exactly one of `git_repo` or `raw` must be set under `source`.",
		)
		return
	}

	var config stackResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Source == nil || config.Source.GitRepo == nil {
		return
	}

	repo := config.Source.GitRepo
	refTypeSet := !repo.RefType.IsNull() && !repo.RefType.IsUnknown() && repo.RefType.ValueString() != ""
	refValueSet := !repo.RefValue.IsNull() && !repo.RefValue.IsUnknown() && repo.RefValue.ValueString() != ""
	if refTypeSet != refValueSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("source").AtName("git_repo"),
			"Invalid Git Repository Reference",
			"`ref_type` and `ref_value` must be set together.",
		)
	}

	if repo.Auth == nil {
		return
	}
	var authObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("source").AtName("git_repo").AtName("auth"), &authObj)...)
	if resp.Diagnostics.HasError() || authObj.IsNull() || authObj.IsUnknown() {
		return
	}
	if !exactlyOneAttrSet(authObj, []string{"http", "ssh"}) {
		resp.Diagnostics.AddAttributeError(
			path.Root("source").AtName("git_repo").AtName("auth"),
			"Invalid Stack Source Authentication",
			"Exactly one of `http` or `ssh` must be set under `source.git_repo.auth`.",
		)
	}
}

func (r *stackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *stackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, d := expandStackSource(plan.Source)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := cycle.CreateStackJSONRequestBody{
		Name:   plan.Name.ValueString(),
		Source: source,
	}
	if !plan.Identifier.IsNull() && !plan.Identifier.IsUnknown() {
		identifier := plan.Identifier.ValueString()
		body.Identifier = &identifier
	}
	vars, d := stackVariablesToAPI(ctx, plan.Variables)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Variables = vars

	createResp, err := r.client.Client.CreateStackWithResponse(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating stack", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating stack", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(stackModelFromAPI(ctx, &plan, createResp.JSON201.Data, plan.Source)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetStackWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading stack", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading stack", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	stack := getResp.JSON200.Data
	if stack.State.Current == cycle.StackStateCurrentDeleted || stack.State.Current == cycle.StackStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(stackModelFromAPI(ctx, &state, stack, state.Source)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *stackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan stackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state stackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, d := expandStackSource(plan.Source)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	body := cycle.UpdateStackJSONRequestBody{
		Name:   &name,
		Source: &source,
	}
	vars, d := stackVariablesToAPI(ctx, plan.Variables)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Variables = vars

	updateResp, err := r.client.Client.UpdateStackWithResponse(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating stack", err.Error())
		return
	}
	if updateResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "updating stack", updateResp.StatusCode(), updateResp.JSONDefault)
		return
	}

	resp.Diagnostics.Append(stackModelFromAPI(ctx, &plan, updateResp.JSON200.Data, plan.Source)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stackResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteStackWithResponse(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting stack", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	if deleteResp.JSON202 == nil {
		addAPIError(&resp.Diagnostics, "deleting stack", deleteResp.StatusCode(), deleteResp.JSONDefault)
		return
	}
	if job := deleteResp.JSON202.Data.Job; job != nil {
		if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
			resp.Diagnostics.AddError("Error waiting for stack deletion", err.Error())
		}
	}
}

func (r *stackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func stackSourceSchema(computed bool) schema.SingleNestedAttribute {
	stringAttr := func(required bool, description string) schema.StringAttribute {
		attr := schema.StringAttribute{MarkdownDescription: description}
		if computed {
			attr.Computed = true
			return attr
		}
		if required {
			attr.Required = true
		} else {
			attr.Optional = true
		}
		return attr
	}

	sensitiveString := func(required bool, description string) schema.StringAttribute {
		attr := stringAttr(required, description)
		attr.Sensitive = true
		return attr
	}

	return schema.SingleNestedAttribute{
		Required:            !computed,
		Computed:            computed,
		MarkdownDescription: "The stack source. Exactly one of `git_repo` or `raw` must be set.",
		Attributes: map[string]schema.Attribute{
			"git_repo": schema.SingleNestedAttribute{
				Optional:            !computed,
				Computed:            computed,
				MarkdownDescription: "Source the stack from a git repository containing a stack spec file (defaults to `cycle.json` at the repo root).",
				Attributes: map[string]schema.Attribute{
					"url":        stringAttr(true, "The URL of the git repository."),
					"branch":     stringAttr(false, "The branch to use. Defaults to `master` on the platform when omitted."),
					"ref_type":   stringAttr(false, "The type of git reference to pin (for example `branch`, `tag`, or `commit`). Must be set together with `ref_value`."),
					"ref_value":  stringAttr(false, "The value of the git reference identified by `ref_type`. Must be set together with `ref_type`."),
					"stack_file": stringAttr(false, "Path to the stack spec file in the repository. Defaults to `cycle.json` at the repo root."),
					"auth": schema.SingleNestedAttribute{
						Optional:            !computed,
						Computed:            computed,
						MarkdownDescription: "Authentication for a private repository. Exactly one of `http` (user/token) or `ssh` must be set.",
						Attributes: map[string]schema.Attribute{
							"http": schema.SingleNestedAttribute{
								Optional:            !computed,
								Computed:            computed,
								MarkdownDescription: "HTTP user/token credentials. The token is sent to the Cycle API as the HTTP password.",
								Attributes: map[string]schema.Attribute{
									"username": stringAttr(true, "The username used for HTTP authentication."),
									"token":    sensitiveString(true, "The token (or password) used for HTTP authentication."),
								},
							},
							"ssh": schema.SingleNestedAttribute{
								Optional:            !computed,
								Computed:            computed,
								MarkdownDescription: "SSH key credentials for the repository.",
								Attributes: map[string]schema.Attribute{
									"username":    stringAttr(true, "The username used to authenticate the SSH key."),
									"private_key": sensitiveString(true, "A PEM-encoded private key."),
									"passphrase":  sensitiveString(false, "The passphrase for the private key, if any."),
								},
							},
						},
					},
				},
			},
			"raw": schema.StringAttribute{
				Optional:            !computed,
				Computed:            computed,
				CustomType:          jsontypes.NormalizedType{},
				MarkdownDescription: "An inline Cycle stack spec as normalized JSON. Unmarshaled into a `StackSpec` (`version`, `containers`, optional `about`, and so on).",
			},
		},
	}
}

func stackVariablesToAPI(ctx context.Context, m types.Map) (*map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}
	vars := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &vars, false)...)
	if diags.HasError() {
		return nil, diags
	}
	return &vars, diags
}

func expandStackSource(src *stackSourceModel) (cycle.StackSource, diag.Diagnostics) {
	var diags diag.Diagnostics
	var source cycle.StackSource
	if src == nil {
		diags.AddError("Invalid Stack Source", "source is required.")
		return source, diags
	}

	switch {
	case src.GitRepo != nil:
		repo := cycle.StackRepoSource{Type: cycle.GitRepo}
		repo.Details.Url = src.GitRepo.URL.ValueString()
		repo.Details.Branch = stringPointerIfSet(src.GitRepo.Branch)
		repo.Details.StackFile = stringPointerIfSet(src.GitRepo.StackFile)
		if !src.GitRepo.RefType.IsNull() && !src.GitRepo.RefType.IsUnknown() && src.GitRepo.RefType.ValueString() != "" {
			repo.Details.Ref = &struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			}{
				Type:  src.GitRepo.RefType.ValueString(),
				Value: src.GitRepo.RefValue.ValueString(),
			}
		}
		if src.GitRepo.Auth != nil {
			auth, d := expandStackRepoAuth(src.GitRepo.Auth)
			diags.Append(d...)
			if diags.HasError() {
				return source, diags
			}
			repo.Details.Auth = auth
		}
		if err := source.FromStackRepoSource(repo); err != nil {
			diags.AddError("Error building stack source", err.Error())
		}
		return source, diags
	case !src.Raw.IsNull() && !src.Raw.IsUnknown() && src.Raw.ValueString() != "" && src.Raw.ValueString() != "null":
		var spec cycle.StackSpec
		if err := json.Unmarshal([]byte(src.Raw.ValueString()), &spec); err != nil {
			diags.AddError("Error parsing stack spec", fmt.Sprintf("source.raw must be a JSON object matching a Cycle stack spec: %s", err))
			return source, diags
		}
		if err := source.FromStackRawSource(cycle.StackRawSource{
			Type:    cycle.StackRawSourceTypeRaw,
			Details: &spec,
		}); err != nil {
			diags.AddError("Error building stack source", err.Error())
		}
		return source, diags
	default:
		diags.AddError("Invalid Stack Source", "Exactly one of `git_repo` or `raw` must be set under `source`.")
		return source, diags
	}
}

func expandStackRepoAuth(auth *stackGitAuthModel) (*cycle.StackRepoSource_Details_Auth, diag.Diagnostics) {
	var diags diag.Diagnostics
	var out cycle.StackRepoSource_Details_Auth

	switch {
	case auth.HTTP != nil:
		creds := cycle.CredentialsHTTP{Type: cycle.CredentialsHTTPTypeHttp}
		creds.Credentials.Username = auth.HTTP.Username.ValueString()
		creds.Credentials.Password = auth.HTTP.Token.ValueString()
		if err := out.FromCredentialsHTTP(creds); err != nil {
			diags.AddError("Error building stack source authentication", err.Error())
			return nil, diags
		}
	case auth.SSH != nil:
		creds := cycle.CredentialsSSH{Type: cycle.Ssh}
		creds.Credentials.Username = auth.SSH.Username.ValueString()
		creds.Credentials.PrivateKey = auth.SSH.PrivateKey.ValueString()
		if !auth.SSH.Passphrase.IsNull() && !auth.SSH.Passphrase.IsUnknown() {
			creds.Credentials.Passphrase = auth.SSH.Passphrase.ValueString()
		}
		if err := out.FromCredentialsSSH(creds); err != nil {
			diags.AddError("Error building stack source authentication", err.Error())
			return nil, diags
		}
	default:
		diags.AddError("Invalid Stack Source Authentication", "Exactly one of `http` or `ssh` must be set under `source.git_repo.auth`.")
		return nil, diags
	}

	return &out, diags
}

func stackModelFromAPI(ctx context.Context, model *stackResourceModel, stack cycle.Stack, prior *stackSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(stack.Id)
	model.Name = types.StringValue(stack.Name)
	model.Identifier = types.StringValue(stack.Identifier)
	model.HubID = types.StringValue(stack.HubId)
	model.State = types.StringValue(string(stack.State.Current))

	if stack.Variables == nil || len(*stack.Variables) == 0 {
		if model.Variables.IsNull() || model.Variables.IsUnknown() {
			model.Variables = types.MapNull(types.StringType)
		}
	} else {
		vars, d := types.MapValueFrom(ctx, types.StringType, *stack.Variables)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		model.Variables = vars
	}

	source, d := flattenStackSource(stack.Source, prior)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	model.Source = source
	return diags
}

func flattenStackSource(source cycle.StackSource, prior *stackSourceModel) (*stackSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	discriminator, err := source.Discriminator()
	if err != nil {
		diags.AddError("Error reading stack source", fmt.Sprintf("determining source type: %s", err))
		return nil, diags
	}

	out := &stackSourceModel{Raw: jsontypes.NewNormalizedNull()}
	switch discriminator {
	case "git-repo":
		repo, err := source.AsStackRepoSource()
		if err != nil {
			diags.AddError("Error reading stack source", err.Error())
			return nil, diags
		}
		git := &stackGitRepoModel{
			URL:       types.StringValue(repo.Details.Url),
			Branch:    types.StringNull(),
			RefType:   types.StringNull(),
			RefValue:  types.StringNull(),
			StackFile: types.StringNull(),
		}
		if repo.Details.Branch != nil {
			git.Branch = types.StringValue(*repo.Details.Branch)
		}
		if repo.Details.Ref != nil {
			git.RefType = types.StringValue(repo.Details.Ref.Type)
			git.RefValue = types.StringValue(repo.Details.Ref.Value)
		}
		if repo.Details.StackFile != nil {
			git.StackFile = types.StringValue(*repo.Details.StackFile)
		}
		git.Auth = flattenStackRepoAuth(repo.Details.Auth, priorGitAuth(prior))
		out.GitRepo = git
	case "raw":
		raw, err := source.AsStackRawSource()
		if err != nil {
			diags.AddError("Error reading stack source", err.Error())
			return nil, diags
		}
		if raw.Details == nil {
			out.Raw = jsontypes.NewNormalizedNull()
			break
		}
		b, err := json.Marshal(raw.Details)
		if err != nil {
			diags.AddError("Error encoding stack spec", err.Error())
			return nil, diags
		}
		out.Raw = jsontypes.NewNormalizedValue(string(b))
	default:
		diags.AddError(
			"Error reading stack source",
			fmt.Sprintf("the Cycle API returned an unsupported stack source type %q", discriminator),
		)
		return nil, diags
	}

	return out, diags
}

func priorGitAuth(prior *stackSourceModel) *stackGitAuthModel {
	if prior == nil || prior.GitRepo == nil {
		return nil
	}
	return prior.GitRepo.Auth
}

func flattenStackRepoAuth(auth *cycle.StackRepoSource_Details_Auth, prior *stackGitAuthModel) *stackGitAuthModel {
	if auth == nil {
		return prior
	}

	discriminator, err := auth.Discriminator()
	if err != nil {
		return prior
	}

	switch discriminator {
	case "http":
		creds, err := auth.AsCredentialsHTTP()
		if err != nil {
			return prior
		}
		httpAuth := &stackGitHTTPAuthModel{
			Username: types.StringValue(creds.Credentials.Username),
			Token:    types.StringValue(creds.Credentials.Password),
		}
		if creds.Credentials.Password == "" && prior != nil && prior.HTTP != nil {
			httpAuth.Token = prior.HTTP.Token
			if httpAuth.Username.ValueString() == "" {
				httpAuth.Username = prior.HTTP.Username
			}
		}
		return &stackGitAuthModel{HTTP: httpAuth}
	case "ssh":
		creds, err := auth.AsCredentialsSSH()
		if err != nil {
			return prior
		}
		sshAuth := &stackGitSSHAuthModel{
			Username:   types.StringValue(creds.Credentials.Username),
			PrivateKey: types.StringValue(creds.Credentials.PrivateKey),
			Passphrase: types.StringValue(creds.Credentials.Passphrase),
		}
		if prior != nil && prior.SSH != nil {
			if creds.Credentials.PrivateKey == "" {
				sshAuth.PrivateKey = prior.SSH.PrivateKey
			}
			if creds.Credentials.Passphrase == "" {
				sshAuth.Passphrase = prior.SSH.Passphrase
			}
			if creds.Credentials.Username == "" {
				sshAuth.Username = prior.SSH.Username
			}
		}
		return &stackGitAuthModel{SSH: sshAuth}
	default:
		return prior
	}
}
