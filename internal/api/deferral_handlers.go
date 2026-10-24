package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/activity"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// deferralView is how a container's backup waits while its app is busy, as
// GET /api/containers shows it. The API key never leaves the server; APIKeySet
// says whether one is stored.
type deferralView struct {
	RetryMin   int    `json:"retryMin"`
	WindowFrom string `json:"windowFrom"`
	WindowTo   string `json:"windowTo"`
	MaxHours   int    `json:"maxHours"`
	AtDeadline string `json:"atDeadline"`
	Preset     string `json:"preset"`
	APIKeySet  bool   `json:"apiKeySet"`
	PausedMin  int    `json:"pausedMin"`
	FailOpen   bool   `json:"failOpen"`
	NetKiB     int    `json:"netKiB"`
	NetMin     int    `json:"netMin"`
}

// deferralPatch is the whole policy as the card saves it. APIKey replaces the
// stored key when it is not empty, and APIKeyClear removes it.
type deferralPatch struct {
	RetryMin    int    `json:"retryMin"`
	WindowFrom  string `json:"windowFrom"`
	WindowTo    string `json:"windowTo"`
	MaxHours    int    `json:"maxHours"`
	AtDeadline  string `json:"atDeadline"`
	Preset      string `json:"preset"`
	APIKey      string `json:"apiKey"`
	APIKeyClear bool   `json:"apiKeyClear"`
	PausedMin   int    `json:"pausedMin"`
	FailOpen    bool   `json:"failOpen"`
	NetKiB      int    `json:"netKiB"`
	NetMin      int    `json:"netMin"`
}

func deferralViewOf(p store.DeferPolicy) deferralView {
	return deferralView{
		RetryMin: p.RetryMin, WindowFrom: p.WindowFrom, WindowTo: p.WindowTo, MaxHours: p.MaxHours,
		AtDeadline: p.AtDeadline, Preset: p.Preset, APIKeySet: len(p.APIKeyEnc) > 0, PausedMin: p.PausedMin,
		FailOpen: p.FailOpen, NetKiB: p.NetKiB, NetMin: p.NetMin,
	}
}

// Limits of a deferral policy.
const (
	minRetryMin  = 5
	maxRetryMin  = 720
	maxDeferHrs  = 168
	maxPausedMin = 1440
	maxNetKiB    = 1 << 20
	maxNetMin    = 60
)

func validateDeferral(v deferralPatch) error {
	switch {
	case v.RetryMin < minRetryMin || v.RetryMin > maxRetryMin:
		return fmt.Errorf("the time between two tries has to be between %d and %d minutes", minRetryMin, maxRetryMin)
	case (v.WindowFrom == "") != (v.WindowTo == ""):
		return errors.New("a window for the tries needs a start and an end")
	case v.WindowFrom != "" && (!schedule.ValidWindowTime(v.WindowFrom) || !schedule.ValidWindowTime(v.WindowTo)):
		return errors.New("the window for the tries takes times such as 00:00 and 08:00")
	case v.WindowFrom != "" && v.WindowFrom == v.WindowTo:
		return errors.New("the window for the tries has to end at another time than it starts")
	case v.MaxHours < 1 || v.MaxHours > maxDeferHrs:
		return fmt.Errorf("a backup can wait between 1 and %d hours for its app", maxDeferHrs)
	case v.AtDeadline != store.AtDeadlineGiveUp && v.AtDeadline != store.AtDeadlineBackUp:
		return errors.New("at the deadline a backup either gives up or backs up anyway")
	case !activity.Preset(v.Preset).Valid():
		return fmt.Errorf("there is no activity check named %q", v.Preset)
	case v.PausedMin < 0 || v.PausedMin > maxPausedMin:
		return fmt.Errorf("a paused session can be ignored after at most %d minutes", maxPausedMin)
	case v.NetKiB < 0 || v.NetKiB > maxNetKiB:
		return errors.New("the traffic limit is out of range")
	case v.NetMin < 1 || v.NetMin > maxNetMin:
		return fmt.Errorf("the traffic is watched for 1 to %d minutes", maxNetMin)
	case len(v.APIKey) > 512 || strings.ContainsAny(v.APIKey, "\r\n\""):
		return errors.New("that is no API key")
	}
	return nil
}

// SetDeferPolicy stores how a container's backup waits while its app is busy.
// A backup that waits already gets its next try from the new policy.
func (s *Service) SetDeferPolicy(_ context.Context, name string, v deferralPatch) error {
	v.WindowFrom, v.WindowTo, v.APIKey = strings.TrimSpace(v.WindowFrom), strings.TrimSpace(v.WindowTo), strings.TrimSpace(v.APIKey)
	if err := validateDeferral(v); err != nil {
		return err
	}
	tg, err := s.store.GetTargetByContainer(name)
	if errors.Is(err, sql.ErrNoRows) {
		tg, err = s.store.UpsertTarget(store.Target{ContainerName: name})
	}
	if err != nil {
		return err
	}
	p, err := s.store.DeferPolicyOf(tg.ID)
	if err != nil {
		return err
	}
	p.RetryMin, p.WindowFrom, p.WindowTo, p.MaxHours, p.AtDeadline = v.RetryMin, v.WindowFrom, v.WindowTo, v.MaxHours, v.AtDeadline
	p.Preset, p.PausedMin, p.FailOpen, p.NetKiB, p.NetMin = v.Preset, v.PausedMin, v.FailOpen, v.NetKiB, v.NetMin
	switch {
	case v.APIKey != "":
		sealed, sErr := secret.Encrypt(s.cfg.AppKey, []byte(v.APIKey))
		if sErr != nil {
			return sErr
		}
		p.APIKeyEnc = sealed
	case v.APIKeyClear:
		p.APIKeyEnc = nil
	}
	if err := s.store.SetDeferPolicy(p); err != nil {
		return err
	}
	if d, waiting, gErr := s.store.GetDeferral(tg.ID); gErr == nil && waiting {
		next, _ := schedule.NextRetry(p, time.Unix(d.Since, 0), time.Now())
		d.NextTry = next.Unix()
		if sErr := s.store.SaveDeferral(d); sErr != nil {
			log.Printf("api: deferral: %v", sErr)
		}
		s.wakeDeferrals()
	}
	return nil
}
