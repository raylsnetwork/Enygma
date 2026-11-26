package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// errGitHubToken is a token GitHub does not accept.
var errGitHubToken = errors.New("GitHub did not accept the token")

// githubAccount is who a GitHub token belongs to.
type githubAccount struct {
	Login   string    `json:"login"`
	ID      int64     `json:"id"`
	Created time.Time `json:"created_at"`
}

// githubClient asks GitHub's API, never for long.
var githubClient = &http.Client{Timeout: 15 * time.Second}

// githubUser asks GitHub whose token is token: the participant signed in
// on GitHub's device flow, and their program passed the token on. The token
// needs no scopes: an account's login, id and age are public.
func (s *Server) githubUser(ctx context.Context, token string) (githubAccount, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.GitHubAPI+"/user", nil)
	if err != nil {
		return githubAccount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
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
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&a)
	return a, err
}
