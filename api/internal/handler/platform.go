package handler

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func GetPlatformIP(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ip": PlatformIP()})
}

func PlatformIP() string {
	if ip := strings.TrimSpace(viper.GetString("server.server_ip")); ip != "" {
		return ip
	}
	return detectLocalIP()
}

func detectLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
