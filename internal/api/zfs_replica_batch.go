package api

import (
	"context"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// zfsReplicaBatchKey is the progress key of the batch Replicate all now
// starts. Done and Total count its items and Failed the ones that did not
// replicate; each run reports on its own key as well.
const zfsReplicaBatchKey = "zfs-replica-all"

// zfsReplicaRefused is an item a batch leaves out, and why.
type zfsReplicaRefused struct {
	ID    string `json:"id"`
	Code  string `json:"code,omitempty"`
	Error string `json:"error"`
}

// StartZFSReplicaBatch replicates the items one after the other in the
// background. It answers the items refused before the start; an item refused
// or failing on its turn is skipped and the batch goes on. started is false
// while another batch runs.
func (s *Service) StartZFSReplicaBatch(ctx context.Context, ids []string) (queued int, refused []zfsReplicaRefused, started bool) {
	if !s.replica.batch.CompareAndSwap(false, true) {
		return 0, nil, false
	}
	var queue []string
	for _, id := range ids {
		if err := s.zfsReplicaBatchRefusal(id); err != nil {
			refused = append(refused, zfsReplicaRefused{ID: id, Code: zfsReplicaCode(err), Error: scrubError(err)})
			continue
		}
		queue = append(queue, id)
	}
	if len(queue) == 0 {
		s.replica.batch.Store(false)
		return 0, refused, true
	}
	bctx := context.WithoutCancel(ctx)
	s.replica.work.Add(1)
	go func() {
		defer s.replica.work.Done()
		defer s.replica.batch.Store(false)
		defer s.recoverOperation("zfs replica all", nil, nil)
		s.replicateZFSBatch(bctx, queue)
	}()
	return len(queue), refused, true
}

func (s *Service) zfsReplicaBatchRefusal(id string) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return err
	}
	return s.zfsReplicaRunRefusal(d)
}

func (s *Service) replicateZFSBatch(ctx context.Context, queue []string) {
	total, failed := int64(len(queue)), 0
	s.publishZFSReplicaBatch(0, total, failed, true)
	for i, id := range queue {
		if err := s.replicateZFSBatchItem(ctx, id); err != nil {
			failed++
			log.Printf("api: zfs replica all: %s did not replicate (going on): %v", id, err)
		}
		if s.StopContext().Err() != nil {
			break
		}
		s.publishZFSReplicaBatch(int64(i+1), total, failed, true)
	}
	s.publishZFSReplicaBatch(total, total, failed, false)
}

func (s *Service) replicateZFSBatchItem(ctx context.Context, id string) error {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return err
	}
	_, err = s.replicateZFSNow(ctx, d)
	return err
}

func (s *Service) publishZFSReplicaBatch(done, total int64, failed int, active bool) {
	if s.progress == nil {
		return
	}
	s.progress.Publish(progress.Event{
		Key: zfsReplicaBatchKey, Phase: zfsReplicaPhase, Percent: float64(done) / float64(total) * 100,
		Active: active, Done: done, Total: total, Failed: failed,
	})
}
