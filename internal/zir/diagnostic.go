package zir

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// DiagnosticContractVersion versions the diagnostic wire format. This is
// independent of Graph.Version, which versions the graph schema instead —
// the two happen to both start at "v1" but track different things.
const DiagnosticContractVersion = "v1"

type Diagnostic struct {
	Code            string     `json:"code"`
	Severity        Severity   `json:"severity"`
	Message         string     `json:"message"`
	Location        Provenance `json:"location"`
	RelatedNode     NodeID     `json:"related_node,omitempty"`
	ContractVersion string     `json:"contract_version"`
	Target          string     `json:"target,omitempty"`
}
