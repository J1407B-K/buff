package buff

import "strings"

func ensureProtoAddr(addr string) string {
	if strings.Contains(addr, "://") {
		return addr
	}
	return "tcp://" + addr
}
