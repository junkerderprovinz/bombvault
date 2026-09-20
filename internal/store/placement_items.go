package store

import "errors"

// RepoChoice is repo_chosen: whether an item's location is settled.
type RepoChoice int

const (
	RepoOpen         RepoChoice = 0 // takes the default's home at its first backup
	RepoChosen       RepoChoice = 1
	RepoChosenUnread RepoChoice = 2 // Discover left repo empty because the domain path could not be read
)

var ErrRepoChoice = errors.New("repo and repo_chosen disagree")

// checkRepoChoice keeps the rule every writer of repo follows: a repository is
// only ever stored as a choice.
func checkRepoChoice(repo string, c RepoChoice) error {
	switch {
	case c == RepoChosen:
		return nil
	case (c == RepoOpen || c == RepoChosenUnread) && repo == "":
		return nil
	}
	return ErrRepoChoice
}
