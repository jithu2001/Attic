package scanner

import (
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/store"
)

var (
	idA = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	idB = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	idC = uuid.MustParse("33333333-3333-3333-3333-333333333333")

	baseTime = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
)

func stamp(id uuid.UUID, size int64, mtime time.Time) store.FileStamp {
	return store.FileStamp{ID: id, SizeBytes: size, MTime: mtime}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name          string
		known         map[string]store.FileStamp
		found         []OnDisk
		wantIngest    []string
		wantUnchanged []uuid.UUID
		wantRemove    []uuid.UUID
	}{
		{
			name:       "empty library ingests everything",
			known:      map[string]store.FileStamp{},
			found:      []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime}},
			wantIngest: []string{"/m/a.flac"},
		},
		{
			name:  "unchanged files are not re-read",
			known: map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			found: []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime}},
			// This is the case that matters for a 100k-track library: a rescan
			// must not re-hash files nothing has touched.
			wantUnchanged: []uuid.UUID{idA},
		},
		{
			name:       "a newer mtime re-ingests",
			known:      map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			found:      []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime.Add(time.Second)}},
			wantIngest: []string{"/m/a.flac"},
		},
		{
			name:  "an older mtime also re-ingests",
			known: map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			// A restored backup moves mtime backwards; that is still a change.
			found:      []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime.Add(-time.Hour)}},
			wantIngest: []string{"/m/a.flac"},
		},
		{
			name:  "a changed size re-ingests even at the same mtime",
			known: map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			// Re-tagging in place can preserve mtime but never the size.
			found:      []OnDisk{{Path: "/m/a.flac", SizeBytes: 11, MTime: baseTime}},
			wantIngest: []string{"/m/a.flac"},
		},
		{
			name:  "sub-microsecond mtime drift is not a change",
			known: map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			// Postgres stores microseconds; ext4 reports nanoseconds. Without
			// truncation every file would look changed on every scan.
			found:         []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime.Add(400 * time.Nanosecond)}},
			wantUnchanged: []uuid.UUID{idA},
		},
		{
			name:          "mtimes in different zones compare as instants",
			known:         map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			found:         []OnDisk{{Path: "/m/a.flac", SizeBytes: 10, MTime: baseTime.In(time.FixedZone("IST", 5*3600+1800))}},
			wantUnchanged: []uuid.UUID{idA},
		},
		{
			name:       "a vanished file is removed",
			known:      map[string]store.FileStamp{"/m/a.flac": stamp(idA, 10, baseTime)},
			found:      nil,
			wantRemove: []uuid.UUID{idA},
		},
		{
			name: "a mixed directory does all three",
			known: map[string]store.FileStamp{
				"/m/keep.flac":   stamp(idA, 10, baseTime),
				"/m/change.flac": stamp(idB, 10, baseTime),
				"/m/gone.flac":   stamp(idC, 10, baseTime),
			},
			found: []OnDisk{
				{Path: "/m/keep.flac", SizeBytes: 10, MTime: baseTime},
				{Path: "/m/change.flac", SizeBytes: 99, MTime: baseTime},
				{Path: "/m/new.flac", SizeBytes: 5, MTime: baseTime},
			},
			wantIngest:    []string{"/m/change.flac", "/m/new.flac"},
			wantUnchanged: []uuid.UUID{idA},
			wantRemove:    []uuid.UUID{idC},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := Diff(tc.known, tc.found)

			ingested := make([]string, 0, len(plan.Ingest))
			for _, f := range plan.Ingest {
				ingested = append(ingested, f.Path)
			}
			sort.Strings(ingested)
			assertStrings(t, "ingest", ingested, tc.wantIngest)

			assertIDs(t, "unchanged", plan.Unchanged, tc.wantUnchanged)
			assertIDs(t, "remove", plan.Remove, tc.wantRemove)
		})
	}
}

func TestPlanEmpty(t *testing.T) {
	if !(Plan{}).Empty() {
		t.Error("a zero Plan should be empty")
	}
	if (Plan{Unchanged: []uuid.UUID{idA}}).Empty() {
		t.Error("a plan with work in it should not be empty")
	}
}

func assertStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}
}

func assertIDs(t *testing.T, what string, got, want []uuid.UUID) {
	t.Helper()
	sortIDs(got)
	sortIDs(want)
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}
}

func sortIDs(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}
