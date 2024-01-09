package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var timeout = 1

var lc = net.ListenConfig{
	Control: func(network, address string, c syscall.RawConn) error {
		var opErr error
		if err := c.Control(func(fd uintptr) {
			opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
			opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		}); err != nil {
			return err
		}
		return opErr
	},
}

func isIPAddress(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil
}

func isPort(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port > 0 && port <= 65535
}
func isMD5(input string) bool {
	md5Pattern := "^[0-9a-fA-F]{32}$"
	regex := regexp.MustCompile(md5Pattern)
	return regex.MatchString(input)
}
func main() {
	if len(os.Args) <= 5 && len(os.Args) >= 1 {
		if os.Args[0] == os.Args[1] {
			fmt.Println(os.Args[0] + " [lhost] [reuse prot] [rhost] [rport] [md5(myip)]")
			fmt.Println("myip is we link to its md5(IP)")
			fmt.Println("Use it carefully and bear the consequences.")
			os.Exit(0)
		}
		fmt.Println("'" + os.Args[0] + "' " + "is not recognized as an internal or external command, operable program or batch file")
		os.Exit(0)
	}
	lhost := os.Args[1]
	lprot := os.Args[2]
	rhost := os.Args[3]
	rport := os.Args[4]
	myip := os.Args[5]
	if !(isIPAddress(lhost) && isPort(lprot) && isIPAddress(rhost) && isPort(rport) && isMD5(myip)) {
		fmt.Println("'" + os.Args[0] + "' " + "is not recognized as an internal or external command, operable program or batch file")
		os.Exit(0)
	}
	if lhost == "0.0.0.0" || lhost == "127.0.0.1" || lhost == rhost {
		fmt.Println("lhost cannot be equal to 127.0.0.1,0.0.0.0 and rhost")
		os.Exit(0)
	} else if lprot == rport && lhost == rhost {
		fmt.Println("lprot cannot be equal to 1rport")
		os.Exit(0)
	} else if myip == lhost || myip == rhost {
		fmt.Println("myip cannot be equal to lhost and rhost")
		os.Exit(0)
	}
	laddr := fmt.Sprintf("%s:%s", lhost, lprot)
	l, err := lc.Listen(context.Background(), "tcp", laddr)
	go func() {
		time.Sleep(2 * time.Minute)
		l.Close()
		if timeout == 1 {
			os.Exit(0)
		}
	}()
	if err != nil {
		fmt.Println("无法监听端口:", err)
		return
	}
	fmt.Printf("开始监听端口 %s，将转发到 %s:%s\n", lprot, rhost, rport)
	for {
		clientConn, err := l.Accept()
		if err != nil {
			continue
		}
		addr_prot := strings.Split(clientConn.RemoteAddr().String(), ":")
		if fmt.Sprintf("%x", md5.Sum([]byte(addr_prot[0]))) == myip {
			timeout = 0
			go handleClient(clientConn, rhost, rport)
		} else {
			go handleClient(clientConn, rhost, lprot)
		}
	}
}

func handleClient(clientConn net.Conn, remoteHost, remotePort string) {
	// 连接到目标主机
	serverConn, err := net.Dial("tcp", remoteHost+":"+remotePort)
	if err != nil {
		fmt.Println("连接到目标主机时出错:", err)
		clientConn.Close()
		return
	}
	defer serverConn.Close()

	// 在 goroutine 中进行双向数据传输
	done := make(chan struct{})
	go copyData(clientConn, serverConn, done)
	go copyData(serverConn, clientConn, done)

	// 等待任意一个goroutine完成后关闭另一个连接
	<-done
	clientConn.Close()
	<-done
}

func copyData(dst io.Writer, src io.Reader, done chan<- struct{}) {
	buf := make([]byte, 4096) // 使用较大的缓冲区，提高性能
	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil && err != io.EOF {
		fmt.Println("数据传输时出错:", err)
	}
	done <- struct{}{}
}
