package handler

import (
	"ai-workbench-api/internal/security"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

func ListCredentials(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListCredentials())
}

func SaveCredential(c *gin.Context) {
	var cred model.Credential
	if err := c.ShouldBindJSON(&cred); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cred.ID == "" {
		cred.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	} else if existing, ok := store.GetCredential(cred.ID); ok {
		if cred.Password == "******" {
			cred.Password = existing.Password
		}
		if cred.SSHKey == "******" {
			cred.SSHKey = existing.SSHKey
		}
	}
	if cred.Protocol == "" {
		cred.Protocol = "ssh"
	}
	if cred.Port == 0 {
		if strings.EqualFold(cred.Protocol, "winrm") {
			cred.Port = 5985
		} else if strings.EqualFold(cred.Protocol, "wmi") {
			cred.Port = 135
		} else {
			cred.Port = 22
		}
	}
	store.SaveCredential(&cred)
	c.JSON(http.StatusOK, gin.H{"id": cred.ID})
}

func DeleteCredential(c *gin.Context) {
	store.DeleteCredential(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func UninstallCatpaw(c *gin.Context) {
	var req struct {
		RemoteCredential
		CredentialID  string `json:"credential_id"`
		SafetyConfirm string `json:"safety_confirm"`
		TestBatchID   string `json:"test_batch_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !requireSavedCredential(c, &req.RemoteCredential, req.CredentialID) {
		return
	}
	if decision := security.ValidateRemoteHost(req.IP); !decision.Allowed {
		auditEvent(c, "remote.uninstall", req.IP, decision.Level, "reject", decision.Reason, req.TestBatchID)
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}
	var script string
	if strings.EqualFold(req.Protocol, "winrm") {
		script = `schtasks /End /TN Catpaw /F 2>$null; schtasks /Delete /TN Catpaw /F 2>$null; Stop-Process -Name catpaw -Force -ErrorAction SilentlyContinue; Remove-Item -Recurse -Force C:\catpaw -ErrorAction SilentlyContinue; Write-Output "catpaw uninstalled"`
	} else if strings.EqualFold(req.Protocol, "wmi") {
		script = `schtasks /End /TN Catpaw /F 2>$null; schtasks /Delete /TN Catpaw /F 2>$null; Stop-Process -Name catpaw -Force -ErrorAction SilentlyContinue; Remove-Item -Recurse -Force C:\catpaw -ErrorAction SilentlyContinue; Write-Output "catpaw uninstalled"`
	} else {
		script = `pkill -f 'catpaw run' || true; rm -f /usr/local/bin/catpaw; rm -rf /etc/catpaw; echo "catpaw uninstalled"`
	}
	commandDecision := security.ValidateConfirm(security.ClassifyCommand(script), req.SafetyConfirm)
	if !commandDecision.Allowed {
		status := http.StatusPreconditionRequired
		if commandDecision.Level == "L4" {
			status = http.StatusForbidden
		}
		auditEvent(c, "remote.uninstall", req.IP, commandDecision.Level, "reject", commandDecision.Reason, req.TestBatchID)
		c.JSON(status, gin.H{"error": commandDecision.Reason, "safety": commandDecision})
		return
	}
	auditEvent(c, "remote.uninstall", req.IP, commandDecision.Level, "allow", commandDecision.Reason, req.TestBatchID)
	var out string
	var err error
	if strings.EqualFold(req.Protocol, "winrm") {
		out, err = execWinRM(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
	} else if strings.EqualFold(req.Protocol, "wmi") {
		out, err = execWMI(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
	} else {
		out, err = execSSH(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}
