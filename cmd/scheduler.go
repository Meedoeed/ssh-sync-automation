package cmd

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/leader"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var (
	schedulerBackendAddr string
	schedulerInterval    time.Duration
	schedulerID          string
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Запуск планировщика задач",
	Long:  "Планировщик создаёт задания на проверку файлов (probe tasks)",
	Run:   runScheduler,
}

func init() {
	schedulerCmd.Flags().StringVar(&schedulerBackendAddr, "backend", "http://localhost:8082", "Адрес backend RPC сервера")
	schedulerCmd.Flags().DurationVar(&schedulerInterval, "interval", 30*time.Second, "Интервал между циклами планировщика")
	schedulerCmd.Flags().StringVar(&schedulerID, "id", "", "Уникальный ID шедулера")
}

func runScheduler(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме SCHEDULER")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION in SCHEDULER mode")

	if schedulerID == "" {
		schedulerID = "scheduler-" + time.Now().Format("20060102150405") + "-" + uuid.New().String()[:8]
		log.Info().Str("generated_id", schedulerID).Msg("Auto-generated scheduler ID")
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	rpcClient := genconnect.NewBackendServiceClient(httpClient, schedulerBackendAddr)

	election := leader.NewRPCElection(schedulerID, rpcClient, 15*time.Second)
	electionCtx, electionCancel := context.WithCancel(context.Background())
	go election.Start(electionCtx)

	for !election.IsLeader() {
		log.Debug().Msg("Waiting to become leader...")
		time.Sleep(2 * time.Second)
	}

	log.Info().Msg("Became leader, starting scheduler loop")

	ctx, cancel := context.WithCancel(context.Background())
	go schedulerLoop(ctx, rpcClient)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down scheduler...")
	electionCancel()
	cancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("Scheduler stopped")
}

func schedulerLoop(ctx context.Context, client genconnect.BackendServiceClient) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Msg("Scheduler loop stopped")
			return
		case <-ticker.C:
			runSchedulerCycle(ctx, client)
		}
	}
}

func runSchedulerCycle(ctx context.Context, client genconnect.BackendServiceClient) {
	log := logger.Get()

	serversResp, err := client.GetServers(ctx, connect.NewRequest(&gen.GetServersRequest{
		ActiveOnly: true,
	}))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get servers")
		return
	}

	for _, server := range serversResp.Msg.Servers {
		_, err := client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
			ServerId:   server.Id,
			Direction:  "probe_done",
			FileName:   "",
			RemotePath: "done/",
			LocalPath:  "",
			FileSize:   0,
		}))
		if err != nil {
			log.Error().Err(err).Str("server", server.Name).Msg("Failed to create probe task for done/")
		}

		_, err = client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
			ServerId:   server.Id,
			Direction:  "probe_tasks",
			FileName:   "",
			RemotePath: "",
			LocalPath:  "./data/tasks/" + server.Name,
			FileSize:   0,
		}))
		if err != nil {
			log.Error().Err(err).Str("server", server.Name).Msg("Failed to create probe task for local tasks")
		}
	}

	log.Debug().Int("servers", len(serversResp.Msg.Servers)).Msg("Scheduler cycle completed")
}
