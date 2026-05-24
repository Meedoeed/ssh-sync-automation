package leader

import (
	"context"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

type RPCElection struct {
	schedulerID string
	client      genconnect.BackendServiceClient
	ttl         time.Duration
	isLeader    bool
	mu          sync.RWMutex
	cancelFunc  context.CancelFunc
}

func NewRPCElection(schedulerID string, client genconnect.BackendServiceClient, ttl time.Duration) *RPCElection {
	return &RPCElection{
		schedulerID: schedulerID,
		client:      client,
		ttl:         ttl,
		isLeader:    false,
	}
}

func (e *RPCElection) Start(ctx context.Context) {
	ticker := time.NewTicker(e.ttl / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.release()
			return
		case <-ticker.C:
			e.tryBecomeLeader(ctx)
		}
	}
}

func (e *RPCElection) tryBecomeLeader(ctx context.Context) {
	if e.IsLeader() {
		e.renewLeadership(ctx)
		return
	}

	resp, err := e.client.TryBecomeLeader(ctx, connect.NewRequest(&gen.TryBecomeLeaderRequest{
		SchedulerId: e.schedulerID,
		TtlSeconds:  int32(e.ttl.Seconds()),
	}))

	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to try become leader")
		return
	}

	if resp.Msg.IsLeader {
		logger.Get().Info().
			Str("scheduler_id", e.schedulerID).
			Msg("Became leader")
		e.setLeader(true)
	} else {
		if resp.Msg.CurrentLeaderId != "" {
			logger.Get().Debug().
				Str("current_leader", resp.Msg.CurrentLeaderId).
				Msg("Another scheduler is leader")
		}
	}
}

func (e *RPCElection) renewLeadership(ctx context.Context) {
	renewResp, err := e.client.RenewLeadership(ctx, connect.NewRequest(&gen.RenewLeadershipRequest{
		SchedulerId: e.schedulerID,
		TtlSeconds:  int32(e.ttl.Seconds()),
	}))

	if err != nil {
		logger.Get().Warn().Err(err).Msg("Failed to renew leadership")
		e.setLeader(false)
		return
	}

	if !renewResp.Msg.Success {
		logger.Get().Warn().Msg("Leadership renewal failed, lost leadership")
		e.setLeader(false)
	}
}

func (e *RPCElection) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isLeader
}

func (e *RPCElection) setLeader(leader bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isLeader == leader {
		return
	}

	e.isLeader = leader
	if !leader {
		logger.Get().Warn().Str("scheduler_id", e.schedulerID).Msg("Lost leadership")
	}
}

func (e *RPCElection) release() {
	if !e.IsLeader() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := e.client.ReleaseLeadership(ctx, connect.NewRequest(&gen.ReleaseLeadershipRequest{
		SchedulerId: e.schedulerID,
	}))
	if err != nil {
		logger.Get().Warn().Err(err).Msg("Failed to release leadership")
	}
}
