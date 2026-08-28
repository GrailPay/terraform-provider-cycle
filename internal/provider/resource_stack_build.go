package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	cycle "github.com/cycleplatform/api-client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() {
	RegisterResource(NewStackBuildResource)
}

var (
	_ resource.Resource                = (*stackBuildResource)(nil)
	_ resource.ResourceWithConfigure   = (*stackBuildResource)(nil)
	_ resource.ResourceWithImportState = (*stackBuildResource)(nil)
)

// NewStackBuildResource returns the cycle_stack_build resource.
func NewStackBuildResource() resource.Resource {
	return &stackBuildResource{}
}

type stackBuildResource struct {
	client *CycleClient
}

type stackBuildResourceModel struct {
	ID           types.String                 `tfsdk:"id"`
	StackID      types.String                 `tfsdk:"stack_id"`
	About        *stackBuildAboutModel        `tfsdk:"about"`
	Instructions *stackBuildInstructionsModel `tfsdk:"instructions"`
	HubID        types.String                 `tfsdk:"hub_id"`
	State        types.String                 `tfsdk:"state"`
}

type stackBuildAboutModel struct {
	Description types.String `tfsdk:"description"`
	Version     types.String `tfsdk:"version"`
}

type stackBuildInstructionsModel struct {
	Git       *stackBuildInstructionsGitModel `tfsdk:"git"`
	Variables types.Map                       `tfsdk:"variables"`
}

type stackBuildInstructionsGitModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

func (r *stackBuildResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stack_build"
}

func (r *stackBuildResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cycle stack build (`/v1/stacks/{id}/builds`). Builds are create-only: " +
			"changing any user-configured field forces a new build. After create, the provider polls until the " +
			"build reaches a terminal state (`live` or `deleted`). This resource does not trigger a deploy; " +
			"use a pipeline for deployment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique ID of the stack build.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"stack_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the stack to build. Changing this forces a new stack build to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"about": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Optional metadata describing this build. Changing this forces a new stack build to be created.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "A user-defined description for the build. Changing this forces a new stack build to be created.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"version": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "A user-defined version string for the build. Changing this forces a new stack build to be created.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},
			"instructions": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Additional instructions used when generating this stack build (git pin and/or build-time variables). Changing this forces a new stack build to be created.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"git": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Pin the build to a specific git branch, tag, or commit hash.",
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The type of git information being passed: `branch`, `tag`, or `hash`.",
								Validators: []validator.String{
									stringvalidator.OneOf("branch", "tag", "hash"),
								},
							},
							"value": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The branch name, tag name, or commit hash matching `type`.",
							},
						},
					},
					"variables": schema.MapAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						MarkdownDescription: "Custom variables applied to the stack during this build. Anywhere the stack uses `{{variable}}`, the matching key is substituted.",
					},
				},
			},
			"hub_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the hub this stack build belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The current state of the stack build (e.g. `new`, `building`, `live`).",
			},
		},
	}
}

func (r *stackBuildResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *stackBuildResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stackBuildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, d := stackBuildWriteBody(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Client.CreateStackBuildWithResponse(ctx, plan.StackID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating stack build", err.Error())
		return
	}
	if createResp.JSON201 == nil {
		addAPIError(&resp.Diagnostics, "creating stack build", createResp.StatusCode(), createResp.JSONDefault)
		return
	}

	build := createResp.JSON201.Data
	plan.ID = types.StringValue(build.Id)

	// Create returns the build immediately; generation is asynchronous and
	// does not include a job ID. Poll GET until a terminal state.
	final, err := waitForStackBuild(ctx, r.client, plan.StackID.ValueString(), build.Id)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for stack build", err.Error())
		return
	}

	resp.Diagnostics.Append(stackBuildModelFromAPI(ctx, &plan, *final)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stackBuildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stackBuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Client.GetStackBuildWithResponse(ctx, state.StackID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading stack build", err.Error())
		return
	}
	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if getResp.JSON200 == nil {
		addAPIError(&resp.Diagnostics, "reading stack build", getResp.StatusCode(), getResp.JSONDefault)
		return
	}

	build := getResp.JSON200.Data
	if build.State.Current == cycle.StackBuildStateCurrentDeleted || build.State.Current == cycle.StackBuildStateCurrentDeleting {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(stackBuildModelFromAPI(ctx, &state, build)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *stackBuildResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Cannot update stack build",
		"cycle_stack_build does not support in-place updates; all user-configured attributes force replacement.",
	)
}

func (r *stackBuildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stackBuildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.client.Client.DeleteStackBuildWithResponse(ctx, state.StackID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting stack build", err.Error())
		return
	}
	if deleteResp.StatusCode() == http.StatusNotFound {
		return
	}
	// Generated client maps a successful delete to JSON200 (JobDescriptor).
	if deleteResp.JSON200 != nil {
		if job := deleteResp.JSON200.Data.Job; job != nil {
			if err := waitForJobIgnoreMissing(ctx, r.client, job.Id); err != nil {
				resp.Diagnostics.AddError("Error waiting for stack build deletion", err.Error())
			}
		}
		return
	}
	if deleteResp.StatusCode() == http.StatusAccepted {
		return
	}
	addAPIError(&resp.Diagnostics, "deleting stack build", deleteResp.StatusCode(), deleteResp.JSONDefault)
}

func (r *stackBuildResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"stack_id/build_id\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("stack_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func stackBuildWriteBody(ctx context.Context, plan stackBuildResourceModel) (cycle.CreateStackBuildJSONRequestBody, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := cycle.CreateStackBuildJSONRequestBody{}

	if plan.About != nil {
		about := cycle.StackBuildAbout{
			Description: plan.About.Description.ValueString(),
			Version:     plan.About.Version.ValueString(),
		}
		body.About = &about
	}

	if plan.Instructions != nil {
		inst := cycle.StackBuildInstructions{}
		if plan.Instructions.Git != nil {
			inst.Git = &struct {
				Type  cycle.StackBuildInstructionsGitType `json:"type"`
				Value string                              `json:"value"`
			}{
				Type:  cycle.StackBuildInstructionsGitType(plan.Instructions.Git.Type.ValueString()),
				Value: plan.Instructions.Git.Value.ValueString(),
			}
		}
		if !plan.Instructions.Variables.IsNull() && !plan.Instructions.Variables.IsUnknown() {
			vars := map[string]string{}
			diags.Append(plan.Instructions.Variables.ElementsAs(ctx, &vars, false)...)
			if diags.HasError() {
				return body, diags
			}
			inst.Variables = &vars
		}
		body.Instructions = &inst
	}

	return body, diags
}

func stackBuildModelFromAPI(ctx context.Context, model *stackBuildResourceModel, build cycle.StackBuild) diag.Diagnostics {
	var diags diag.Diagnostics

	priorAbout := model.About
	priorInstructions := model.Instructions

	model.ID = types.StringValue(build.Id)
	model.StackID = types.StringValue(build.StackId)
	model.HubID = types.StringValue(build.HubId)
	model.State = types.StringValue(string(build.State.Current))

	if build.About.Description == "" && build.About.Version == "" && priorAbout == nil {
		model.About = nil
	} else if priorAbout != nil {
		// Keep the user-configured about block; the API also returns git_commit
		// metadata we do not manage.
		about := &stackBuildAboutModel{
			Description: priorAbout.Description,
			Version:     priorAbout.Version,
		}
		if build.About.Description != "" {
			about.Description = types.StringValue(build.About.Description)
		}
		if build.About.Version != "" {
			about.Version = types.StringValue(build.About.Version)
		}
		model.About = about
	} else {
		model.About = &stackBuildAboutModel{
			Description: types.StringValue(build.About.Description),
			Version:     types.StringValue(build.About.Version),
		}
	}

	hasGit := build.Instructions.Git != nil
	hasVars := build.Instructions.Variables != nil && len(*build.Instructions.Variables) > 0
	if !hasGit && !hasVars && priorInstructions == nil {
		model.Instructions = nil
	} else {
		inst := &stackBuildInstructionsModel{
			Variables: types.MapNull(types.StringType),
		}
		if priorInstructions != nil {
			inst.Variables = priorInstructions.Variables
		}
		if hasGit {
			inst.Git = &stackBuildInstructionsGitModel{
				Type:  types.StringValue(string(build.Instructions.Git.Type)),
				Value: types.StringValue(build.Instructions.Git.Value),
			}
		} else if priorInstructions != nil {
			inst.Git = priorInstructions.Git
		}
		if hasVars {
			vars, d := types.MapValueFrom(ctx, types.StringType, *build.Instructions.Variables)
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			inst.Variables = vars
		}
		model.Instructions = inst
	}

	return diags
}

// waitForStackBuild polls GET /v1/stacks/{stackId}/builds/{buildId} until the
// build reaches a terminal state. There is no job ID on create.
func waitForStackBuild(ctx context.Context, client *CycleClient, stackID, buildID string) (*cycle.StackBuild, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
	defer cancel()

	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		resp, err := client.Client.GetStackBuildWithResponse(ctx, stackID, buildID)
		if err != nil {
			return nil, fmt.Errorf("polling stack build %s: %w", buildID, err)
		}
		if resp.JSON200 == nil {
			return nil, apiError(fmt.Sprintf("polling stack build %s", buildID), resp.StatusCode(), resp.JSONDefault)
		}

		build := resp.JSON200.Data
		switch build.State.Current {
		case cycle.StackBuildStateCurrentLive:
			return &build, nil
		case cycle.StackBuildStateCurrentDeleted:
			msg := "no error message provided"
			if build.State.Error != nil && build.State.Error.Message != nil && *build.State.Error.Message != "" {
				msg = *build.State.Error.Message
			}
			return nil, fmt.Errorf("stack build %s finished in state %q: %s", buildID, build.State.Current, msg)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for stack build %s, last state %q: %w",
				buildID, build.State.Current, ctx.Err())
		case <-ticker.C:
		}
	}
}
