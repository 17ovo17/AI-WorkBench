package model

type Credential struct {
	ID       string `json:"id"`
	Name     string `json:"name" binding:"required"`
	Protocol string `json:"protocol"` // ssh | winrm
	Username string `json:"username" binding:"required"`
	Password string `json:"password,omitempty"`
	SSHKey   string `json:"ssh_key,omitempty"`
	Port     int    `json:"port"`
	Remark   string `json:"remark"`
}
