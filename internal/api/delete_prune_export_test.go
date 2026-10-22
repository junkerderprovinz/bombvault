package api

// WaitForPrunesAfterDelete returns once every prune a delete of backups
// started has finished.
func (s *Service) WaitForPrunesAfterDelete() { s.deletePrunes.Wait() }
