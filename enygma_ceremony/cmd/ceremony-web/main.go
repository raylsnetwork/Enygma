//go:build js && wasm

// Command ceremony-web is the participant's program built for the browser
// (GOOS=js GOARCH=wasm; make ceremony-dist builds it as ceremony.wasm). The
// coordinator's page runs it in a Web Worker (coordinator/web/worker.js),
// which calls
//
//	enygmaContribute({url, name}, onEvent) -> Promise<result>
//
// to take a participant through a phase exactly as `ceremony join` does: the
// same client (package participant), the same contribution. onEvent receives
// each participant.Event as JSON; the promise resolves to the contribution
// as JSON ({index, sha256, prev, already}), or rejects with the reason.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"syscall/js"

	"enygma_ceremony/participant"
)

func main() {
	js.Global().Set("enygmaContribute", js.FuncOf(contribute))
	select {} // the page calls in; the program never ends on its own
}

// contribute is enygmaContribute.
func contribute(_ js.Value, args []js.Value) any {
	opts, onEvent := args[0], args[1]
	field := func(k string) string {
		if v := opts.Get(k); v.Type() == js.TypeString {
			return v.String()
		}
		return ""
	}
	return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, p []js.Value) any {
		resolve, reject := p[0], p[1]
		go func() {
			url := field("url")
			c := &participant.Client{
				URL: url, Name: field("name"),
				HTTP:      &http.Client{},
				GitHub:    url + "/github", // GitHub answers no browser; the coordinator relays its sign-in
				Heartbeat: -1,              // the page's heartbeat worker sends them: this thread is busy contributing
				Events: func(e participant.Event) {
					data, err := json.Marshal(e)
					if err == nil {
						onEvent.Invoke(string(data))
					}
				},
			}
			if put := js.Global().Get("enygmaPut"); put.Type() == js.TypeFunction {
				c.Put = browserPut(put)
			}
			r, err := c.Run(context.Background())
			if err != nil {
				reject.Invoke(err.Error())
				return
			}
			data, err := json.Marshal(map[string]any{
				"index": r.Index, "sha256": fmt.Sprintf("%x", r.SHA256), "prev": fmt.Sprintf("%x", r.Prev), "already": r.Already,
			})
			if err != nil {
				reject.Invoke(err.Error())
				return
			}
			resolve.Invoke(string(data))
		}()
		return nil
	}))
}

// browserPut uploads through put, the worker's enygmaPut(url, token, bytes,
// onProgress) -> Promise<{status, body}>, which uses the browser's own upload
// API: Go's requests in the browser send a body whole, with no progress.
func browserPut(put js.Value) func(context.Context, string, string, []byte, func(int64)) (int, []byte, error) {
	return func(ctx context.Context, url, token string, body []byte, sent func(int64)) (int, []byte, error) {
		data := js.Global().Get("Uint8Array").New(len(body))
		js.CopyBytesToJS(data, body)
		type answer struct {
			status int
			body   []byte
			err    error
		}
		done := make(chan answer, 1)
		progress := js.FuncOf(func(_ js.Value, a []js.Value) any {
			sent(int64(a[0].Float()))
			return nil
		})
		then := js.FuncOf(func(_ js.Value, a []js.Value) any {
			done <- answer{status: a[0].Get("status").Int(), body: []byte(a[0].Get("body").String())}
			return nil
		})
		catch := js.FuncOf(func(_ js.Value, a []js.Value) any {
			done <- answer{err: errors.New(a[0].Get("message").String())}
			return nil
		})
		defer progress.Release()
		defer then.Release()
		defer catch.Release()
		put.Invoke(url, token, data, progress).Call("then", then, catch)
		select {
		case a := <-done:
			return a.status, a.body, a.err
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		}
	}
}
