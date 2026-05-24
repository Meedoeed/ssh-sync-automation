package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
)

type BackendServer struct {
	taskRepo            repository.TaskRepository
	serverRepo          repository.ServerRepository
	workerRegistry      *WorkerRegistry
	schedulerLeaderRepo repository.SchedulerLeaderRepository
}

func NewBackendServer(
	taskRepo repository.TaskRepository,
	serverRepo repository.ServerRepository,
	schedulerLeaderRepo repository.SchedulerLeaderRepository,
) *BackendServer {
	return &BackendServer{
		taskRepo:            taskRepo,
		serverRepo:          serverRepo,
		workerRegistry:      NewWorkerRegistry(),
		schedulerLeaderRepo: schedulerLeaderRepo,
	}
}

func (s *BackendServer) RegisterWorker(ctx context.Context, req *connect.Request[gen.RegisterWorkerRequest]) (*connect.Response[gen.RegisterWorkerResponse], error) {
	logger.Get().Info().
		Str("worker_id", req.Msg.WorkerId).
		Msg("Worker registered")

	s.workerRegistry.Register(req.Msg.WorkerId)

	return connect.NewResponse(&gen.RegisterWorkerResponse{
		Success: true,
		Message: "Worker registered successfully",
	}), nil
}

func (s *BackendServer) Heartbeat(ctx context.Context, req *connect.Request[gen.HeartbeatRequest]) (*connect.Response[gen.HeartbeatResponse], error) {
	s.workerRegistry.Heartbeat(req.Msg.WorkerId)

	logger.Get().Debug().
		Str("worker_id", req.Msg.WorkerId).
		Msg("Heartbeat received")

	return connect.NewResponse(&gen.HeartbeatResponse{
		Success: true,
	}), nil
}

func (s *BackendServer) GetTask(ctx context.Context, req *connect.Request[gen.GetTaskRequest]) (*connect.Response[gen.GetTaskResponse], error) {
	s.workerRegistry.Heartbeat(req.Msg.WorkerId)

	tasks, err := s.taskRepo.ListPending(ctx, 1)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get pending tasks")
		return connect.NewResponse(&gen.GetTaskResponse{
			HasTask: false,
		}), nil
	}

	if len(tasks) == 0 {
		return connect.NewResponse(&gen.GetTaskResponse{
			HasTask: false,
		}), nil
	}

	task := tasks[0]

	now := time.Now()
	task.Status = domain.StatusProcessing
	task.StartedAt = &now
	task.WorkerID = &req.Msg.WorkerId

	if err := s.taskRepo.Update(ctx, task); err != nil {
		logger.Get().Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to reserve task")
		return connect.NewResponse(&gen.GetTaskResponse{
			HasTask: false,
		}), nil
	}

	server, err := s.serverRepo.GetByID(ctx, task.ServerID)
	if err != nil {
		logger.Get().Error().Err(err).Str("server_id", task.ServerID.String()).Msg("Failed to get server info")
	} else if server != nil {
		logger.Get().Info().
			Str("task_id", task.ID.String()).
			Str("worker_id", req.Msg.WorkerId).
			Str("file", task.FileName).
			Str("server_name", server.Name).
			Msg("Task assigned to worker")
	} else {
		logger.Get().Info().
			Str("task_id", task.ID.String()).
			Str("worker_id", req.Msg.WorkerId).
			Str("file", task.FileName).
			Msg("Task assigned to worker")
	}

	return connect.NewResponse(&gen.GetTaskResponse{
		HasTask: true,
		Task: &gen.Task{
			Id:         task.ID.String(),
			ServerId:   task.ServerID.String(),
			Direction:  string(task.Direction),
			FileName:   task.FileName,
			RemotePath: task.RemotePath,
			LocalPath:  task.LocalPath,
			FileSize:   task.FileSize,
		},
	}), nil
}

func (s *BackendServer) UpdateTaskProgress(ctx context.Context, req *connect.Request[gen.UpdateProgressRequest]) (*connect.Response[gen.UpdateProgressResponse], error) {
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Invalid task ID")
		return connect.NewResponse(&gen.UpdateProgressResponse{
			Success: false,
		}), nil
	}

	if err := s.taskRepo.UpdateProgress(ctx, taskID, req.Msg.BytesTransferred); err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Failed to update progress")
		return connect.NewResponse(&gen.UpdateProgressResponse{
			Success: false,
		}), nil
	}

	return connect.NewResponse(&gen.UpdateProgressResponse{
		Success: true,
	}), nil
}

func (s *BackendServer) CompleteTask(ctx context.Context, req *connect.Request[gen.CompleteTaskRequest]) (*connect.Response[gen.CompleteTaskResponse], error) {
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Invalid task ID")
		return connect.NewResponse(&gen.CompleteTaskResponse{
			Success: false,
		}), nil
	}

	now := time.Now()
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Failed to get task")
		return connect.NewResponse(&gen.CompleteTaskResponse{
			Success: false,
		}), nil
	}

	if task == nil {
		return connect.NewResponse(&gen.CompleteTaskResponse{
			Success: false,
		}), nil
	}

	task.Status = domain.StatusCompleted
	task.CompletedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Failed to complete task")
		return connect.NewResponse(&gen.CompleteTaskResponse{
			Success: false,
		}), nil
	}

	logger.Get().Info().
		Str("task_id", req.Msg.TaskId).
		Str("worker_id", req.Msg.WorkerId).
		Msg("Task completed successfully")

	return connect.NewResponse(&gen.CompleteTaskResponse{
		Success: true,
	}), nil
}

func (s *BackendServer) FailTask(ctx context.Context, req *connect.Request[gen.FailTaskRequest]) (*connect.Response[gen.FailTaskResponse], error) {
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Invalid task ID")
		return connect.NewResponse(&gen.FailTaskResponse{
			Success: false,
		}), nil
	}

	now := time.Now()
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Failed to get task")
		return connect.NewResponse(&gen.FailTaskResponse{
			Success: false,
		}), nil
	}

	if task == nil {
		return connect.NewResponse(&gen.FailTaskResponse{
			Success: false,
		}), nil
	}

	task.Status = domain.StatusFailed
	task.CompletedAt = &now
	errMsg := req.Msg.ErrorMessage
	task.ErrorMessage = &errMsg

	if err := s.taskRepo.Update(ctx, task); err != nil {
		logger.Get().Error().Err(err).Str("task_id", req.Msg.TaskId).Msg("Failed to fail task")
		return connect.NewResponse(&gen.FailTaskResponse{
			Success: false,
		}), nil
	}

	logger.Get().Warn().
		Str("task_id", req.Msg.TaskId).
		Str("worker_id", req.Msg.WorkerId).
		Str("error", req.Msg.ErrorMessage).
		Msg("Task failed")

	return connect.NewResponse(&gen.FailTaskResponse{
		Success: true,
	}), nil
}

func (s *BackendServer) CreateTask(ctx context.Context, req *connect.Request[gen.CreateTaskRequest]) (*connect.Response[gen.CreateTaskResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		return connect.NewResponse(&gen.CreateTaskResponse{
			Success: false,
			Error:   "invalid server id",
		}), nil
	}

	task := &domain.SyncTask{
		ID:          uuid.New(),
		ServerID:    serverID,
		Direction:   domain.SyncDirection(req.Msg.Direction),
		FileName:    req.Msg.FileName,
		RemotePath:  req.Msg.RemotePath,
		LocalPath:   req.Msg.LocalPath,
		FileSize:    req.Msg.FileSize,
		Status:      domain.StatusPending,
		MaxAttempts: 5,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to create task")
		return connect.NewResponse(&gen.CreateTaskResponse{
			Success: false,
			Error:   err.Error(),
		}), nil
	}

	logger.Get().Info().
		Str("task_id", task.ID.String()).
		Str("server_id", serverID.String()).
		Str("file", task.FileName).
		Msg("Task created via RPC")

	return connect.NewResponse(&gen.CreateTaskResponse{
		Success: true,
		TaskId:  task.ID.String(),
	}), nil
}

func (s *BackendServer) GetServer(ctx context.Context, req *connect.Request[gen.GetServerRequest]) (*connect.Response[gen.GetServerResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		logger.Get().Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Invalid server ID")
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		logger.Get().Error().Err(err).Str("server_id", req.Msg.ServerId).Msg("Failed to get server")
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	if server == nil {
		return connect.NewResponse(&gen.GetServerResponse{
			Server: nil,
		}), nil
	}

	var password string
	if server.Password != nil {
		password = *server.Password
	}
	var privateKey string
	if server.PrivateKey != nil {
		privateKey = *server.PrivateKey
	}

	return connect.NewResponse(&gen.GetServerResponse{
		Server: &gen.Server{
			Id:         server.ID.String(),
			Name:       server.Name,
			Host:       server.Host,
			Port:       int32(server.Port),
			Username:   server.Username,
			AuthType:   server.AuthType,
			Password:   password,
			PrivateKey: privateKey,
			IsActive:   server.IsActive,
		},
	}), nil
}

func (s *BackendServer) GetServers(ctx context.Context, req *connect.Request[gen.GetServersRequest]) (*connect.Response[gen.GetServersResponse], error) {
	servers, err := s.serverRepo.List(ctx, req.Msg.ActiveOnly)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to list servers")
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := make([]*gen.Server, 0, len(servers))
	for _, server := range servers {
		var password string
		if server.Password != nil {
			password = *server.Password
		}
		var privateKey string
		if server.PrivateKey != nil {
			privateKey = *server.PrivateKey
		}

		result = append(result, &gen.Server{
			Id:         server.ID.String(),
			Name:       server.Name,
			Host:       server.Host,
			Port:       int32(server.Port),
			Username:   server.Username,
			AuthType:   server.AuthType,
			Password:   password,
			PrivateKey: privateKey,
			IsActive:   server.IsActive,
		})
	}

	return connect.NewResponse(&gen.GetServersResponse{
		Servers: result,
	}), nil
}

func (s *BackendServer) StartHeartbeatMonitor(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Get().Debug().Msg("Heartbeat monitor stopped")
				return
			case <-ticker.C:
				s.reassignDeadWorkersTasks(ctx)
			}
		}
	}()
}

func (s *BackendServer) reassignDeadWorkersTasks(ctx context.Context) {
	deadWorkers := s.workerRegistry.GetDeadWorkers(30 * time.Second)
	if len(deadWorkers) == 0 {
		return
	}

	for _, workerID := range deadWorkers {
		logger.Get().Warn().
			Str("worker_id", workerID).
			Msg("Worker is dead, reassigning its tasks")

		tasks, err := s.taskRepo.GetTasksByWorker(ctx, workerID)
		if err != nil {
			logger.Get().Error().
				Err(err).
				Str("worker_id", workerID).
				Msg("Failed to get tasks for dead worker")
			continue
		}

		for _, task := range tasks {
			logger.Get().Info().
				Str("task_id", task.ID.String()).
				Str("worker_id", workerID).
				Msg("Reassigning task from dead worker")

			if err := s.taskRepo.ReassignTask(ctx, task.ID); err != nil {
				logger.Get().Error().
					Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to reassign task")
			} else {
				logger.Get().Info().
					Str("task_id", task.ID.String()).
					Msg("Task reassigned to pending")
			}
		}

		s.workerRegistry.Unregister(workerID)
	}
}

func (s *BackendServer) CheckTaskExists(ctx context.Context, req *connect.Request[gen.CheckTaskExistsRequest]) (*connect.Response[gen.CheckTaskExistsResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		return connect.NewResponse(&gen.CheckTaskExistsResponse{
			Exists: false,
		}), nil
	}

	exists, err := s.taskRepo.CheckExistingTask(ctx, serverID, domain.SyncDirection(req.Msg.Direction), req.Msg.RemotePath)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to check existing task")
		return connect.NewResponse(&gen.CheckTaskExistsResponse{
			Exists: false,
		}), nil
	}

	return connect.NewResponse(&gen.CheckTaskExistsResponse{
		Exists: exists,
	}), nil
}

func (s *BackendServer) TryBecomeLeader(ctx context.Context, req *connect.Request[gen.TryBecomeLeaderRequest]) (*connect.Response[gen.TryBecomeLeaderResponse], error) {
	ttl := time.Duration(req.Msg.TtlSeconds) * time.Second

	became, err := s.schedulerLeaderRepo.TryBecomeLeader(ctx, req.Msg.SchedulerId, ttl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	leader, _ := s.schedulerLeaderRepo.GetLeader(ctx)
	currentLeader := ""
	if leader != nil {
		currentLeader = leader.LeaderID
	}

	return connect.NewResponse(&gen.TryBecomeLeaderResponse{
		IsLeader:        became,
		CurrentLeaderId: currentLeader,
	}), nil
}

func (s *BackendServer) RenewLeadership(ctx context.Context, req *connect.Request[gen.RenewLeadershipRequest]) (*connect.Response[gen.RenewLeadershipResponse], error) {
	ttl := time.Duration(req.Msg.TtlSeconds) * time.Second

	renewed, err := s.schedulerLeaderRepo.RenewLeader(ctx, req.Msg.SchedulerId, ttl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gen.RenewLeadershipResponse{
		Success: renewed,
	}), nil
}

func (s *BackendServer) ReleaseLeadership(ctx context.Context, req *connect.Request[gen.ReleaseLeadershipRequest]) (*connect.Response[gen.ReleaseLeadershipResponse], error) {
	err := s.schedulerLeaderRepo.ReleaseLeader(ctx, req.Msg.SchedulerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gen.ReleaseLeadershipResponse{
		Success: true,
	}), nil
}

func (s *BackendServer) GetLeader(ctx context.Context, req *connect.Request[gen.GetLeaderRequest]) (*connect.Response[gen.GetLeaderResponse], error) {
	leader, err := s.schedulerLeaderRepo.GetLeader(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if leader == nil {
		return connect.NewResponse(&gen.GetLeaderResponse{
			LeaderId: "",
		}), nil
	}

	return connect.NewResponse(&gen.GetLeaderResponse{
		LeaderId:      leader.LeaderID,
		LastHeartbeat: leader.LastHeartbeat.Format(time.RFC3339),
	}), nil
}

func (s *BackendServer) StartCleanupScheduler(ctx context.Context, taskService *service.TaskService, interval time.Duration, retention time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		logger.Get().Info().
			Dur("interval", interval).
			Dur("retention", retention).
			Msg("Cleanup scheduler started")

		for {
			select {
			case <-ctx.Done():
				logger.Get().Debug().Msg("Cleanup scheduler stopped")
				return
			case <-ticker.C:
				count, err := taskService.CleanupOldTasks(ctx, retention)
				if err != nil {
					logger.Get().Error().Err(err).Msg("Failed to cleanup old tasks")
				} else if count > 0 {
					logger.Get().Info().
						Int64("deleted", count).
						Dur("retention", retention).
						Msg("Old tasks cleaned up")
				}
			}
		}
	}()
}
