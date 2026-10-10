package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A curve is drawn from the same judgement that raises a finding: the points
// are the measurements the rules read, and the band is the level and threshold
// a finding on the newest run would carry.

func (f *engineFixture) series(t *testing.T, targetID string) []AnomalySeries {
	t.Helper()
	series, err := f.svc.AnomalyItemSeries(context.Background(), targetID)
	if err != nil {
		t.Fatal(err)
	}
	return series
}

func seriesOf(t *testing.T, all []AnomalySeries, scopeKind, part string) AnomalySeries {
	t.Helper()
	for _, s := range all {
		if s.ScopeKind == scopeKind && s.Part == part {
			return s
		}
	}
	t.Fatalf("no %s series %q in %+v", scopeKind, part, all)
	return AnomalySeries{}
}

func quantityOf(t *testing.T, s AnomalySeries, name string) AnomalyQuantity {
	t.Helper()
	for _, q := range s.Quantities {
		if q.Quantity == name {
			return q
		}
	}
	t.Fatalf("the %s series measures no %s: %+v", s.ScopeKind, name, s.Quantities)
	return AnomalyQuantity{}
}

func TestSeriesHasNoBandWhileLearning(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 4, 40*gib)

	all := f.series(t, id)
	if len(all) != 1 {
		t.Fatalf("a container without dumps has one series, got %+v", all)
	}
	item := seriesOf(t, all, anomalyScopeItem, "")
	for _, name := range []string{quantitySourceBytes, quantitySourceFiles, quantityNewData, quantityDuration} {
		q := quantityOf(t, item, name)
		if !q.Learning || q.Band != nil {
			t.Fatalf("%s after four backups = %+v, want it learning without a band", name, q)
		}
		if q.Needed != anomalyMinSamples || q.Samples >= q.Needed {
			t.Fatalf("%s learned %d of %d", name, q.Samples, q.Needed)
		}
		if len(q.Points) != 4 {
			t.Fatalf("%s has %d points, want one per backup", name, len(q.Points))
		}
	}
	sizes := quantityOf(t, item, quantitySourceBytes).Points
	if sizes[0].At >= sizes[3].At {
		t.Fatalf("points are not oldest first: %+v", sizes)
	}
	if sizes[3].Value != 40*gib || sizes[3].RunID == "" {
		t.Fatalf("newest point = %+v, want the run and its 40 GiB", sizes[3])
	}
}

func TestSeriesBandIsWhatAFindingCarries(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)

	item := seriesOf(t, f.series(t, id), anomalyScopeItem, "")
	sizes := quantityOf(t, item, quantitySourceBytes)
	if sizes.Learning || sizes.Band == nil || sizes.Band.Low == nil || sizes.Band.High == nil {
		t.Fatalf("source size after twelve backups = %+v, want a band on both sides", sizes)
	}
	if low := sizes.Band.Low; low.Metric != metricSourceBytesShrink || low.Expected != 40*gib {
		t.Fatalf("low edge = %+v", low)
	}
	if high := sizes.Band.High; high.Metric != metricSourceBytesGrowth || high.Threshold <= high.Expected {
		t.Fatalf("high edge = %+v", high)
	}
	if files := quantityOf(t, item, quantitySourceFiles); files.Band == nil || files.Band.High != nil {
		t.Fatalf("file count = %+v, want a band that is open upwards", files.Band)
	}
	for _, name := range []string{quantityNewData, quantityDuration} {
		if q := quantityOf(t, item, name); q.Band == nil || q.Band.Low != nil || q.Band.High == nil {
			t.Fatalf("%s = %+v, want a band that is open downwards", name, q.Band)
		}
	}

	f.run(t, id, "backup", f.now-60, 20*gib)
	low := quantityOf(t, seriesOf(t, f.series(t, id), anomalyScopeItem, ""), quantitySourceBytes).Band.Low
	f.pass(t)
	row := onlyRow(t, f.openRows(t))
	if row.Metric != metricSourceBytesShrink {
		t.Fatalf("the halved source raised %s", row.Metric)
	}
	if low.Expected != row.Expected || low.Threshold != row.Threshold || low.Samples != row.Samples {
		t.Fatalf("low edge = %+v, the finding on the same run = %v/%v from %d samples",
			low, row.Expected, row.Threshold, row.Samples)
	}
}

func TestSeriesBandStaysWhereAnOpenFindingStarted(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	for i := 30; i >= 19; i-- {
		f.run(t, id, "backup", f.now-int64(i)*86400, 40*gib)
	}
	f.run(t, id, "backup", f.now-18*86400, 20*gib)
	f.pass(t)
	row := onlyRow(t, f.openRows(t))

	for i := 17; i >= 1; i-- {
		f.run(t, id, "backup", f.now-int64(i)*86400, 20*gib)
	}
	f.pass(t)

	low := quantityOf(t, seriesOf(t, f.series(t, id), anomalyScopeItem, ""), quantitySourceBytes).Band.Low
	if low.Expected != row.Expected || low.Threshold != row.Threshold {
		t.Fatalf("low edge = %+v, want the level the finding opened on (%v/%v)", low, row.Expected, row.Threshold)
	}
	if low.Expected != 40*gib {
		t.Fatalf("the window absorbed the smaller source: expected = %v", low.Expected)
	}
}

func TestSeriesOfAContainerWithDumps(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "postgres")
	f.steadySeries(t, id, "backup", 12, 40*gib)
	f.steadySeries(t, id, "dbdump", 12, 300*mib)

	dumps := seriesOf(t, f.series(t, id), anomalyScopeDump, "")
	if len(dumps.Quantities) != 2 {
		t.Fatalf("a dump series is measured in size and duration, got %+v", dumps.Quantities)
	}
	size := quantityOf(t, dumps, quantitySourceBytes)
	if size.Band == nil || size.Band.Low.Metric != metricDumpBytesShrink || size.Band.High.Metric != metricDumpBytesGrowth {
		t.Fatalf("dump size band = %+v", size.Band)
	}
	if took := quantityOf(t, dumps, quantityDuration); took.Band == nil || took.Band.High.Metric != metricDumpDurationSlower {
		t.Fatalf("dump duration band = %+v", took.Band)
	}
}

func TestSeriesListsFailedRuns(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 3, 40*gib)
	failed, err := f.st.StartRun(id, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishRun(failed, "failed", "", 0, "repository is locked"); err != nil {
		t.Fatal(err)
	}
	f.backdate(t, failed, f.now-3600)

	item := seriesOf(t, f.series(t, id), anomalyScopeItem, "")
	if len(item.Failed) != 1 || item.Failed[0].RunID != failed {
		t.Fatalf("failed = %+v, want the one failed run", item.Failed)
	}
	if n := len(quantityOf(t, item, quantitySourceBytes).Points); n != 3 {
		t.Fatalf("a failed run measured nothing, yet the size has %d points", n)
	}
}

func TestSeriesOfAZFSItemIsPerDataset(t *testing.T) {
	f := newEngineFixture(t)
	item := f.zfsItem(t, "tank/a")
	f.steadyTree(t, item, 12, backedUp("tank/a", 1000*gib), backedUp("tank/a/child", 20*gib))

	all := f.series(t, item)
	if len(all) != 3 {
		t.Fatalf("want the tree and its two datasets, got %+v", all)
	}
	if tree := seriesOf(t, all, anomalyScopeItem, ""); len(tree.Quantities) != 0 {
		t.Fatalf("a tree's total hides its datasets and is not measured, got %+v", tree.Quantities)
	}
	child := seriesOf(t, all, anomalyScopeZFSDS, "tank/a/child")
	sizes := quantityOf(t, child, quantitySourceBytes)
	if len(sizes.Points) != 12 || sizes.Points[11].Value != 20*gib {
		t.Fatalf("child sizes = %+v", sizes.Points)
	}
	if sizes.Learning || sizes.Band == nil || sizes.Band.Low.Expected != 20*gib {
		t.Fatalf("child band = %+v, want it around the dataset's own 20 GiB", sizes.Band)
	}
	root := quantityOf(t, seriesOf(t, all, anomalyScopeZFSDS, "tank/a"), quantitySourceBytes)
	if root.Band == nil || root.Band.Low.Expected != 1000*gib {
		t.Fatalf("root band = %+v", root.Band)
	}
	for _, name := range []string{quantitySourceFiles, quantityNewData, quantityDuration} {
		quantityOf(t, child, name)
	}
}

// seriesPayload mirrors the JSON of GET /api/anomalies/items/{targetId}/series.
// Decoding into it with DisallowUnknownFields fails on a key the page does not
// know.
type seriesPayload struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	Code   string `json:"code"`
	Series []struct {
		ScopeKind  string `json:"scopeKind"`
		Part       string `json:"part"`
		Quantities []struct {
			Quantity string `json:"quantity"`
			Points   []struct {
				RunID string  `json:"runId"`
				At    int64   `json:"at"`
				Value float64 `json:"value"`
			} `json:"points"`
			Learning bool `json:"learning"`
			Samples  int  `json:"samples"`
			Needed   int  `json:"needed"`
			Band     *struct {
				Low  *limitPayload `json:"low"`
				High *limitPayload `json:"high"`
			} `json:"band"`
		} `json:"quantities"`
		Failed []struct {
			RunID string `json:"runId"`
			At    int64  `json:"at"`
		} `json:"failed"`
	} `json:"series"`
}

type limitPayload struct {
	Metric    string  `json:"metric"`
	Expected  float64 `json:"expected"`
	Threshold float64 `json:"threshold"`
	Samples   int     `json:"samples"`
}

func getSeries(t *testing.T, f *engineFixture, targetID string) (seriesPayload, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/anomalies/items/"+targetID+"/series", nil)
	req.SetPathValue("targetId", targetID)
	rec := httptest.NewRecorder()
	itemsHandler(f).handleAnomalyItemSeries(rec, req)

	body := rec.Body.String()
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	var payload seriesPayload
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("the series payload must decode into the shape the page reads: %v (body=%s)", err, body)
	}
	return payload, body
}

func TestSeriesPayload(t *testing.T) {
	f := newEngineFixture(t)
	id := f.container(t, "nextcloud")
	f.steadySeries(t, id, "backup", 12, 40*gib)

	payload, body := getSeries(t, f, id)
	if !payload.OK || len(payload.Series) != 1 {
		t.Fatalf("series envelope: %s", body)
	}
	item := payload.Series[0]
	if item.ScopeKind != anomalyScopeItem || len(item.Quantities) != 4 || item.Failed == nil {
		t.Fatalf("item series: %s", body)
	}
	if !strings.Contains(body, `"low":{"metric":"source_bytes_shrink"`) {
		t.Fatalf("the band's edges are keyed low and high: %s", body)
	}

	learning := f.container(t, "fresh")
	f.steadySeries(t, learning, "backup", 2, gib)
	payload, body = getSeries(t, f, learning)
	if !payload.OK || !strings.Contains(body, `"learning":true`) || !strings.Contains(body, `"band":null`) {
		t.Fatalf("a learning item says so and sends a null band: %s", body)
	}
}

func TestSeriesOfAnUnknownItem(t *testing.T) {
	f := newEngineFixture(t)

	payload, body := getSeries(t, f, "no-such-item")
	if payload.OK || payload.Code != "not-found" {
		t.Fatalf("an unknown item is refused as not found: %s", body)
	}
}
