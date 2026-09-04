package job

// StatusRow is one job's lifecycle projection: the same triple a repository's
// GetStatus returns for a single id, named so a bulk read can hand back a map
// of them. It carries no kill plan, so reading many rows never loads plan
// blobs.
//
// The projection is part of the contract, not a storage detail: FailureReason
// is meaningful only on a failed job and SegmentCount only while recording, and
// every repository blanks the other cases before returning a row.
type StatusRow struct {
	Status        Status
	FailureReason string
	SegmentCount  int
}
