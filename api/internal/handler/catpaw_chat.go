package handler

import (
	"ai-workbench-api/internal/security"
	"ai-workbench-api/internal/store"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000"
	},
}

// CatpawChat 通过 WebSocket 桥接 SSH PTY，运行 catpaw chat
func CatpawChat(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	// 读取连接参数（第一条消息）
	var params struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		SSHKey   string `json:"ssh_key"`
		CredID   string `json:"cred_id"`
	}
	if err := ws.ReadJSON(&params); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("command failed: "+err.Error()))
		return
	}
	if decision := security.ValidateRemoteHost(params.IP); !decision.Allowed {
		ws.WriteMessage(websocket.TextMessage, []byte("command blocked: "+decision.Reason))
		return
	}
	if params.CredID != "" {
		if saved, ok := store.GetCredential(params.CredID); ok {
			params.Port = saved.Port
			params.Username = saved.Username
			params.Password = saved.Password
			params.SSHKey = saved.SSHKey
		}
	}

	port := params.Port
	if port == 0 {
		port = 22
	}

	var auth []ssh.AuthMethod
	if params.SSHKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(params.SSHKey))
		if err == nil {
			auth = append(auth, ssh.PublicKeys(signer))
		}
	}
	if params.Password != "" {
		auth = append(auth, ssh.Password(params.Password))
	}

	cfg := &ssh.ClientConfig{
		User:            params.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", params.IP, port), cfg)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("SSH 连接失败: "+err.Error()))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("创建会话失败: "+err.Error()))
		return
	}
	defer session.Close()

	// 请求 PTY
	if err := session.RequestPty("xterm", 40, 200, ssh.TerminalModes{
		ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("PTY 失败: "+err.Error()))
		return
	}

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()

	session.Start("catpaw chat --configs /etc/catpaw/conf.d")

	// SSH 输出 → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				ws.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket 输入 → SSH stdin
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		stdin.Write(msg)
	}
}
