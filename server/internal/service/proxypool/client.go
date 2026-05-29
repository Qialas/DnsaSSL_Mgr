package proxypool

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"qdl/server/internal/model"
	"strconv"
	"strings"
	"sync"
	"time"

	xproxy "golang.org/x/net/proxy"
	"gorm.io/gorm"
)

const (
	ModeManual = "manual"
	ModeAuto   = "auto"
)

var (
	roundRobinMu      sync.Mutex
	roundRobinCounter = map[string]int{}
)

func Resolve(db *gorm.DB, scope string, useProxy bool, mode string, proxyID uint) (*model.ProxySetting, error) {
	if !useProxy {
		return nil, nil
	}
	if strings.TrimSpace(mode) == "" {
		mode = ModeManual
	}
	if mode == ModeAuto {
		var rows []model.ProxySetting
		if err := db.Where("status = ?", model.StatusEnabled).Order("id ASC").Find(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("代理池没有可用代理")
		}
		roundRobinMu.Lock()
		index := roundRobinCounter[scope] % len(rows)
		roundRobinCounter[scope]++
		roundRobinMu.Unlock()
		return &rows[index], nil
	}
	if proxyID == 0 {
		return nil, fmt.Errorf("请选择代理池节点")
	}
	var row model.ProxySetting
	if err := db.First(&row, proxyID).Error; err != nil {
		return nil, fmt.Errorf("代理池节点不存在")
	}
	if row.Status != model.StatusEnabled {
		return nil, fmt.Errorf("代理池节点未启用")
	}
	return &row, nil
}

func Client(setting *model.ProxySetting, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if setting == nil {
		return &http.Client{Timeout: timeout}, nil
	}
	if err := Validate(setting); err != nil {
		return nil, err
	}
	transport := &http.Transport{}
	addr := net.JoinHostPort(setting.Host, strconv.Itoa(setting.Port))
	switch setting.Protocol {
	case "sock4":
		transport.DialContext = func(ctx context.Context, network, target string) (net.Conn, error) {
			return dialSOCKS4(ctx, addr, target, setting.Username)
		}
	case "sock5", "sock5h":
		var auth *xproxy.Auth
		if setting.Username != "" || setting.Password != "" {
			auth = &xproxy.Auth{User: setting.Username, Password: setting.Password}
		}
		dialer, err := xproxy.SOCKS5("tcp", addr, auth, xproxy.Direct)
		if err != nil {
			return nil, err
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("当前 SOCK5 代理不支持上下文拨号")
		}
		transport.DialContext = func(ctx context.Context, network, target string) (net.Conn, error) {
			if setting.Protocol == "sock5" {
				localTarget, err := resolveTargetLocally(ctx, target)
				if err != nil {
					return nil, err
				}
				target = localTarget
			}
			return contextDialer.DialContext(ctx, network, target)
		}
	default:
		transport.Proxy = http.ProxyURL(URL(*setting))
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func URL(setting model.ProxySetting) *url.URL {
	addr := net.JoinHostPort(setting.Host, strconv.Itoa(setting.Port))
	scheme := setting.Protocol
	if scheme == "sock4" {
		scheme = "socks4"
	}
	if scheme == "sock5" {
		scheme = "socks5"
	}
	if scheme == "sock5h" {
		scheme = "socks5h"
	}
	proxyURL := &url.URL{Scheme: scheme, Host: addr}
	if setting.Username != "" || setting.Password != "" {
		proxyURL.User = url.UserPassword(setting.Username, setting.Password)
	}
	return proxyURL
}

func Validate(row *model.ProxySetting) error {
	row.Protocol = strings.ToLower(strings.TrimSpace(row.Protocol))
	row.Host = strings.TrimSpace(row.Host)
	row.Name = strings.TrimSpace(row.Name)
	if row.Protocol == "" {
		row.Protocol = "http"
	}
	if row.Status == "" {
		row.Status = model.StatusEnabled
	}
	if row.Weight < 1 {
		row.Weight = 1
	}
	if row.Name == "" {
		return fmt.Errorf("请输入代理名称")
	}
	if row.Host == "" {
		return fmt.Errorf("请输入代理主机")
	}
	if row.Port < 1 || row.Port > 65535 {
		return fmt.Errorf("请输入有效端口")
	}
	switch row.Protocol {
	case "http", "https", "sock4", "sock5", "sock5h":
		return nil
	default:
		return fmt.Errorf("代理协议仅支持 http、https、sock4、sock5、sock5h")
	}
}

func resolveTargetLocally(ctx context.Context, target string) (string, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if ip.IP.To4() != nil || ip.IP.To16() != nil {
			return net.JoinHostPort(ip.IP.String(), port), nil
		}
	}
	return "", fmt.Errorf("无法解析目标地址：%s", host)
}

func dialSOCKS4(ctx context.Context, proxyAddr string, target string, username string) (net.Conn, error) {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("目标端口无效：%d", port)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var ipv4 net.IP
	for _, item := range ips {
		if item.IP.To4() != nil {
			ipv4 = item.IP.To4()
			break
		}
	}
	if ipv4 == nil {
		return nil, fmt.Errorf("SOCK4 仅支持 IPv4 目标地址：%s", host)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
		defer conn.SetDeadline(time.Time{})
	}
	req := make([]byte, 0, 9+len(username))
	req = append(req, 0x04, 0x01)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)
	req = append(req, ipv4...)
	req = append(req, []byte(username)...)
	req = append(req, 0x00)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	res := make([]byte, 8)
	if _, err := io.ReadFull(reader, res); err != nil {
		conn.Close()
		return nil, err
	}
	if res[1] != 0x5a {
		conn.Close()
		return nil, fmt.Errorf("SOCK4 代理连接失败，响应码 0x%02x", res[1])
	}
	return conn, nil
}
