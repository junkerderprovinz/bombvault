package api

import (
	"context"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The push message behind a finding. It is English like every other BombVault
// notification, while the page builds the reader's own sentence from the same
// numbers.

const (
	// anomalyNotifyLines is how many findings one message spells out before it
	// counts the rest.
	anomalyNotifyLines = 10
	// anomalyEventMaxAge keeps a message about a single run from arriving days
	// after that run.
	anomalyEventMaxAge = 24 * 3600
)

// quietMetrics are the findings nothing is sent for: the failed backup, the
// failed dump and the failed restore check each send their own message
// already, so a second one about the same event is noise.
var quietMetrics = []string{
	metricFailureStreak, metricFlaky,
	metricDumpFailureStreak, metricDumpFlaky,
	metricDrillSubset, metricDrillDR,
}

// anomalyNotifyText renders one pass's findings as one message.
func anomalyNotifyText(views []AnomalyView) (title, body string) {
	lines := make([]string, 0, len(views))
	for _, v := range views[:min(len(views), anomalyNotifyLines)] {
		lines = append(lines, anomalyMessageLine(v))
	}
	if rest := len(views) - len(lines); rest > 0 {
		lines = append(lines, fmt.Sprintf("and %d more", rest))
	}
	if len(views) == 1 {
		return "BombVault: anomaly in the backup of " + anomalyNotifyName(views[0]), lines[0]
	}
	return fmt.Sprintf("BombVault: %d anomalies", len(views)), strings.Join(lines, "\n")
}

// anomalyMessageLine is one finding's sentence plus what the reader has to act
// on: that deleting old backups is waiting, and which backup to restore from.
func anomalyMessageLine(v AnomalyView) string {
	parts := []string{anomalySentence(v)}
	if v.RetentionHeld {
		parts = append(parts, "Deleting old backups of "+anomalyNotifyName(v)+
			" is paused until you acknowledge this or mark it as expected.")
	}
	if at := anomalyDetailInt(v, "lastGoodAt"); at > 0 {
		parts = append(parts, "Last good backup: "+time.Unix(at, 0).Format("2006-01-02 15:04")+".")
	}
	return strings.Join(parts, " ")
}

// anomalyNotifyName is what a finding is called in a message. A domain with no
// per-item name keeps the label notifyBackup gives it, and a series inside an
// item is named as that series.
func anomalyNotifyName(v AnomalyView) string {
	switch v.ScopeKind {
	case anomalyScopeDump:
		return "database dump of " + v.Name
	case anomalyScopeZFSDS:
		return v.Part + " (ZFS)"
	case anomalyScopeVolume:
		return "the backup storage"
	case anomalyScopeDomain:
		return v.Domain
	}
	if label := singletonItemName(v.Domain); label != "" {
		return label
	}
	return v.Name
}

func anomalySentence(v AnomalyView) string {
	name := anomalyNotifyName(v)
	observed, expected := int64(v.Observed), int64(v.Expected)
	switch v.Metric {
	case metricNewData:
		return fmt.Sprintf("%s added %s of new data in one backup. Usually it adds at most %s.",
			name, humanBytes(observed), humanBytes(anomalyDetailInt(v, "refBytes")))
	case metricNewDataRewrite:
		source := humanBytes(anomalyDetailInt(v, "sourceBytes"))
		if v.Details["allFilesNew"] == true {
			return fmt.Sprintf("Every file of %s looks new although an earlier backup exists, and %s of %s were stored again. "+
				"Files that were renamed and rewritten at once are typical of ransomware. Check the files before anything else.",
				name, humanBytes(observed), source)
		}
		return fmt.Sprintf("%s rewrote %s in one backup, at least half of everything it backs up (%s). "+
			"If you did not change that much, check whether its files were encrypted or replaced.",
			name, humanBytes(observed), source)
	case metricNewDataFull:
		return fmt.Sprintf("BombVault read every file of %s again because no earlier backup matched. New data stored: %s.",
			name, humanBytes(observed))
	case metricSourceBytesShrink:
		switch {
		case v.Details["collapse"] == true:
			return fmt.Sprintf("%s is almost empty: %s, usually %s. Check whether its data was deleted or a disk is not mounted.",
				name, humanBytes(observed), humanBytes(expected))
		case v.Details["drain"] == true:
			return fmt.Sprintf("%s has lost most of its data over the last weeks: %s, earlier %s.",
				name, humanBytes(observed), humanBytes(expected))
		}
		return fmt.Sprintf("%s shrank to %s, usually %s.", name, humanBytes(observed), humanBytes(expected))
	case metricSourceBytesGrowth:
		return fmt.Sprintf("%s grew to %s, usually %s.", name, humanBytes(observed), humanBytes(expected))
	case metricSourceFilesShrink:
		if v.Details["collapse"] == true || v.Details["drain"] == true {
			return fmt.Sprintf("%s holds almost no files any more. File count: %d, usually %d.", name, observed, expected)
		}
		return fmt.Sprintf("The file count of %s dropped to %d, usually %d.", name, observed, expected)
	case metricDumpBytesShrink:
		if v.Details["collapse"] == true || v.Details["drain"] == true {
			return fmt.Sprintf("The database dump of %s is almost empty: %s, usually %s. Check whether the database lost its data.",
				v.Name, humanBytes(observed), humanBytes(expected))
		}
		return fmt.Sprintf("The database dump of %s shrank to %s, usually %s.", v.Name, humanBytes(observed), humanBytes(expected))
	case metricDumpBytesGrowth:
		return fmt.Sprintf("The database dump of %s grew to %s, usually %s.", v.Name, humanBytes(observed), humanBytes(expected))
	case metricDurationSlower:
		return fmt.Sprintf("Backing up %s took %s, usually %s.", name, anomalyRuntime(v.Observed), anomalyRuntime(v.Expected))
	case metricDumpDurationSlower:
		return fmt.Sprintf("The database dump of %s took %s, usually %s.", v.Name, anomalyRuntime(v.Observed), anomalyRuntime(v.Expected))
	case metricCapacityETA:
		return fmt.Sprintf("The disk holding %s will be full in about %s at the current rate.",
			name, anomalyDays(v.Observed))
	case metricCapacityLow:
		return fmt.Sprintf("The disk holding %s has only %s left (%d%%).",
			name, humanBytes(anomalyDetailInt(v, "freeBytes")), int(math.Round(100*v.Observed)))
	}
	return fmt.Sprintf("BombVault found something unusual in the backup of %s.", name)
}

func anomalyDetailInt(v AnomalyView, key string) int64 {
	if value, ok := v.Details[key].(float64); ok {
		return int64(value)
	}
	return 0
}

func anomalyRuntime(ms float64) string {
	return (time.Duration(ms) * time.Millisecond).Round(time.Second).String()
}

func anomalyDays(days float64) string {
	if days < 1 {
		return "less than a day"
	}
	if n := int(math.Round(days)); n != 1 {
		return fmt.Sprintf("%d days", n)
	}
	return "1 day"
}

// sendNotifications pushes the open findings that have not been reported at
// their current severity yet, as one message per pass. Nothing is stamped
// while no channel is listening, so switching one on later still delivers what
// is open.
func (e *anomalyEngine) sendNotifications(ctx context.Context, settings store.Settings) error {
	rows, err := e.svc.store.PendingAnomalyNotifications()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	prefs, err := e.svc.store.ListItemPrefs()
	if err != nil {
		return err
	}
	items, err := e.items(settings)
	if err != nil {
		return err
	}

	now := e.now().Unix()
	var views []AnomalyView
	var ids []string
	severities := map[string]string{}
	for _, row := range rows {
		if !anomalyNotifiable(row, prefs[row.TargetID].NotifyMin, settings, now) {
			continue
		}
		views = append(views, anomalyViewOf(row, items, nil, nil, settings))
		ids = append(ids, row.ID)
		severities[row.ID] = row.Severity
	}
	if len(views) == 0 {
		return nil
	}

	cfg, err := e.svc.NotifyConfig()
	if err != nil {
		return err
	}
	if !cfg.Active() || !cfg.Configured() {
		return nil
	}
	title, body := anomalyNotifyText(views)
	// The findings of one pass can come from several domains, and none of them
	// is the run a health check watches.
	notify.Send(notify.WithHealthchecksSuppressed(ctx), cfg, "anomaly",
		notify.Event{Title: title, Message: body, OK: false})
	if e.svc.unraidGate(cfg.Unraid) {
		level := "warning"
		if slices.ContainsFunc(views, func(v AnomalyView) bool { return v.Severity == "critical" }) {
			level = "alert"
		}
		if uErr := e.svc.sendUnraidNotify(ctx, title, body, level); uErr != nil {
			log.Printf("notify: unraid: %v", uErr)
		}
	}
	return e.svc.store.MarkAnomaliesNotified(ids, severities, now)
}

// anomalyNotifiable decides whether one finding is pushed at all: never for a
// metric whose event already has a message of its own, only while an event's
// run is recent, and otherwise from the item's own minimum severity upward.
func anomalyNotifiable(row store.Anomaly, itemMin string, settings store.Settings, now int64) bool {
	if slices.Contains(quietMetrics, row.Metric) {
		return false
	}
	if slices.Contains(eventMetrics, row.Metric) && now-row.LastRunAt > anomalyEventMaxAge {
		return false
	}
	min := effectiveNotifyMin(itemMin, settings.AnomalyNotifyMin)
	return notifySeverityRank(row.Severity) >= notifySeverityRank(min)
}

// notifySeverityRank orders the notification levels. Off sits above every
// severity, so nothing reaches it.
func notifySeverityRank(level string) int {
	switch level {
	case "info":
		return 1
	case "warning":
		return 2
	case "off":
		return 4
	}
	return 3
}
