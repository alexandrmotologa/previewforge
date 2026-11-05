package webhook

import "encoding/json"

// PullRequestPayload represents GitHub's pull_request webhook JSON body.
type PullRequestPayload struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
	Sender      User        `json:"sender"`
}

// PullRequest contains details about the pull request.
type PullRequest struct {
	ID       int64   `json:"id"`
	Number   int     `json:"number"`
	State    string  `json:"state"`
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	HTMLURL  string  `json:"html_url"`
	User     User    `json:"user"`
	Head     PRHead  `json:"head"`
	Base     PRBase  `json:"base"`
	Labels   []Label `json:"labels"`
	Merged   bool    `json:"merged"`
	Draft    bool    `json:"draft"`
}

// PRHead contains details about the source branch and repository.
type PRHead struct {
	Ref  string     `json:"ref"`
	SHA  string     `json:"sha"`
	Repo Repository `json:"repo"`
}

// PRBase contains details about the destination branch.
type PRBase struct {
	Ref string `json:"ref"`
}

// Repository contains details about the GitHub repository.
type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	HTMLURL  string `json:"html_url"`
	Fork     bool   `json:"fork"`
	Owner    User   `json:"owner"`
}

// User represents a GitHub user account.
type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

// Label represents a GitHub label attached to an issue or pull request.
type Label struct {
	Name string `json:"name"`
}

// ParsePullRequestPayload decodes the raw JSON bytes into a PullRequestPayload.
func ParsePullRequestPayload(data []byte) (*PullRequestPayload, error) {
	var payload PullRequestPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// HasLabel checks if the pull request contains a specific label by name.
func (p *PullRequest) HasLabel(name string) bool {
	if name == "" {
		return true
	}
	for _, l := range p.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}
