package specdown

import _ "embed"

//go:embed specs/overview.md
var SkillOverview string

//go:embed specs/syntax.md
var SkillSyntax string

//go:embed specs/config.md
var SkillConfig string

//go:embed specs/cli.md
var SkillCLI string

//go:embed specs/adapter-protocol.md
var SkillAdapterProtocol string

//go:embed specs/alloy.md
var SkillAlloy string

//go:embed specs/report.md
var SkillReport string

//go:embed specs/internals.md
var SkillInternals string

//go:embed specs/best-practices.md
var SkillBestPractices string

//go:embed specs/validation.md
var SkillValidation string

//go:embed specs/traceability.md
var SkillTraceability string

//go:embed docs/guide-alloy-explore.md
var SkillGuideAlloyExplore string

//go:embed docs/workflow-new-project.md
var SkillWorkflowNewProject string

//go:embed docs/workflow-adopt.md
var SkillWorkflowAdopt string

//go:embed docs/workflow-evolve.md
var SkillWorkflowEvolve string
