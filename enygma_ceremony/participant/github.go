package participant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// githubSignIn signs the participant in to the coordinator's GitHub OAuth
// app on GitHub's device flow: they open GitHub's page in any browser and
// type the code shown, and this program never sees their password. It
// returns the token GitHub grants, which needs no scopes: the coordinator
// only reads who it belongs to.
func (c *Client) githubSignIn(ctx context.Context, clientID string) (string, error) {
	var code struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := c.githubForm(ctx, "/login/device/code", url.Values{"client_id": {clientID}}, &code); err != nil {
		return "", fmt.Errorf("starting GitHub sign-in: %w", err)
	}
	if code.DeviceCode == "" || code.UserCode == "" {
		return "", errors.New("GitHub did not start a sign-in: is the coordinator's OAuth app set up for the device flow?")
	}
	uri, userCode := printable(code.VerificationURI), printable(code.UserCode)
	fmt.Fprintf(c.Out, "\nSign in with GitHub to contribute: open %s and enter the code\n\n    %s\n\n", uri, userCode)
	c.event(Event{Stage: StageSignIn, Code: userCode, CodeURL: uri})
	// Whatever pacing comes back, within sane bounds (audit R-054): GitHub
	// asks for 5 s and gives 15 minutes.
	interval := time.Duration(min(max(code.Interval, 1), 60)) * time.Second
	deadline := time.Now().Add(time.Duration(min(max(code.ExpiresIn, 60), 1800)) * time.Second)
	for time.Now().Before(deadline) {
		if err := sleep(ctx, interval); err != nil {
			return "", err
		}
		var grant struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		err := c.githubForm(ctx, "/login/oauth/access_token", url.Values{
			"client_id":   {clientID},
			"device_code": {code.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}, &grant)
		if err != nil {
			return "", fmt.Errorf("finishing GitHub sign-in: %w", err)
		}
		switch grant.Error {
		case "":
			if grant.AccessToken == "" {
				return "", errors.New("GitHub granted no token")
			}
			fmt.Fprintln(c.Out, "Signed in with GitHub.")
			return grant.AccessToken, nil
		case "authorization_pending":
		case "slow_down":
			interval = min(interval+5*time.Second, time.Minute)
		case "access_denied":
			return "", errors.New("the GitHub sign-in was declined")
		case "expired_token":
			return "", errors.New("the GitHub code expired before the sign-in; run this again")
		default:
			return "", fmt.Errorf("GitHub sign-in failed: %s", grant.Error)
		}
	}
	return "", errors.New("the GitHub code expired before the sign-in; run this again")
}

// githubForm posts form to GitHub's path and decodes the JSON reply into v.
func (c *Client) githubForm(ctx context.Context, path string, form url.Values, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(c.GitHub, "/")+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub answered %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(v)
}
