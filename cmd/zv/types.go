package main

type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"-"`
	Body        string `json:"-"`
	// Uncataloged marks a Claude Code skill (frontmatter metadata
	// zv-catalog: "false") that the zv skill contract does not validate.
	Uncataloged bool `json:"-"`
}

type skillDetail struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

type skillIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type skillCheckResult struct {
	OK            bool         `json:"ok"`
	SkillsChecked int          `json:"skills_checked"`
	Issues        []skillIssue `json:"issues"`
}

type workflowDoc struct {
	Path     string
	Required []string
	Body     string
}

type workflowCheckResult struct {
	OK               bool `json:"ok"`
	SkillsChecked    int  `json:"skills_checked"`
	WorkflowsChecked int  `json:"workflows_checked"`
	// The legacy JSON key counts executable workflow source files, not Markdown.
	WorkflowDocsChecked int          `json:"workflow_docs_checked"`
	Issues              []skillIssue `json:"issues"`
}

type workflowInfo struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Command         string            `json:"command"`
	RunCommand      string            `json:"run_command"`
	ValidateCommand string            `json:"validate_command"`
	Arguments       workflowArguments `json:"arguments"`
	Safety          workflowSafety    `json:"safety"`
	Contract        workflowContract  `json:"contract"`
	RunArgs         []string          `json:"-"`
}

type workflowArguments struct {
	Positionals             []workflowPositionalArgument     `json:"positionals"`
	RequiredFlags           []string                         `json:"required_flags"`
	OptionalValueFlags      []string                         `json:"optional_value_flags"`
	BooleanFlags            []string                         `json:"boolean_flags"`
	ValueConstraints        []workflowValueConstraint        `json:"value_constraints"`
	ConditionalRequirements []workflowConditionalRequirement `json:"conditional_requirements"`
}

type workflowPositionalArgument struct {
	Name        string `json:"name"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
}

type workflowConditionalRequirement struct {
	Description         string   `json:"description"`
	UnlessAnyFlags      []string `json:"unless_any_flags"`
	RequiredFlags       []string `json:"required_flags"`
	RequiredPositionals []string `json:"required_positionals"`
}

type workflowValueConstraint struct {
	Flag             string   `json:"flag"`
	AllowedValues    []string `json:"allowed_values"`
	Default          string   `json:"default,omitempty"`
	DiscoveryCommand string   `json:"discovery_command,omitempty"`
}

type workflowSafety struct {
	ReadOnly       bool `json:"read_only"`
	SupportsDryRun bool `json:"supports_dry_run"`
	LongRunning    bool `json:"long_running"`
}

type workflowContract struct {
	RequiredArtifacts    []string `json:"required_artifacts"`
	ProducedArtifactKeys []string `json:"produced_artifact_keys"`
	SafetyGates          []string `json:"safety_gates"`
	DryRunBehavior       string   `json:"dry_run_behavior"`
	LiveBehavior         string   `json:"live_behavior"`
	ResumePolicy         string   `json:"resume_policy"`
}

type workflowValidationResult struct {
	OK       bool            `json:"ok"`
	Scope    string          `json:"scope"`
	Workflow string          `json:"workflow"`
	Argv     []string        `json:"argv"`
	Safety   *workflowSafety `json:"safety,omitempty"`
	Executed bool            `json:"executed"`
	Error    string          `json:"error,omitempty"`
}
