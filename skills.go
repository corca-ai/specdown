package specdown

import _ "embed"

//go:embed selfspecs/best-practices.spec.md
var SkillWritingGuide string

//go:embed selfspecs/adapter-protocol.spec.md
var SkillAdapterProtocol string

//go:embed selfspecs/syntax.spec.md
var SkillSyntax string
