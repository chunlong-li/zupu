package discovery

import (
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/mdns"
)

// StartMDNS 用标准 mDNS/Bonjour 广播本服务（服务类型 _zupu._tcp）。
//
// 适用场景：
//   - 非容器安装（二进制 / tar.gz / zip 直接跑在宿主机）→ 多播可达局域网，手机 App 能自动发现；
//   - 容器安装必须用 host 网络（bridge 不转发多播，广播出不去）。
//
// 关掉：FAMILY_TREE_MDNS=0
func StartMDNS(servicePort int, version string) {
	if !mdnsEnabled() {
		log.Printf("[发现] mDNS 已关闭（FAMILY_TREE_MDNS=0）")
		return
	}
	port := servicePort
	if p := observed(); p > 0 { // 复用「从请求里学到的对外端口」
		port = p
	}
	name := "zupu"
	if v := strings.TrimSpace(os.Getenv("FAMILY_TREE_MDNS_NAME")); v != "" {
		name = v
	}
	instance := name + "-" + strconv.Itoa(port)

	txt := []string{"app=zupu", "path=/", "name=族谱系统", "port=" + strconv.Itoa(port)}
	if version != "" {
		txt = append(txt, "version="+version)
	}

	var ips []net.IP
	if picked := pickLanIP(); picked != "" {
		if ip := net.ParseIP(picked); ip != nil {
			ips = []net.IP{ip.To4()}
		}
	}

	// Linux 上如果宿主机已经跑着 avahi-daemon，它会独占 5353，本库自己发不出去；
	// 这种情况优先借用系统的 avahi-publish-service（和侧车同一条路），最稳。
	if path, err := exec.LookPath("avahi-publish-service"); err == nil && dialAvahiOK() {
		args := []string{instance, "_zupu._tcp", strconv.Itoa(port)}
		for _, t := range txt {
			args = append(args, t)
		}
		cmd := exec.Command(path, args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err == nil {
			log.Printf("[发现] mDNS 已通过系统 avahi 广播：%s _zupu._tcp 端口 %d", instance, port)
			go func() { _ = cmd.Wait() }()
			return
		}
	}

	svc, err := mdns.NewMDNSService(instance, "_zupu._tcp", "", "", port, ips, txt)
	if err != nil {
		log.Printf("[发现] mDNS 服务构造失败：%v", err)
		return
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		log.Printf("[发现] mDNS 广播启动失败（容器内需 host 网络）：%v", err)
		return
	}
	log.Printf("[发现] mDNS 已广播：%s _zupu._tcp 端口 %d", instance, port)
	_ = server // 常驻，不主动关闭
}

func mdnsEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FAMILY_TREE_MDNS")))
	return v != "0" && v != "false" && v != "off"
}

// pickLanIP 挑一个局域网 IPv4（优先 192.168 / 10. / 172.16-31）
func pickLanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var fallback string
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			s := ip.String()
			if strings.HasPrefix(s, "192.168.") || strings.HasPrefix(s, "10.") || strings.HasPrefix(s, "172.") {
				return s
			}
			if fallback == "" {
				fallback = s
			}
		}
	}
	return fallback
}

// dialAvahiOK 判断宿主机 avahi 的 dbus socket 是否可用
func dialAvahiOK() bool {
	_, err := os.Stat("/var/run/avahi-daemon/socket")
	return err == nil
}
