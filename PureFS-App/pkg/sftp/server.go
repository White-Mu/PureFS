// Package sftp implements an SFTP server backed by PureFS storage.
// It uses golang.org/x/crypto/ssh for the SSH transport and implements
// a subset of the SFTP protocol (v3) sufficient for common file operations.
package sftp

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
	"golang.org/x/crypto/ssh"
)

// Server is the SFTP server that accepts authenticated SSH connections and
// presents each user with a view of their own files under /users/{userID}/.
type Server struct {
	cfg       *config.Config
	store     storage.Storage
	userRepo  *repository.UserRepo
	fileRepo  *repository.FileRepo
	listener  net.Listener
	sshConfig *ssh.ServerConfig
	reqID     uint32
}

// New creates a new SFTP Server. It generates or loads the host key from the
// configured HostKeyFile path.
func New(cfg *config.Config, store storage.Storage, userRepo *repository.UserRepo, fileRepo *repository.FileRepo) (*Server, error) {
	s := &Server{cfg: cfg, store: store, userRepo: userRepo, fileRepo: fileRepo}

	hostKey, err := s.loadOrGenerateHostKey()
	if err != nil {
		return nil, fmt.Errorf("sftp: host key: %w", err)
	}

	s.sshConfig = &ssh.ServerConfig{
		PasswordCallback: s.passwordAuth,
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			// SSH public key auth: look up the user by their stored public key.
			username := conn.User()
			user, err := s.userRepo.GetByUsername(username)
			if err != nil {
				return nil, fmt.Errorf("unknown user")
			}
			if user.SSHPublicKey == "" {
				return nil, fmt.Errorf("no public key configured")
			}
			allowed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(user.SSHPublicKey))
			if err != nil {
				return nil, fmt.Errorf("invalid stored key")
			}
			if ssh.FingerprintSHA256(key) == ssh.FingerprintSHA256(allowed) {
				return &ssh.Permissions{
					Extensions: map[string]string{"user-id": fmt.Sprintf("%d", user.ID)},
				}, nil
			}
			return nil, fmt.Errorf("public key mismatch")
		},
	}
	s.sshConfig.AddHostKey(hostKey)

	return s, nil
}

// loadOrGenerateHostKey reads the host key from disk or generates a new
// ED25519 key and writes it if none exists.
func (s *Server) loadOrGenerateHostKey() (ssh.Signer, error) {
	if s.cfg.SFTP.HostKeyFile != "" {
		if data, err := os.ReadFile(s.cfg.SFTP.HostKeyFile); err == nil {
			return ssh.ParsePrivateKey(data)
		}
	}
	// Generate a new key
	_, privateKeyRaw, err := generateED25519Key()
	if err != nil {
		return nil, err
	}
	// Persist
	if s.cfg.SFTP.HostKeyFile != "" {
		dir := s.cfg.SFTP.HostKeyFile
		if idx := strings.LastIndex(dir, "/"); idx >= 0 {
			dir = dir[:idx]
		} else if idx = strings.LastIndex(dir, "\\"); idx >= 0 {
			dir = dir[:idx]
		}
		if dir != "" {
			os.MkdirAll(dir, 0700)
		}
		if writeErr := os.WriteFile(s.cfg.SFTP.HostKeyFile, privateKeyRaw, 0600); writeErr != nil {
			log.Printf("sftp: failed to persist host key: %v", writeErr)
		}
	}
	return ssh.ParsePrivateKey(privateKeyRaw)
}

// passwordAuth validates SSH password authentication against the PureFS user
// database. bcrypt password hashes are checked.
func (s *Server) passwordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	user, err := s.userRepo.GetByUsername(conn.User())
	if err != nil {
		return nil, fmt.Errorf("auth failed")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("account disabled")
	}
	// Delegate password checking to the auth package
	if checkErr := checkSSHPassword(string(password), user.PasswordHash); checkErr != nil {
		return nil, fmt.Errorf("auth failed")
	}
	return &ssh.Permissions{
		Extensions: map[string]string{"user-id": fmt.Sprintf("%d", user.ID)},
	}, nil
}

// Start begins listening for SSH connections on the configured port.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.SFTP.Port)
	if s.cfg.SFTP.Port == 0 {
		addr = "0.0.0.0:2022"
	}
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("sftp listen %s: %w", addr, err)
	}
	log.Printf("SFTP server listening on %s", addr)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener closed, server shutting down
			return nil
		}
		go s.handleConn(conn)
	}
}

// Shutdown gracefully stops the SFTP server.
func (s *Server) Shutdown() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// handleConn performs the SSH handshake and then dispatches channels (session
// / subsystem requests) to the SFTP handler.
func (s *Server) handleConn(netConn net.Conn) {
	defer netConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.sshConfig)
	if err != nil {
		log.Printf("sftp: handshake failed from %s: %v", netConn.RemoteAddr(), err)
		return
	}
	defer sshConn.Close()

	// Discard global requests
	go ssh.DiscardRequests(reqs)

	// Accept channels. We expect exactly one "session" channel for SFTP.
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "only session channels supported")
			continue
		}
		ch, requests, err := newCh.Accept()
		if err != nil {
			log.Printf("sftp: accept channel: %v", err)
			continue
		}
		go s.handleSession(ch, requests, sshConn)
	}
}

// handleSession waits for an "sftp" subsystem request on the session channel,
// then starts the SFTP request loop.
func (s *Server) handleSession(ch ssh.Channel, requests <-chan *ssh.Request, sshConn *ssh.ServerConn) {
	defer ch.Close()

	userIDStr := ""
	if p := sshConn.Permissions; p != nil {
		userIDStr = p.Extensions["user-id"]
	}

	var userID int64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || userID == 0 {
		log.Printf("sftp: missing user ID in session")
		return
	}

	for req := range requests {
		switch req.Type {
		case "subsystem":
			if string(req.Payload[4:]) == "sftp" {
				req.Reply(true, nil)
				fs := newFileSystem(s.store, s.fileRepo, userID, s.cfg)
				s.serveSFTP(ch, fs)
				return
			}
			req.Reply(false, nil)
		case "exec":
			req.Reply(false, nil)
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// --- Minimal SFTP protocol (v3) implementation ---

// SFTP packet types
const (
	SSH_FXP_INIT     = 1
	SSH_FXP_VERSION  = 2
	SSH_FXP_OPEN     = 3
	SSH_FXP_CLOSE    = 4
	SSH_FXP_READ     = 5
	SSH_FXP_WRITE    = 6
	SSH_FXP_LSTAT    = 7
	SSH_FXP_FSTAT    = 8
	SSH_FXP_SETSTAT  = 9
	SSH_FXP_FSETSTAT = 10
	SSH_FXP_OPENDIR  = 11
	SSH_FXP_READDIR  = 12
	SSH_FXP_REMOVE   = 13
	SSH_FXP_MKDIR    = 14
	SSH_FXP_RMDIR    = 15
	SSH_FXP_REALPATH = 16
	SSH_FXP_STAT     = 17
	SSH_FXP_RENAME   = 18
	SSH_FXP_READLINK = 19
	SSH_FXP_SYMLINK  = 20

	SSH_FXP_STATUS = 101
	SSH_FXP_HANDLE = 102
	SSH_FXP_DATA   = 103
	SSH_FXP_NAME   = 104
	SSH_FXP_ATTRS  = 105

	SSH_FXP_EXTENDED = 200
	SSH_FXP_EXTENDED_REPLY = 201
)

// Status codes
const (
	SSH_FX_OK              = 0
	SSH_FX_EOF             = 1
	SSH_FX_NO_SUCH_FILE    = 2
	SSH_FX_PERMISSION_DENIED = 3
	SSH_FX_FAILURE         = 4
	SSH_FX_BAD_MESSAGE     = 5
	SSH_FX_NO_CONNECTION   = 6
	SSH_FX_CONNECTION_LOST = 7
	SSH_FX_OP_UNSUPPORTED  = 8
)

// File open flags
const (
	SSH_FXF_READ   = 0x00000001
	SSH_FXF_WRITE  = 0x00000002
	SSH_FXF_APPEND = 0x00000004
	SSH_FXF_CREAT  = 0x00000008
	SSH_FXF_TRUNC  = 0x00000010
	SSH_FXF_EXCL   = 0x00000020
)

// File type attributes
const (
	SSH_FILEXFER_ATTR_SIZE        = 0x00000001
	SSH_FILEXFER_ATTR_PERMISSIONS = 0x00000004
)

// attr perm mode bits
const (
	S_IFDIR  = 0040000
	S_IFREG  = 0100000
	S_IRUSR  = 0000400
	S_IWUSR  = 0000200
	S_IXUSR  = 0000100
	S_IRGRP  = 0000040
	S_IWGRP  = 0000020
	S_IXGRP  = 0000010
	S_IROTH  = 0000004
	S_IWOTH  = 0000002
	S_IXOTH  = 0000001
)

// serveSFTP reads SFTP packets from the channel and dispatches them.
func (s *Server) serveSFTP(ch ssh.Channel, fs *fileSystem) {
	buf := make([]byte, 32768)
	for {
		// Read the 4-byte length prefix
		if _, err := io.ReadFull(ch, buf[:4]); err != nil {
			return
		}
		pktLen := binary.BigEndian.Uint32(buf[:4])
		if pktLen < 1 {
			// Minimum: 1 byte type
			continue
		}
		if int(pktLen) > len(buf)-4 {
			// Packet too large
			continue
		}
		if _, err := io.ReadFull(ch, buf[4:4+pktLen]); err != nil {
			return
		}
		// pktType is the first byte after length
		pktType := buf[4]
		pktBody := buf[5 : 4+pktLen]

		s.dispatch(ch, fs, pktType, pktBody, buf)
	}
}

func (s *Server) dispatch(ch ssh.Channel, fs *fileSystem, pktType byte, body []byte, buf []byte) {
	switch pktType {
	case SSH_FXP_INIT:
		s.handleInit(ch, body)
	case SSH_FXP_OPEN:
		s.handleOpen(ch, fs, body, buf)
	case SSH_FXP_CLOSE:
		s.handleClose(ch, fs, body)
	case SSH_FXP_READ:
		s.handleRead(ch, fs, body, buf)
	case SSH_FXP_WRITE:
		s.handleWrite(ch, fs, body)
	case SSH_FXP_FSTAT:
		s.handleFStat(ch, fs, body, buf)
	case SSH_FXP_SETSTAT:
		s.handleSetStat(ch, fs, body)
	case SSH_FXP_OPENDIR:
		s.handleOpenDir(ch, fs, body, buf)
	case SSH_FXP_READDIR:
		s.handleReadDir(ch, fs, body, buf)
	case SSH_FXP_REMOVE:
		s.handleRemove(ch, fs, body)
	case SSH_FXP_MKDIR:
		s.handleMkdir(ch, fs, body)
	case SSH_FXP_RMDIR:
		s.handleRmdir(ch, fs, body)
	case SSH_FXP_REALPATH:
		s.handleRealPath(ch, fs, body, buf)
	case SSH_FXP_STAT, SSH_FXP_LSTAT:
		s.handleStat(ch, fs, body, buf)
	case SSH_FXP_RENAME:
		s.handleRename(ch, fs, body)
	case SSH_FXP_READLINK:
		s.sendStatus(ch, 0, SSH_FX_OP_UNSUPPORTED, "readlink not supported")
	case SSH_FXP_SYMLINK:
		s.sendStatus(ch, 0, SSH_FX_OP_UNSUPPORTED, "symlink not supported")
	case SSH_FXP_EXTENDED:
		s.sendStatus(ch, 0, SSH_FX_OP_UNSUPPORTED, "extended not supported")
	default:
		s.sendStatus(ch, 0, SSH_FX_BAD_MESSAGE, "unknown packet type")
	}
}

// --- SFTP packet helpers ---

func readString(data []byte) (string, []byte, error) {
	if len(data) < 4 {
		return "", nil, fmt.Errorf("short string")
	}
	l := binary.BigEndian.Uint32(data[:4])
	if int(l) > len(data[4:]) {
		return "", nil, fmt.Errorf("string overflow")
	}
	return string(data[4 : 4+l]), data[4+l:], nil
}

func writeString(buf []byte, s string) []byte {
	l := uint32(len(s))
	buf = binary.BigEndian.AppendUint32(buf, l)
	return append(buf, []byte(s)...)
}

// --- Response helpers ---

func makePacket(buf []byte, reqID uint32, pktType byte, body []byte) []byte {
	pktLen := 1 + 4 + len(body) // type + reqID + body
	out := buf[:4]
	binary.BigEndian.PutUint32(out, uint32(pktLen))
	out = append(out, pktType)
	out = binary.BigEndian.AppendUint32(out, reqID)
	out = append(out, body...)
	return out
}

func (s *Server) sendPacket(ch ssh.Channel, buf []byte) {
	if _, err := ch.Write(buf); err != nil {
		log.Printf("sftp: write error: %v", err)
	}
}

func (s *Server) sendStatus(ch ssh.Channel, reqID uint32, code uint32, msg string) {
	body := binary.BigEndian.AppendUint32(nil, code)
	body = writeString(body, msg)
	body = binary.BigEndian.AppendUint32(body, 0) // language tag (empty)
	buf := make([]byte, 4)
	s.sendPacket(ch, makePacket(buf, reqID, SSH_FXP_STATUS, body))
}

func (s *Server) sendHandle(ch ssh.Channel, reqID uint32, handle string) {
	body := writeString(nil, handle)
	buf := make([]byte, 4)
	s.sendPacket(ch, makePacket(buf, reqID, SSH_FXP_HANDLE, body))
}

func (s *Server) sendData(ch ssh.Channel, reqID uint32, data []byte) {
	body := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	body = append(body, data...)
	buf := make([]byte, 4)
	s.sendPacket(ch, makePacket(buf, reqID, SSH_FXP_DATA, body))
}

func (s *Server) sendName(ch ssh.Channel, reqID uint32, entries []nameEntry) {
	buf := make([]byte, 4)

	body := binary.BigEndian.AppendUint32(nil, uint32(len(entries)))
	for _, e := range entries {
		body = writeString(body, e.Filename)
		body = writeString(body, e.Longname)
		body = append(body, encodeAttrs(e.Attrs)...)
	}
	s.sendPacket(ch, makePacket(buf, reqID, SSH_FXP_NAME, body))
}

func (s *Server) sendAttrs(ch ssh.Channel, reqID uint32, attrs fileAttrs) {
	buf := make([]byte, 4)
	body := encodeAttrs(attrs)
	s.sendPacket(ch, makePacket(buf, reqID, SSH_FXP_ATTRS, body))
}

// --- Handler methods ---

func (s *Server) handleInit(ch ssh.Channel, body []byte) {
	// SFTP INIT: client sends its version (4 bytes), we reply with our version.
	// body = client_version (uint32)
	if len(body) < 4 {
		return
	}
	// Send version packet: SSH_FXP_VERSION + version 3
	resp := make([]byte, 9)
	resp[0] = SSH_FXP_VERSION
	// No request ID for INIT response
	binary.BigEndian.PutUint32(resp[1:5], 3) // version 3

	// Also include extensions (empty for now)
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, uint32(len(resp)))
	out = append(out, resp...)
	s.sendPacket(ch, out)
}

func (s *Server) handleOpen(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]

	path, body, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if len(body) < 12 {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	_ = binary.BigEndian.Uint32(body[:4]) // desired-access flags
	flags := binary.BigEndian.Uint32(body[4:8])
	// attrs starts at offset 8, 4 bytes for flags

	handle := s.nextHandle()
	if err := fs.OpenFile(handle, path, flags); err != nil {
		errCode := uint32(SSH_FX_PERMISSION_DENIED)
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			errCode = SSH_FX_NO_SUCH_FILE
		}
		s.sendStatus(ch, reqID, errCode, errMsg)
		return
	}
	s.sendHandle(ch, reqID, handle)
}

func (s *Server) handleClose(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	handle, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.CloseFile(handle); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleRead(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	handle, body, err := readString(body)
	if err != nil || len(body) < 8 {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	offset := binary.BigEndian.Uint64(body[:8])
	length := binary.BigEndian.Uint32(body[8:12])

	data, eof, err := fs.ReadFile(handle, int64(offset), int(length))
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	_ = eof
	s.sendData(ch, reqID, data)
}

func (s *Server) handleWrite(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	handle, body, err := readString(body)
	if err != nil || len(body) < 8 {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	offset := binary.BigEndian.Uint64(body[:8])
	dataLen := binary.BigEndian.Uint32(body[8:12])
	if int(dataLen) > len(body[12:]) {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	data := body[12 : 12+dataLen]

	if err := fs.WriteFile(handle, int64(offset), data); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleFStat(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	handle, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	attrs, err := fs.FStat(handle)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendAttrs(ch, reqID, attrs)
}

func (s *Server) handleSetStat(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, body, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.SetStat(path, body); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleOpenDir(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	handle := s.nextHandle()
	if err := fs.OpenDir(handle, path); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_NO_SUCH_FILE, err.Error())
		return
	}
	s.sendHandle(ch, reqID, handle)
}

func (s *Server) handleReadDir(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	handle, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	entries, err := fs.ReadDir(handle)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	if len(entries) == 0 {
		s.sendStatus(ch, reqID, SSH_FX_EOF, "end of directory")
		return
	}
	s.sendName(ch, reqID, entries)
}

func (s *Server) handleRemove(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.Remove(path); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleMkdir(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.Mkdir(path); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleRmdir(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.Rmdir(path); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) handleRealPath(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	resolved := fs.RealPath(path)
	attrs := fileAttrsFromStorageInfo(nil)
	attrs.Size = 0
	attrs.Permissions = S_IFDIR | 0755
	s.sendName(ch, reqID, []nameEntry{{
		Filename: resolved,
		Longname: dirLongname(resolved, attrs),
		Attrs:    attrs,
	}})
}

func (s *Server) handleStat(ch ssh.Channel, fs *fileSystem, body []byte, buf []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	path, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	attrs, err := fs.Stat(path)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_NO_SUCH_FILE, err.Error())
		return
	}
	s.sendAttrs(ch, reqID, attrs)
}

func (s *Server) handleRename(ch ssh.Channel, fs *fileSystem, body []byte) {
	reqID := binary.BigEndian.Uint32(body[:4])
	body = body[4:]
	oldPath, body, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	newPath, _, err := readString(body)
	if err != nil {
		s.sendStatus(ch, reqID, SSH_FX_BAD_MESSAGE, "invalid packet")
		return
	}
	if err := fs.Rename(oldPath, newPath); err != nil {
		s.sendStatus(ch, reqID, SSH_FX_FAILURE, err.Error())
		return
	}
	s.sendStatus(ch, reqID, SSH_FX_OK, "")
}

func (s *Server) nextHandle() string {
	id := atomic.AddUint32(&s.reqID, 1)
	return fmt.Sprintf("h%d", id)
}
