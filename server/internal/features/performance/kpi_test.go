package performance

import "testing"

// A member with no task in the month must not lose the work-related KPI:
// both progress and quality are full, not 0%.
func TestBuildKPIMemberNoTasksIsFull(t *testing.T) {
	settings := KPISettingsResponse{CustomerWeight: 10, ProgressWeight: 30, QualityWeight: 30, InitiativeWeight: 10, PunctualityWeight: 10, AttitudeWeight: 10}
	k := buildKPIMember("m", "a", "n", "", "month", "2026-10", nil, 0, 0, 0, kpiAttitude{}, 0, 0, settings)
	if k.Progress.KPI != 30 || k.Quality.KPI != 30 || k.Quality.Ratio != 1 {
		t.Fatalf("progress=%v quality=%v ratio=%v", k.Progress.KPI, k.Quality.KPI, k.Quality.Ratio)
	}

	zero := &kpiTaskStats{}
	k = buildKPIMember("m", "a", "n", "", "month", "2026-10", zero, 0, 0, 0, kpiAttitude{}, 0, 0, settings)
	if k.Quality.KPI != 30 {
		t.Fatalf("zero-estimation quality=%v", k.Quality.KPI)
	}

	// Real work still scores by ratio.
	k = buildKPIMember("m", "a", "n", "", "month", "2026-10", &kpiTaskStats{totalWorkerEstimation: 10, totalBugEstimation: 5}, 0, 0, 0, kpiAttitude{}, 0, 0, settings)
	if k.Quality.KPI != 15 {
		t.Fatalf("half-clean quality=%v", k.Quality.KPI)
	}
}
