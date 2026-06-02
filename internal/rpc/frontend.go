package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
)

func (s *BackendServer) ListServers(ctx context.Context, req *connect.Request[gen.ListServersRequest]) (*connect.Response[gen.ListServersResponse], error) {
	servers, err := s.serverRepo.List(ctx, req.Msg.ActiveOnly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := make([]*gen.ServerProto, 0, len(servers))
	for _, server := range servers {
		result = append(result, serverToProto(server))
	}

	return connect.NewResponse(&gen.ListServersResponse{
		Servers: result,
	}), nil
}

func (s *BackendServer) GetServerById(ctx context.Context, req *connect.Request[gen.GetServerByIdRequest]) (*connect.Response[gen.GetServerByIdResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if server == nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&gen.GetServerByIdResponse{
		Server: serverToProto(server),
	}), nil
}

func (s *BackendServer) CreateServer(ctx context.Context, req *connect.Request[gen.CreateServerRequest]) (*connect.Response[gen.CreateServerResponse], error) {
	server := &domain.Server{
		Name:     req.Msg.Name,
		Host:     req.Msg.Host,
		Port:     int(req.Msg.Port),
		Username: req.Msg.Username,
		AuthType: req.Msg.AuthType,
		IsActive: req.Msg.IsActive,
	}

	if req.Msg.Password != nil {
		server.Password = req.Msg.Password
	}
	if req.Msg.PrivateKey != nil {
		server.PrivateKey = req.Msg.PrivateKey
	}

	if err := s.serverRepo.Create(ctx, server); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Server created via RPC")

	return connect.NewResponse(&gen.CreateServerResponse{
		Server: serverToProto(server),
	}), nil
}

func (s *BackendServer) UpdateServer(ctx context.Context, req *connect.Request[gen.UpdateServerRequest]) (*connect.Response[gen.UpdateServerResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if server == nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	if req.Msg.Name != nil {
		server.Name = *req.Msg.Name
	}
	if req.Msg.Host != nil {
		server.Host = *req.Msg.Host
	}
	if req.Msg.Port != nil {
		server.Port = int(*req.Msg.Port)
	}
	if req.Msg.Username != nil {
		server.Username = *req.Msg.Username
	}
	if req.Msg.AuthType != nil {
		server.AuthType = *req.Msg.AuthType
	}
	if req.Msg.Password != nil {
		server.Password = req.Msg.Password
	}
	if req.Msg.PrivateKey != nil {
		server.PrivateKey = req.Msg.PrivateKey
	}
	if req.Msg.IsActive != nil {
		server.IsActive = *req.Msg.IsActive
	}

	if err := s.serverRepo.Update(ctx, server); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Server updated via RPC")

	return connect.NewResponse(&gen.UpdateServerResponse{
		Server: serverToProto(server),
	}), nil
}

func (s *BackendServer) DeleteServer(ctx context.Context, req *connect.Request[gen.DeleteServerRequest]) (*connect.Response[gen.DeleteServerResponse], error) {
	serverID, err := uuid.Parse(req.Msg.ServerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.serverRepo.Delete(ctx, serverID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	logger.Get().Info().
		Str("server_id", serverID.String()).
		Msg("Server deleted via RPC")

	return connect.NewResponse(&gen.DeleteServerResponse{
		Success: true,
	}), nil
}

// ========== Задачи для фронта ==========

func (s *BackendServer) ListTasks(ctx context.Context, req *connect.Request[gen.ListTasksRequest]) (*connect.Response[gen.ListTasksResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var tasks []*domain.SyncTask
	var err error

	if req.Msg.ServerId != nil {
		serverID, _ := uuid.Parse(*req.Msg.ServerId)
		var status *domain.SyncStatus
		if req.Msg.Status != nil {
			s := domain.SyncStatus(*req.Msg.Status)
			status = &s
		}
		tasks, err = s.taskRepo.ListByServer(ctx, serverID, status, limit)
	} else {
		tasks, err = s.taskRepo.ListAll(ctx, limit)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	result := make([]*gen.TaskProto, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, taskToProto(task))
	}

	return connect.NewResponse(&gen.ListTasksResponse{
		Tasks: result,
	}), nil
}

func (s *BackendServer) GetTaskById(ctx context.Context, req *connect.Request[gen.GetTaskByIdRequest]) (*connect.Response[gen.GetTaskByIdResponse], error) {
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if task == nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&gen.GetTaskByIdResponse{
		Task: taskToProto(task),
	}), nil
}

func (s *BackendServer) GetWorkerStats(ctx context.Context, req *connect.Request[gen.GetWorkerStatsRequest]) (*connect.Response[gen.GetWorkerStatsResponse], error) {
	stats := s.workerRegistry.GetStats()

	workersStats := make([]*gen.WorkerStatProto, 0, len(stats.Workers))
	for _, w := range stats.Workers {
		workersStats = append(workersStats, &gen.WorkerStatProto{
			ServerId:   w.WorkerID,
			ServerName: w.WorkerID,
			State:      w.State,
			LastSync:   w.LastSync.Format(time.RFC3339),
			SyncCount:  int64(w.CurrentTasks),
			ErrorCount: 0,
			LastError:  "",
		})
	}

	return connect.NewResponse(&gen.GetWorkerStatsResponse{
		TotalWorkers: int32(stats.TotalWorkers),
		Workers:      workersStats,
	}), nil
}

func (s *BackendServer) HealthCheck(ctx context.Context, req *connect.Request[gen.HealthCheckRequest]) (*connect.Response[gen.HealthCheckResponse], error) {
	return connect.NewResponse(&gen.HealthCheckResponse{
		Status: "alive",
	}), nil
}

func serverToProto(server *domain.Server) *gen.ServerProto {
	result := &gen.ServerProto{
		Id:        server.ID.String(),
		Name:      server.Name,
		Host:      server.Host,
		Port:      int32(server.Port),
		Username:  server.Username,
		AuthType:  server.AuthType,
		IsActive:  server.IsActive,
		CreatedAt: server.CreatedAt.Format(time.RFC3339),
		UpdatedAt: server.UpdatedAt.Format(time.RFC3339),
	}

	if server.Password != nil {
		result.Password = server.Password
	}
	if server.PrivateKey != nil {
		result.PrivateKey = server.PrivateKey
	}
	if server.LastSeen != nil {
		lastSeen := server.LastSeen.Format(time.RFC3339)
		result.LastSeen = &lastSeen
	}

	return result
}

func taskToProto(task *domain.SyncTask) *gen.TaskProto {
	result := &gen.TaskProto{
		Id:               task.ID.String(),
		ServerId:         task.ServerID.String(),
		Direction:        string(task.Direction),
		FileName:         task.FileName,
		RemotePath:       task.RemotePath,
		LocalPath:        task.LocalPath,
		FileSize:         task.FileSize,
		BytesTransferred: task.BytesTransferred,
		Progress:         task.Progress(),
		Status:           string(task.Status),
		AttemptCount:     int32(task.AttemptCount),
		MaxAttempts:      int32(task.MaxAttempts),
		CreatedAt:        task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        task.UpdatedAt.Format(time.RFC3339),
	}

	if task.ErrorMessage != nil {
		result.ErrorMessage = task.ErrorMessage
	}
	if task.StartedAt != nil {
		started := task.StartedAt.Format(time.RFC3339)
		result.StartedAt = &started
	}
	if task.CompletedAt != nil {
		completed := task.CompletedAt.Format(time.RFC3339)
		result.CompletedAt = &completed
	}

	return result
}
