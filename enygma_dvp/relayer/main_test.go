package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRelayerEnforcesHeaderDeadline(t *testing.T) {
	if os.Getenv("ENYGMA_TEST_RUN_RELAYER") == "1" {
		main()
		return
	}

	addr, _ := startRelayer(t, "TestRelayerEnforcesHeaderDeadline", "")
	conn, err := net.DialTimeout("tcp4", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	headerStarted := time.Now()
	if err := conn.SetDeadline(headerStarted.Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET /health HTTP/1.1\r\nHost: localhost"); err != nil {
		t.Fatal(err)
	}
	var response [1]byte
	if _, err := conn.Read(response[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("relayer did not close the incomplete-header request: got %v, want EOF", err)
	}
	if elapsed := time.Since(headerStarted); elapsed < time.Second {
		t.Fatalf("relayer closed the connection too early to exercise its header deadline: %s", elapsed)
	}
}

func TestRelayerLogsAWarningWhenMinFeeIsZero(t *testing.T) {
	if os.Getenv("ENYGMA_TEST_RUN_RELAYER") == "1" {
		main()
		return
	}

	for _, tc := range []struct {
		minFee string
		warn   bool
	}{
		{"", true},
		{"1", false},
	} {
		t.Run("RELAYER_MIN_FEE="+tc.minFee, func(t *testing.T) {
			_, output := startRelayer(t, "TestRelayerLogsAWarningWhenMinFeeIsZero", tc.minFee)
			if got := strings.Contains(output(), "WARNING: RELAYER_MIN_FEE"); got != tc.warn {
				t.Fatalf("startup log has the minimum-fee warning: %v, want %v\n%s", got, tc.warn, output())
			}
		})
	}
}

// startRelayer runs the test binary as the relayer (the named test calls main
// when ENYGMA_TEST_RUN_RELAYER is 1), waits until it listens, and returns its
// address and a function reading its log so far. The relayer is stopped when
// t ends.
func startRelayer(t *testing.T, testName, minFee string) (string, func() string) {
	t.Helper()
	root := t.TempDir()
	relayerDir := filepath.Join(root, "relayer")
	if err := os.Mkdir(relayerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"artifacts/contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json",
		"artifacts/contracts/core/contracts/vaults/AbstractCoinVault.sol/AbstractCoinVault.json",
		"artifacts/contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json",
	} {
		file := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(`{"abi":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENYGMA_TEST_RUN_RELAYER", "1")
	t.Setenv("RELAYER_PORT", fmt.Sprint(addr.Port))
	t.Setenv("RELAYER_DVP_ADDR", "0x0000000000000000000000000000000000000001")
	t.Setenv("RELAYER_PRIVATE_KEY", fmt.Sprintf("%064x", 1))
	t.Setenv("RELAYER_API_KEY", "test")
	t.Setenv("RELAYER_RPC_URL", "http://127.0.0.1:1")
	t.Setenv("RELAYER_TRUSTED_PROXIES", "")
	t.Setenv("RELAYER_CHAIN_ID", "1337")
	t.Setenv("RELAYER_FEE_SPEND_PRIVATE_KEY", "")
	t.Setenv("RELAYER_MIN_FEE", minFee)
	t.Setenv("RELAYER_TX_TIMEOUT", "")

	log, err := os.Create(filepath.Join(root, "relayer.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	output := func() string {
		b, err := os.ReadFile(log.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+testName+"$")
	cmd.Dir = relayerDir
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	startupDeadline := time.Now().Add(4 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp4", addr.String(), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return addr.String(), output
		}
		if time.Now().After(startupDeadline) {
			t.Fatalf("relayer did not listen: %v\n%s", err, output())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
