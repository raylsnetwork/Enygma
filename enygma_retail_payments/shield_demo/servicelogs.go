package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

// services are the stack's services whose logs the terminal shows, by the
// name run.sh gives each one's log file in its log directory.
var services = []string{"hardhat", "gnark", "relayer"}

// tailPoll is how often a tailed log is checked for new output.
const tailPoll = 100 * time.Millisecond

// termEvent is one line of a service's log, as the page's terminal gets it.
type termEvent struct {
	Kind    string `json:"kind"`
	Service string `json:"service"`
	Line    string `json:"line"`
	TS      string `json:"ts"`
}

// tail follows path from its start until ctx ends, calling emit with each
// complete line. The file may not exist yet; if it shrinks it is read again
// from the start.
func tail(ctx context.Context, path string, emit func(line string)) {
	var (
		f       *os.File
		offset  int64
		partial []byte
		buf     = make([]byte, 64*1024)
	)
	defer func() {
		if f != nil {
			f.Close()
		}
	}()
	tick := time.NewTicker(tailPoll)
	defer tick.Stop()
	for {
		if f == nil {
			f, _ = os.Open(path)
		}
		if f != nil {
			if st, err := f.Stat(); err == nil && st.Size() < offset {
				offset, partial = 0, nil
				if _, err := f.Seek(0, io.SeekStart); err != nil {
					// Reopened from the start on the next poll.
					f.Close()
					f = nil
				}
			}
		}
		if f != nil {
			for {
				n, err := f.Read(buf)
				offset += int64(n)
				partial = append(partial, buf[:n]...)
				for {
					i := bytes.IndexByte(partial, '\n')
					if i < 0 {
						break
					}
					emit(string(bytes.TrimSuffix(partial[:i], []byte("\r"))))
					partial = partial[i+1:]
				}
				if n == 0 || err != nil {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// tailServiceLogs streams each service's log in dir to the broker.
func (b *broker) tailServiceLogs(ctx context.Context, dir string) {
	for _, svc := range services {
		go tail(ctx, filepath.Join(dir, svc+".log"), func(line string) {
			b.publishLine(termEvent{Kind: "term", Service: svc, Line: line, TS: stamp()})
		})
	}
}
