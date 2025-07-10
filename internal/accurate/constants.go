package accurate

const MetaPrefix = "accurate.cybozu.com/"

// Labels
const (
	LabelType     = MetaPrefix + "type"
	LabelTemplate = MetaPrefix + "template"
	LabelParent   = MetaPrefix + "parent"
)

// Annotations
const (
	AnnPropagate = MetaPrefix + "propagate"
)

// Label or annotation values
const (
	NSTypeTemplate  = "template"
	NSTypeRoot      = "root"
	PropagateUpdate = "update"
)
