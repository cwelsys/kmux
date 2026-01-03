package store

// BundledLayouts contains default layout templates.
var BundledLayouts = map[string]string{
	"tall": `name: tall
description: Two panes - main left, shell right

tabs:
  - title: main
    layout: tall
    bias: 60
    panes:
      - ""
      - ""
`,
	"fat": `name: fat
description: Two panes - main top, shell bottom

tabs:
  - title: main
    layout: fat
    bias: 70
    panes:
      - ""
      - ""
`,
	"dev": `name: dev
description: Three panes - editor with shell sidebar

tabs:
  - title: dev
    layout: tall
    bias: 65
    panes:
      - nvim .
      - ""
      - ""
`,
}
