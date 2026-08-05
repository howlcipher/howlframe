package zir

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

type Diagnostic struct {
	Code        string     `json:"code"`
	Severity    Severity   `json:"severity"`
	Message     string     `json:"message"`
	Location    Provenance `json:"location"`
	RelatedNode NodeID     `json:"related_node,omitempty"`
}
