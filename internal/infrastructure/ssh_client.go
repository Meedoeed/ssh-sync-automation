package infrastructure

import (
	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	sshClint   *ssh.Client
	sftpClient *sftp.Client
	config     *config.SyncCfg
}

type SSHClientInterface interface {
	Connect(server *domain.Server) error
	Close() error
	ListFiles(remotePath string) ([]string, error)
	DownloadFile(remotePath, localPath string) error
	UploadFile(localPath, remotePath string) error
	DeleteFile(remotePath string) error
}
