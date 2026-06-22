package tests

import "net"

// serverAddr is the auction gnark server's address (distinct from the DvP
// server's :8081 and the retail payments server's :8082).
const serverAddr = "localhost:8083"

// serverAvailable reports whether something is listening on addr, so tests
// can skip cleanly rather than fail when the gnark server isn't running.
func serverAvailable(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
