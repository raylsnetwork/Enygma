package coordinator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// A participant of a GitHub phase signs in on GitHub's device flow from their
// own program, straight with GitHub, and passes the token on to register.
// The program refuses a token carrying any scope before it leaves the
// participant's machine (audit R-026), so a coordinator naming some other
// app, which the participant once gave more access, is handed nothing it can
// use. The coordinator, for its part, admits only tokens of its own OAuth app,
// as GitHub tells it (audit R-022), and relays nothing (audit R-023).

var (
	// errGitHubToken is a token GitHub does not accept, or not as one of
	// this coordinator's app.
	errGitHubToken = errors.New("GitHub did not accept the token")
	// errGitHubTokenForm is a token that cannot be one: not printable ASCII,
	// or too long (audit R-049).
	errGitHubTokenForm = errors.New("not a GitHub token")
	// errGitHubBusy is GitHub asked as often as githubLookups allows.
	errGitHubBusy = errors.New("too many GitHub sign-ins at once")
)

// githubLookups bounds how many registrations a minute the coordinator asks
// GitHub about, from everywhere: each asks GitHub twice, under the
// coordinator's app and address (audit R-023).
const githubLookups = 300

// githubAccount is who a GitHub token belongs to.
type githubAccount struct {
	Login   string    `json:"login"`
	ID      int64     `json:"id"`
	Created time.Time `json:"created_at"`
}

// githubClient asks GitHub's API, never for long.
var githubClient = &http.Client{Timeout: 15 * time.Second}

// githubUser asks GitHub whose token is token: one the participant's program
// was granted on GitHub's device flow for this coordinator's app. The token
// needs no scopes: an account's login, id and age are public.
func (s *Server) githubUser(ctx context.Context, token string) (githubAccount, error) {
	if len(token) == 0 || len(token) > 255 {
		return githubAccount{}, errGitHubTokenForm
	}
	for i := range len(token) {
		if token[i] < 0x21 || token[i] > 0x7e {
			return githubAccount{}, errGitHubTokenForm
		}
	}
	if !s.githubLookup(s.cfg.now()) {
		return githubAccount{}, errGitHubBusy
	}
	if err := s.githubOurs(ctx, token); err != nil {
		return githubAccount{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.GitHubAPI+"/user", nil)
	if err != nil {
		return githubAccount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	githubHeaders(req)
	resp, err := githubClient.Do(req)
	if err != nil {
		return githubAccount{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized: // a 403 is GitHub's rate limit, not the token
		return githubAccount{}, errGitHubToken
	default:
		return githubAccount{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var a githubAccount
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&a); err != nil {
		return githubAccount{}, err
	}
	// An answer missing a field would admit an account of no age, or many
	// accounts as github:0 (audit R-047); a login is shown, so it is a name.
	if a.ID <= 0 || a.Created.IsZero() {
		return githubAccount{}, fmt.Errorf("GitHub's answer has no account id or creation time: %+v", a)
	}
	if name, err := cleanName(a.Login); err != nil || name != a.Login {
		return githubAccount{}, fmt.Errorf("GitHub's answer has a login %q that is not a name", a.Login)
	}
	return a, nil
}

// githubOurs asks GitHub whether token is one of this coordinator's OAuth
// app, signed in as the app (GitHub's "check a token"). A token GET /user
// accepts may be any app's, such as one the participant's account gave
// another site, or another coordinator; only the app's own are admitted
// (audit R-022).
func (s *Server) githubOurs(ctx context.Context, token string) error {
	body, err := json.Marshal(map[string]string{"access_token": token})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.cfg.GitHubAPI+"/applications/"+url.PathEscape(s.cfg.GitHubClientID)+"/token", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.cfg.GitHubClientID, s.cfg.GitHubClientSecret)
	req.Header.Set("Content-Type", "application/json")
	githubHeaders(req)
	resp, err := githubClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound, http.StatusUnprocessableEntity: // not a token of this app
		return errGitHubToken
	case http.StatusUnauthorized:
		return errors.New("GitHub refused the app's client ID and secret: check -github-client-secret-file")
	default:
		return fmt.Errorf("GitHub answered %s checking the token", resp.Status)
	}
}

// githubHeaders asks for GitHub's API as this coordinator was written for.
func githubHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// githubLookup counts a registration's questions to GitHub now, unless
// githubLookups have been asked this minute.
func (s *Server) githubLookup(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if minute := now.Truncate(time.Minute); !minute.Equal(s.githubMinute) {
		s.githubMinute, s.githubAsked = minute, 0
	}
	if s.githubAsked >= githubLookups {
		return false
	}
	s.githubAsked++
	return true
}
