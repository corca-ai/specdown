package specdown

import _ "embed"

//go:embed specs/overview.spec.md
var SkillOverview string

//go:embed specs/syntax.spec.md
var SkillSyntax string

//go:embed specs/config.spec.md
var SkillConfig string

//go:embed specs/cli.spec.md
var SkillCLI string

//go:embed specs/adapter-protocol.spec.md
var SkillAdapterProtocol string

//go:embed specs/alloy.spec.md
var SkillAlloy string

//go:embed specs/report.spec.md
var SkillReport string

//go:embed specs/internals.spec.md
var SkillInternals string

//go:embed specs/best-practices.spec.md
var SkillBestPractices string

//go:embed specs/validation.spec.md
var SkillValidation string

//go:embed specs/traceability.spec.md
var SkillTraceability string

//go:embed docs/workflow-new-project.md
var SkillWorkflowNewProject string

//go:embed docs/workflow-adopt.md
var SkillWorkflowAdopt string

//go:embed docs/workflow-evolve.md
var SkillWorkflowEvolve string
