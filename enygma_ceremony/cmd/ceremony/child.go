package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"enygma_ceremony/ceremony"
)

// A participant's contribution runs in a child process: this program, started
// again with childEnv set. The child reads the state on stdin, contributes,
// writes the contribution to stdout and, on stderr, one JSON line per progress
// report and a last one with its receipt (or its error); then it exits. The
// randomness lives only in the child, which is gone as soon as the
// contribution is written: the participant's program, which carries on to
// upload, print the receipt and wait for Enter, never holds it. Core dumps
// are off in both (disableCoreDumps).

// childEnv, set to 1, makes this program a contribution's child.
const childEnv = "ENYGMA_CEREMONY_CONTRIBUTE"

// childSizeEnv tells the child the size of the state its parent sends, which
// the parent has bounded (join -max-state); the child reads that and no more.
const childSizeEnv = "ENYGMA_CEREMONY_STATE_SIZE"

// childMessage is one line the child writes on stderr.
type childMessage struct {
	Progress *ceremony.Progress `json:"progress,omitempty"`
	Receipt  *ceremony.Receipt  `json:"receipt,omitempty"`
	Error    string             `json:"error,omitempty"`
}

// isChild reports whether this process is a contribution's child.
func isChild() bool { return os.Getenv(childEnv) == "1" }

// runChild is the child: it contributes to the state on stdin, and returns
// its exit code.
func runChild(stdin io.Reader, stdout, stderr io.Writer) int {
	disableCoreDumps()
	enc := json.NewEncoder(stderr)
	fail := func(err error) int {
		_ = enc.Encode(childMessage{Error: err.Error()}) // the exit code says it failed if this does not get through
		return 1
	}
	size, err := strconv.ParseInt(os.Getenv(childSizeEnv), 10, 64)
	if err != nil || size < 1 {
		return fail(fmt.Errorf("the size of the state to contribute to is not given (%s)", childSizeEnv))
	}
	state, err := io.ReadAll(io.LimitReader(stdin, size+1))
	if err != nil {
		return fail(err)
	}
	if int64(len(state)) != size {
		return fail(fmt.Errorf("a state of %d bytes, announced as %d", len(state), size))
	}
	out := bufio.NewWriterSize(stdout, 1<<20)
	receipt, err := ceremony.ContributeWithProgress(bytes.NewReader(state), int64(len(state)), out, func(p ceremony.Progress) {
		_ = enc.Encode(childMessage{Progress: &p}) // progress is for show
	})
	if err == nil {
		err = out.Flush()
	}
	if err != nil {
		return fail(err)
	}
	// The receipt carries hashes and numbers only: a Phase 2's names come
	// from the state the coordinator handed over, and the parent needs none
	// of them (audit I15).
	receipt.Ceremony, receipt.Circuits, receipt.R1CS = "", nil, nil
	if err := enc.Encode(childMessage{Receipt: &receipt}); err != nil {
		return 1
	}
	return 0
}

// contributeInChild contributes as ceremony.ContributeWithProgress does, in a
// child process (participant.Client.Contribute).
func contributeInChild(ctx context.Context) func(in io.Reader, size int64, out io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error) {
	return func(in io.Reader, size int64, out io.Writer, report func(ceremony.Progress)) (ceremony.Receipt, error) {
		exe, err := os.Executable()
		if err != nil {
			return ceremony.Receipt{}, fmt.Errorf("finding this program, to contribute in a process of its own: %w", err)
		}
		cmd := exec.CommandContext(ctx, exe)
		cmd.Env = append(os.Environ(), childEnv+"=1", childSizeEnv+"="+strconv.FormatInt(size, 10))
		cmd.Stdin, cmd.Stdout = io.LimitReader(in, size), out
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return ceremony.Receipt{}, err
		}
		if err := cmd.Start(); err != nil {
			return ceremony.Receipt{}, fmt.Errorf("starting the contribution: %w", err)
		}
		var (
			receipt  *ceremony.Receipt
			childErr string
			other    []string // what is not a message: a crash's report
		)
		lines := bufio.NewScanner(stderr)
		lines.Buffer(make([]byte, 64<<10), 1<<20)
		for lines.Scan() {
			var m childMessage
			if json.Unmarshal(lines.Bytes(), &m) != nil {
				if len(other) < 20 {
					other = append(other, lines.Text())
				}
				continue
			}
			switch {
			case m.Progress != nil:
				if report != nil {
					report(*m.Progress)
				}
			case m.Receipt != nil:
				receipt = m.Receipt
			case m.Error != "":
				childErr = m.Error
			}
		}
		// A line past the scanner's limit ends the scan; the child may still
		// be writing, and would block for ever on a pipe nobody reads while
		// Wait waits for it (audit I15). Read it to the end first.
		_, _ = io.Copy(io.Discard, stderr)
		why := ""
		if errors.Is(lines.Err(), bufio.ErrTooLong) {
			why = " (it wrote a line past 1 MiB, and what followed was not read)"
		}
		waited := cmd.Wait()
		switch {
		case childErr != "":
			return ceremony.Receipt{}, errors.New(childErr)
		case waited != nil:
			return ceremony.Receipt{}, fmt.Errorf("the contribution stopped: %v%s: %s", waited, why, strings.Join(other, " | "))
		case receipt == nil:
			return ceremony.Receipt{}, errors.New("the contribution ended without a receipt" + why)
		}
		return *receipt, nil
	}
}
