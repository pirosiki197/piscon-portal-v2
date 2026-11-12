// Package sourceip は、runner ホストが対象サーバーに通信するときの送信元 IP を求める。
package sourceip

import (
	"fmt"
	"net"
)

// For は、targetIP に通信するときに OS が選ぶ送信元 IP を返す。
// 対象サーバーから runner ホストに戻ってくる通信の宛先に使う。パケットは送らない。
func For(targetIP string) (string, error) {
	// UDP の Dial は経路を選ぶだけで、パケットを送らない。ポート番号は経路の選択に影響しない。
	conn, err := net.Dial("udp", net.JoinHostPort(targetIP, "9"))
	if err != nil {
		return "", fmt.Errorf("find route to %s: %w", targetIP, err)
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected local address %v", conn.LocalAddr())
	}
	return addr.IP.String(), nil
}
