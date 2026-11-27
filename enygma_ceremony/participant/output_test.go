package participant

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"enygma_ceremony/coordinator"
)

// forged is what a server, or something on the network answering in its
// place, could write to draw the end of a turn on the participant's
// terminal: clear the screen, and show a receipt for its own sha256.
const forged = "\x1b[2J\x1b[HDone: your contribution sha256 abab  <- publish this"

// fooled reports whether out shows any of forged: its escape sequence, or
// its receipt, which a participant could take for the program's.
func fooled(out string) bool { return strings.Contains(out, "\x1b") || strings.Contains(out, "Done:") }

// TestCertificateTextIsNotShown (audit M1): a certificate that does not
// verify ends the run at once, and its names, which whoever made it chose,
// are not printed: they could hold escape sequences that draw a fake
// receipt. The client trusts the certificate's maker here, so that only its
// names are wrong: the failure Go reports by listing them.
func TestCertificateTextIsNotShown(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: forged},
		DNSNames:              []string{forged + ".example"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	defer srv.Close()

	client := HTTPClient()
	tr := client.Transport.(plainErrors).rt.(*http.Transport)
	tr.TLSClientConfig = &tls.Config{RootCAs: x509.NewCertPool()}
	tr.TLSClientConfig.RootCAs.AddCert(cert)
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	var out bytes.Buffer
	c := &Client{URL: "https://coordinator.test", HTTP: client, Out: &out}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = c.Run(ctx)
	if !errors.Is(err, errCertificate) {
		t.Fatalf("got %v, want the certificate refused at once", err)
	}
	if fooled(err.Error()) || fooled(out.String()) {
		t.Fatalf("the certificate's text was shown: %q, then %q", out.String(), err)
	}
}

// TestProxyRefusalIsShownByItsCode (audit M1): a proxy refusing to connect
// is reported by its status code, not by the reason phrase it sent with it,
// which anything answering as the proxy chose.
func TestProxyRefusalIsShownByItsCode(t *testing.T) {
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proxy.Close() }()
	go func() {
		for {
			conn, err := proxy.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				fmt.Fprintf(conn, "HTTP/1.1 403 %s\r\nContent-Length: 0\r\n\r\n", forged)
			}()
		}
	}()
	client := HTTPClient()
	client.Transport.(plainErrors).rt.(*http.Transport).Proxy = http.ProxyURL(&url.URL{Scheme: "http", Host: proxy.Addr().String()})
	resp, err := client.Get("https://coordinator.test/v1/config")
	if err == nil {
		resp.Body.Close()
		t.Fatal("got an answer through a proxy that refused")
	}
	if !errors.Is(err, errProxy) || !strings.Contains(err.Error(), "403") || fooled(err.Error()) {
		t.Fatalf("got %q, want the proxy's refusal by its code alone", err)
	}
}

// TestRetryingIsOneLine (audit M1): an error printed while trying again is
// one line of plain text, whatever it carries: it cannot start a line of
// its own, such as a fake receipt.
func TestRetryingIsOneLine(t *testing.T) {
	var out bytes.Buffer
	c := &Client{Out: &out}
	c.retrying("Cannot reach the coordinator", &url.Error{Op: "Get", URL: "https://coordinator.test", Err: errors.New("\r\n" + forged)}, "trying again")
	if got := out.String(); strings.Count(got, "\n") != 1 || strings.Contains(got, "\x1b") || !strings.HasSuffix(got, "trying again.\n") {
		t.Fatalf("printed %q", got)
	}
}

// TestGitHubTextIsNotShown (audit I1): what GitHub's sign-in answers, a
// failure's code or its status line, reaches the terminal as plain text.
func TestGitHubTextIsNotShown(t *testing.T) {
	for _, c := range []struct {
		name  string
		grant func(w http.ResponseWriter)
	}{
		{"an error code", func(w http.ResponseWriter) {
			json.NewEncoder(w).Encode(map[string]string{"error": "\n" + forged}) // JSON escapes ESC as \u001b, which decodes back to it
		}},
		{"a status line", func(w http.ResponseWriter) {
			conn, buf, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			fmt.Fprintf(buf, "HTTP/1.1 500 %s\r\nContent-Length: 0\r\n\r\n", forged)
			_ = buf.Flush() // the client reads what got through
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/login/device/code":
					fmt.Fprint(w, `{"device_code":"dc","user_code":"WDJB-MJHT","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`)
				case "/login/oauth/access_token":
					c.grant(w)
				}
			}))
			defer github.Close()
			cl := &Client{GitHub: github.URL, HTTP: http.DefaultClient, Out: io.Discard}
			_, err := cl.githubSignIn(context.Background(), "Iv1.app")
			if err == nil || strings.ContainsAny(err.Error(), "\x1b\n") {
				t.Fatalf("got %q, want GitHub's answer as one line of plain text", err)
			}
		})
	}
}

// TestReceiptOnAcceptance (audit L5, H2, L10): the program prints the
// receipt once the coordinator's reply to its upload names it, before it
// reads the transcript back: a coordinator stopping at the close may not
// answer that. It reads the transcript back as anyone does, with no
// session token, and reports whether it could.
func TestReceiptOnAcceptance(t *testing.T) {
	t.Run("a transcript that is no chain", func(t *testing.T) {
		p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
		inner := *p.handler.Load()
		wrapped := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/transcript" {
				inner.ServeHTTP(w, r)
				return
			}
			rec := httptest.NewRecorder()
			inner.ServeHTTP(rec, r)
			var tr map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
				t.Error(err)
			}
			tr["start"] = strings.Repeat("00", 32) // the entries no longer build on it
			_ = json.NewEncoder(w).Encode(tr)
		}))
		p.handler.Store(&wrapped)
		if _, err := p.client(t, "alice").Run(context.Background()); err == nil || !strings.Contains(err.Error(), "not contribution 1 on top of") {
			t.Fatalf("Run against a transcript that is no chain: %v, want it refused", err)
		}
	})
	for _, down := range []bool{false, true} {
		p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
		inner := *p.handler.Load()
		var withToken atomic.Bool
		wrapped := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/transcript" {
				withToken.Store(withToken.Load() || r.Header.Get("Authorization") != "")
				if down {
					closeConnection(t, w)
					return
				}
			}
			inner.ServeHTTP(w, r)
		}))
		p.handler.Store(&wrapped)
		var out syncBuffer
		c := p.client(t, "alice")
		c.Out = &out
		r, err := c.Run(context.Background())
		if err != nil {
			t.Fatalf("transcript down %v: %v", down, err)
		}
		if want := fmt.Sprintf("accepted your contribution as number 1: sha256 %x", r.SHA256); !strings.Contains(out.String(), want) {
			t.Fatalf("transcript down %v: printed\n%s\nwant %q", down, out.String(), want)
		}
		if r.Confirmed == down || withToken.Load() {
			t.Fatalf("transcript down %v: confirmed %v, read with a session token %v", down, r.Confirmed, withToken.Load())
		}
	}
}

// TestTimeoutIsUnreachable (audit L6): a reply that does not come within
// the client's own timeout is the coordinator unreachable for now, which
// asking again may get past; the caller's context ending is not.
func TestTimeoutIsUnreachable(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer slow.Close()
	client := HTTPClient()
	client.Transport.(plainErrors).rt.(*http.Transport).ResponseHeaderTimeout = 50 * time.Millisecond
	resp, err := client.Get(slow.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("no timeout")
	}
	if !unreachable(context.Background(), err) {
		t.Fatalf("a reply past the client's timeout (%v) is not taken as unreachable", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if unreachable(ctx, err) {
		t.Fatal("an error after the caller gave up is taken as unreachable")
	}
}

// swallow is a response that goes nowhere, but still reaches the
// connection underneath for deadlines and hijacking.
type swallow struct{ http.ResponseWriter }

func (s swallow) WriteHeader(int)                     {}
func (s swallow) Write(b []byte) (int, error)         { return len(b), nil }
func (s swallow) Unwrap() http.ResponseWriter         { return s.ResponseWriter }
func (s swallow) Header() http.Header                 { return http.Header{} }
func (s swallow) ReadFrom(r io.Reader) (int64, error) { return io.Copy(io.Discard, r) }

// TestReceiptWhenTheAnswerIsLost (audit L5): when the coordinator accepts
// an upload but its answer is lost, the program still prints the receipt:
// once the coordinator says it has the contribution, or, if it has closed
// meanwhile, as one that may be in the transcript. A transcript cut short
// as it is read back leaves the receipt standing, unconfirmed.
func TestReceiptWhenTheAnswerIsLost(t *testing.T) {
	run := func(t *testing.T, then func(p *phase), cut bool) (*syncBuffer, Result, error) {
		p := newPhase(t, coordinator.Config{Auth: coordinator.AuthOpen})
		inner := *p.handler.Load()
		var dropped atomic.Bool
		wrapped := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPut && r.URL.Path == "/v1/contribution" && then != nil && dropped.CompareAndSwap(false, true):
				inner.ServeHTTP(swallow{w}, r) // accepted, the answer lost
				then(p)
				closeConnection(t, w)
				return
			case r.URL.Path == "/v1/transcript" && cut:
				conn, buf, err := http.NewResponseController(w).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				fmt.Fprint(buf, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{\"start\":")
				_ = buf.Flush()
				return
			}
			inner.ServeHTTP(w, r)
		}))
		p.handler.Store(&wrapped)
		var out syncBuffer
		c := p.client(t, "alice")
		c.Out = &out
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		r, err := c.Run(ctx)
		return &out, r, err
	}
	t.Run("the answer lost", func(t *testing.T) {
		out, r, err := run(t, func(*phase) {}, false)
		if err != nil || !r.Confirmed || !strings.Contains(out.String(), "accepted your contribution as number 1") {
			t.Fatalf("got %+v, %v; printed\n%s", r, err, out.String())
		}
	})
	t.Run("the answer lost, and the phase closed", func(t *testing.T) {
		// Restarted closed, the coordinator forgets the session, and answers
		// the program joining again that the phase is closed.
		out, _, err := run(t, func(p *phase) {
			p.cfg.CloseAt = time.Now().Add(-time.Second)
			p.restart(t)
		}, false)
		if !errors.Is(err, ErrClosed) || !strings.Contains(out.String(), "was uploaded, but the coordinator's answer was lost") {
			t.Fatalf("got %v; printed\n%s", err, out.String())
		}
	})
	t.Run("the transcript cut short", func(t *testing.T) {
		out, r, err := run(t, nil, true)
		if err != nil || r.Confirmed || !strings.Contains(out.String(), "accepted your contribution as number 1") {
			t.Fatalf("got %+v, %v; printed\n%s", r, err, out.String())
		}
	})
}
