package hdbread

// The record kind is a test-side view: no command reads it, the output
// writers test r.Log and r.Audit directly.

// Record kinds, as Record.Kind returns them.
const (
	KindTrend = "trend" // history:NumericTrendRecord, BooleanTrendRecord, EnumTrendRecord
	KindLog   = "log"   // history:LogRecord
	KindAudit = "audit" // history:AuditRecord
)

// Kind is KindTrend, KindLog or KindAudit.
func (r Record) Kind() string {
	switch {
	case r.Log != nil:
		return KindLog
	case r.Audit != nil:
		return KindAudit
	}
	return KindTrend
}
