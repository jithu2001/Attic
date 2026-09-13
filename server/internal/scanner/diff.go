// Package scanner walks the music library and keeps the database in step with
// what is actually on disk.
package scanner

import (
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/store"
)

// OnDisk is what a walk found: a file, its size and its modification time.
type OnDisk struct {
	Path      string
	SizeBytes int64
	MTime     time.Time
}

// Plan is the work one directory's diff produced.
type Plan struct {
	// Ingest are new or changed files that must be hashed, probed and tagged.
	Ingest []OnDisk

	// Unchanged are rows whose file is untouched; they only need their
	// verified_at stamp refreshed.
	Unchanged []uuid.UUID

	// Remove are rows whose file is gone from this directory.
	Remove []uuid.UUID
}

// Empty reports whether the plan would change nothing at all.
func (p Plan) Empty() bool {
	return len(p.Ingest) == 0 && len(p.Remove) == 0 && len(p.Unchanged) == 0
}

// Diff decides what to do with one directory.
//
// It is deliberately pure — no filesystem, no database — because this is the
// logic that decides whether a 100k-track library gets re-hashed on every scan
// or not, and that deserves to be exhaustively testable.
//
// A file is considered unchanged when both its size and its modification time
// match what was recorded. Timestamps are compared at microsecond resolution
// because that is all a Postgres timestamptz keeps: comparing the nanoseconds
// an ext4 stat returns would mark every file as changed on every scan.
func Diff(known map[string]store.FileStamp, found []OnDisk) Plan {
	plan := Plan{}
	seen := make(map[string]struct{}, len(found))

	for _, file := range found {
		seen[file.Path] = struct{}{}

		stamp, isKnown := known[file.Path]
		switch {
		case !isKnown:
			plan.Ingest = append(plan.Ingest, file)
		case stamp.SizeBytes != file.SizeBytes || !sameInstant(stamp.MTime, file.MTime):
			plan.Ingest = append(plan.Ingest, file)
		default:
			plan.Unchanged = append(plan.Unchanged, stamp.ID)
		}
	}

	for path, stamp := range known {
		if _, stillThere := seen[path]; !stillThere {
			plan.Remove = append(plan.Remove, stamp.ID)
		}
	}

	return plan
}

func sameInstant(a, b time.Time) bool {
	return a.UTC().Truncate(time.Microsecond).Equal(b.UTC().Truncate(time.Microsecond))
}
