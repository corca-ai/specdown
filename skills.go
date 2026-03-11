package specdown

import _ "embed"

//go:embed specs/best-practices.spec.md
var SkillWritingGuide string

//go:embed specs/adapter-protocol.spec.md
var SkillAdapterProtocol string

//go:embed specs/syntax.spec.md
var SkillSyntax string

//go:embed specs/config.spec.md
var SkillConfig string

//go:embed specs/traceability.spec.md
var SkillTraceability string

//go:embed specs/alloy.spec.md
var SkillAlloy string
