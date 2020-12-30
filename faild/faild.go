package faild

import (
    "os/exec"
	"log"
	"strings"
	"fmt"
	"bytes"
	"strconv"

	"golang.org/x/net/context"
	"github.com/vishvananda/netlink"
	"github.com/drael/GOnetstat"
)

type Server struct {
	Iface string
	VIP string
}

func (s *Server) Init(ctx context.Context, in *InitRequest) (*Status, error) {
	status := Status{
		Code: 0, 
		Message: "Success",
	}
	link, err := netlink.LinkByName("lo")
	if err != nil {
		log.Printf("unable to find loopback device. Error %q", err)
		status.Code = 5
		status.Message = "unable to find loopback device"
		return &status, err
	}
	log.Printf("Initializing with Virtual IP: %s", in.VIP)
	addr, err := netlink.ParseAddr(in.VIP)
	if err != nil {
		log.Printf("unable to parse VIP address. Error %q", err)
		status.Code = 3
		status.Message = "unable to parse VIP address"
		return &status, err
	}
	err = netlink.AddrReplace(link, addr)
	if err != nil {
		log.Printf("unable to assign VIP address. Error %q", err)
		status.Code = 3
		status.Message = "unable to assign VIP address"
		return &status, err
	}
	s.VIP = in.VIP
	return &status, nil
}

func (s *Server) Start(ctx context.Context, in *Empty) (*Status, error) {
	status := Status{
		Code: 0, 
		Message: "Success",
	}
	_, err :=  cmdOutput(fmt.Sprintf("faild -p %s", s.Iface))
	if err != nil {
		log.Printf("unable to start faild. Error %q", err)
		status.Code = 5
		status.Message = "unable to start faild"
		return &status, err
	}
	return &status, nil
}

func (s *Server) Stop(ctx context.Context, in *Empty) (*Status, error) {
	status := Status{
		Code: 0, 
		Message: "Success",
	}
	_, err :=  cmdOutput(fmt.Sprintf("faild -u %s", s.Iface))
	if err != nil {
		log.Printf("unable to stop faild. Error %q", err)
		status.Code = 5
		status.Message = "unable to stop faild"
		return &status, err
	}
	return &status, nil
}

func (s *Server) GetStatistics(ctx context.Context, in *Empty) (*Stats, error) {
	var estSessions int32
	stats := Stats{}
	stats.Stats = make(map[string]int64)
	log.Printf("Getting stats")
	tcpSessions := GOnetstat.Tcp()
	for _, session := range(tcpSessions) {
		if session.State == "ESTABLISHED" && strings.Contains(s.VIP, session.Ip) {
			estSessions++
		}
	}
	stats.Stats["established_tcp_sessions"] = int64(estSessions)
	if err := s.ParseFaildStats(&stats); err != nil {
		log.Printf("unable to parse faild statistics. Error %q", err)
	}
	return &stats, nil
}

func (s *Server) ParseFaildStats(stats *Stats) error {
	out, err :=  cmdOutput(fmt.Sprintf("faild -s %s", s.Iface))
	if err != nil {
		return err
	}
	lines := strings.Split(out, "\n")
	for _,line := range(lines) {
		kv := strings.Split(line, ":")
		if len(kv) < 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			return err
		}
		log.Printf("%s: %d", key, val)
		stats.Stats[key] = val
	}
	return nil
}


func cmdOutput(command string) (string, error) {
	cmds := strings.Fields(command)
	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	log.Printf("%v", cmd)
	if err := cmd.Run(); err != nil {
		log.Printf("unable to execute command. Error %q. Stdout: %s", err, cmdOutput.String())
		return "", err
	}
	return cmdOutput.String(), nil
}