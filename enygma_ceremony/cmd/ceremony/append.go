package main

import (
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"enygma_ceremony/ceremony"
	"enygma_ceremony/coordinator"
	"enygma_ceremony/transcript"
)

// appendCmd adds a contribution made by hand on top of a phase's latest
// state as its next state, with its entry: the rescue for a contributor
// lined up who got no turn (ADR 0008). It takes the phase directory's
// lock, as a coordinator does, so it refuses a directory one is serving.
func appendCmd(fs *flag.FlagSet) func([]string, io.Writer) error {
	dir := fs.String("dir", "", "the phase's directory, which no coordinator is serving")
	in := fs.String("in", "", "the contribution, made with ceremony contribute on the phase's latest state")
	name := fs.String("name", "", "the contributor's name, as the transcript shows it")
	return func(args []string, w io.Writer) error {
		if *dir == "" || *in == "" || *name == "" || len(args) != 0 {
			return fmt.Errorf("%w: append takes -dir, -in and -name", errUsage)
		}
		// Opening the directory clears its incoming files, and a state
		// written into it is one the transcript has no entry for.
		switch within, err := inside(*in, *dir); {
		case err != nil:
			return err
		case within:
			return fmt.Errorf("%w: -in is a file inside -dir: write the contribution outside the phase's directory", errUsage)
		}
		c, err := coordinator.New(coordinator.Config{Dir: *dir})
		if err != nil {
			return err
		}
		defer c.Release()
		start, err := ceremony.Inspect(transcript.Path(*dir, 0)) // before the state is added, which then stands
		if err != nil {
			return err
		}
		e, err := c.Append(*in, *name)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Added %s's contribution as state %d of %s, on top of %s:\n"+
			"  head sha256 %s (contribution %d)\n"+
			"Check that every contributor you lined up for this phase is in this chain, then announce this head:\n"+
			"  ceremony verify -dir %s -head %s %s -mine <their published receipts, comma-separated>\n"+
			"and seal with their receipts:\n"+
			"  ceremony %s -transcript %s -head %s -include <their receipts> -drand <round> -close-at <the close>\n",
			e.Participant, e.Index, *dir, e.Prev, e.SHA256, e.Index, *dir, e.SHA256, phaseFlags(start), sealCommand(start), *dir, e.SHA256)
		return nil
	}
}

// inside reports whether path, or the file a link at path names, is in
// dir or under it: the directories each is in are compared with dir as
// files, so a link, or a name in another case, does not hide it.
func inside(path, dir string) (bool, error) {
	d, err := os.Stat(dir)
	if err != nil {
		return false, err
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}
	for _, p := range []string{path, target} {
		if p, err = filepath.Abs(p); err != nil {
			return false, err
		}
		for cur := filepath.Dir(p); ; cur = filepath.Dir(cur) {
			if fi, err := os.Stat(cur); err == nil && os.SameFile(fi, d) {
				return true, nil
			}
			if filepath.Dir(cur) == cur {
				break
			}
		}
	}
	return false, nil
}

// readDenyFile reads -deny-file: one address or CIDR a line; blank lines,
// and what follows a #, are left out.
func readDenyFile(path string) ([]netip.Prefix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []string
	for i, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#") // a line left empty is one parseProxies passes over
		if strings.Contains(line, ",") {
			return nil, fmt.Errorf("%s:%d: one address or CIDR a line", path, i+1)
		}
		items = append(items, strings.TrimSpace(line))
	}
	networks, err := parseProxies(strings.Join(items, ","))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return networks, nil
}
