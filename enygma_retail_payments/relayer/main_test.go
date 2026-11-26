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
	"testing"
	"time"
)

func TestRelayerEnforcesHeaderDeadline(t *testing.T) {
	if os.Getenv("ENYGMA_TEST_RUN_RELAYER") == "1" {
		main()
		return
	}

	root := t.TempDir()
	relayerDir := filepath.Join(root, "relayer")
	if err := os.Mkdir(relayerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"contracts/abis/EnygmaDvp.json",
		"contracts/abis/Erc20CoinVault.json",
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
	t.Setenv("RELAYER_ERC20_VAULT_ADDR", "0x0000000000000000000000000000000000000002")
	t.Setenv("RELAYER_PRIVATE_KEY", fmt.Sprintf("%064x", 1))
	t.Setenv("RELAYER_API_KEY", "test")
	t.Setenv("RELAYER_RPC_URL", "http://127.0.0.1:1")
	t.Setenv("RELAYER_TRUSTED_PROXIES", "")
	t.Setenv("RELAYER_CHAIN_ID", "1337")
	t.Setenv("RELAYER_FEE_SPEND_PRIVATE_KEY", "")
	t.Setenv("RELAYER_MIN_FEE", "")
	t.Setenv("RELAYER_TX_TIMEOUT", "")

	log, err := os.Create(filepath.Join(root, "relayer.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRelayerEnforcesHeaderDeadline$")
	cmd.Dir = relayerDir
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	var conn net.Conn
	startupDeadline := time.Now().Add(4 * time.Second)
	for {
		conn, err = net.DialTimeout("tcp4", addr.String(), 100*time.Millisecond)
		if err == nil {
			break
		}
		if time.Now().After(startupDeadline) {
			output, readErr := os.ReadFile(log.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}
			t.Fatalf("relayer did not listen: %v\n%s", err, output)
		}
		time.Sleep(20 * time.Millisecond)
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
