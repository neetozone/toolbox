package storage

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"
)

type SFTPStorage struct {
	SSHClient     *ssh.Client
	SFTPClient    *sftp.Client
	StorageConfig StorageConfig
}

type SFTPStorageOptions struct {
	URL            string
	Username       string
	PrivateKeyPath string
	Config         StorageConfig
}

func NewSFTPStorage(options SFTPStorageOptions) (*SFTPStorage, error) {
	sshClient, err := createSSHClient(options)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		log.Errorf("Error creating sftp client: %v", err)
		_ = sshClient.Close()
		return nil, err
	}

	storage := SFTPStorage{
		SSHClient:     sshClient,
		SFTPClient:    sftpClient,
		StorageConfig: options.Config,
	}

	return &storage, nil
}

func (s *SFTPStorage) Config() StorageConfig {
	return s.StorageConfig
}

func createSSHClient(options SFTPStorageOptions) (*ssh.Client, error) {
	sshKeyPath := resolvePath(options.PrivateKeyPath)

	// #nosec
	bytes, err := ioutil.ReadFile(sshKeyPath)
	if err != nil {
		log.Errorf("Error reading file %s: %v", sshKeyPath, err)
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(bytes)
	if err != nil {
		log.Errorf("Error parsing private key: %v", err)
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: options.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// #nosec
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := dialSSH(options.URL, config, httpTimeoutsFromEnv().Header)
	if err != nil {
		log.Errorf("Error dialing ssh: %v", err)
		return nil, err
	}

	return sshClient, nil
}

// dialSSH is ssh.Dial with bounded connect and handshake times.
func dialSSH(address string, config *ssh.ClientConfig, handshakeTimeout time.Duration) (*ssh.Client, error) {
	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return nil, err
	}

	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(clientConn, channels, requests), nil
}

func resolvePath(path string) string {
	if strings.HasPrefix(path, "~") {
		return strings.Replace(path, "~", os.Getenv("HOME"), 1)
	}

	return path
}
