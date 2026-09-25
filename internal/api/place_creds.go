package api

import (
	"fmt"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// credSetEnv renders a credential set into restic's environment: by its kind
// when it has one, through cloudEnv otherwise. placeID names the rclone remote
// of a WebDAV place.
func credSetEnv(set CloudCredSet, placeID string) ([]string, error) {
	if set.Kind == "" {
		return cloudEnv(set.CloudCreds), nil
	}
	return places.Env(places.Kind(set.Kind), placeCredsOf(set), placeID)
}

func placeCredsOf(set CloudCredSet) places.Creds {
	return places.Creds{
		S3KeyID: set.S3KeyID, S3Secret: set.S3Secret, S3Region: set.S3Region, S3StorageClass: set.S3StorageClass,
		RESTUser: set.RESTUser, RESTPassword: set.RESTPassword,
		WebDAVURL: set.WebDAVURL, WebDAVVendor: set.WebDAVVendor, WebDAVUser: set.WebDAVUser, WebDAVPass: set.WebDAVPass,
		AzureAccount: set.AzureAccount, AzureKey: set.AzureKey,
	}
}

// placeCredSet is the decrypted credential set a place's rows run with: its
// own, or the shared credentials while it names none.
func (s *Service) placeCredSet(p store.Place) (CloudCredSet, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return CloudCredSet{}, fmt.Errorf("read settings: %w", err)
	}
	return s.credSetFor(settings, p.CredsRef)
}

func (s *Service) placeCreds(p store.Place) (places.Creds, error) {
	set, err := s.placeCredSet(p)
	return placeCredsOf(set), err
}

// placeEnv is the environment a place's rows run with, rendered the way
// applyTargetCreds renders it for each of them.
func (s *Service) placeEnv(p store.Place) ([]string, error) {
	set, err := s.placeCredSet(p)
	if err != nil {
		return nil, err
	}
	return credSetEnv(set, p.ID)
}
