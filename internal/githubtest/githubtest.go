// Package githubtest is an in-memory stand-in for the slice of GitHub's
// REST API that internal/publicregistry uses — the public registry's
// repository, the publisher's fork of it, and pull requests between them
// — so publishing can be tested with no network and no real account.
package githubtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const upstream = "zorvok-dev/zorvok-specs"

// BaseSHA is the commit the registry's default branch points at, and
// BaseTree is that commit's tree.
const (
	BaseSHA  = "b000000000000000000000000000000000000000"
	BaseTree = "a000000000000000000000000000000000000000"
)

// BotLogin is the account the registry's publish workflow comments as.
const BotLogin = "github-actions[bot]"

// UserID is the GitHub user ID of the account every Fake is signed in as.
const UserID = 1001

// Fake is one fake GitHub, on which Login (GitHub user UserID) is signed
// in.
type Fake struct {
	// URL is the fake API's base URL; Token is the only token it accepts.
	URL   string
	Token string
	Login string

	mu              sync.Mutex
	upstream        map[string]string            // the registry's default branch: path → contents
	trees           map[string]map[string]string // tree SHA → files
	commits         map[string]string            // commit SHA → tree SHA
	branches        map[string]string            // fork branch → head commit SHA
	forked          bool
	forkNotReadyFor int
	verdict         string
	verdictDetails  string
	pulls           []*PullRequest
	comments        map[int][]Comment
	lastID          int
}

// PullRequest is a pull request against the fake registry.
type PullRequest struct {
	Number int
	Head   string // "<fork owner>:<branch>"
	Base   string
	Title  string
	Body   string
	State  string // "open" or "closed"
	Merged bool
}

// Comment is a comment on a pull request.
type Comment struct {
	User string
	Body string
}

// New starts a fake GitHub for login; it shuts down when the test ends.
func New(t *testing.T, login string) *Fake {
	t.Helper()
	f := &Fake{
		Token:    "test-token",
		Login:    login,
		upstream: map[string]string{},
		commits:  map[string]string{BaseSHA: BaseTree},
		branches: map[string]string{},
		comments: map[int][]Comment{},
	}
	f.trees = map[string]map[string]string{BaseTree: f.upstream}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

// VerdictComment is a comment in the format the registry's publish
// workflow posts.
func VerdictComment(sha, verdict, details string) string {
	return fmt.Sprintf("<!-- zorvok-publish sha=%s verdict=%s -->\n%s", sha, verdict, details)
}

// SetUpstreamFile puts a file on the registry's default branch.
func (f *Fake) SetUpstreamFile(path, contents string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upstream[path] = contents
}

// DelayFork makes the fork answer 404 to its first n commit reads, the
// way a fork GitHub is still creating does.
func (f *Fake) DelayFork(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forkNotReadyFor = n
}

// ActAsPublishWorkflow makes the fake judge every new head of an open
// pull request the way the registry's publish workflow does: comment
// verdict (with details) on it, and merge it if verdict is "published".
func (f *Fake) ActAsPublishWorkflow(verdict, details string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verdict, f.verdictDetails = verdict, details
}

// SeedBranch leaves a branch on the fork, with files added on top of the
// registry's tip, the way an earlier attempt would have; it returns the
// branch's head commit.
func (f *Fake) SeedBranch(branch string, files map[string]string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forked = true
	tree := copyFiles(f.upstream)
	for p, c := range files {
		tree[p] = c
	}
	treeSHA := f.newID()
	f.trees[treeSHA] = tree
	head := f.newID()
	f.commits[head] = treeSHA
	f.branches[branch] = head
	return head
}

// OpenPull opens a pull request from the fork's branch, the way an
// earlier attempt would have, and returns its number.
func (f *Fake) OpenPull(branch string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr := &PullRequest{Number: len(f.pulls) + 1, Head: f.Login + ":" + branch, Base: "main", Title: "earlier attempt", State: "open"}
	f.pulls = append(f.pulls, pr)
	return pr.Number
}

// AddComment adds a comment to pull request number.
func (f *Fake) AddComment(number int, user, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments[number] = append(f.comments[number], Comment{User: user, Body: body})
}

// MergePull marks pull request number merged, as a maintainer would.
func (f *Fake) MergePull(number int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pr := f.pull(number); pr != nil {
		pr.Merged, pr.State = true, "closed"
	}
}

// Forked reports whether the publisher's fork exists.
func (f *Fake) Forked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.forked
}

// BranchHead returns the head commit of the fork's branch ("" if none).
func (f *Fake) BranchHead(branch string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.branches[branch]
}

// BranchFiles returns every file at the head of the fork's branch.
func (f *Fake) BranchFiles(branch string) (map[string]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	head, ok := f.branches[branch]
	if !ok {
		return nil, false
	}
	return copyFiles(f.trees[f.commits[head]]), true
}

// Pulls returns copies of every pull request opened so far.
func (f *Fake) Pulls() []PullRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := make([]PullRequest, 0, len(f.pulls))
	for _, pr := range f.pulls {
		list = append(list, *pr)
	}
	return list
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+f.Token {
		reply(w, http.StatusUnauthorized, message("Bad credentials"))
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	up := "/repos/" + upstream
	fork := "/repos/" + f.Login + "/zorvok-specs"
	path, q := r.URL.Path, r.URL.Query()

	switch {
	case r.Method == http.MethodGet && path == "/user":
		reply(w, http.StatusOK, map[string]any{"login": f.Login, "id": UserID})

	case r.Method == http.MethodGet && path == up:
		reply(w, http.StatusOK, map[string]string{"full_name": upstream, "default_branch": "main"})

	case r.Method == http.MethodGet && path == up+"/git/ref/heads/main":
		reply(w, http.StatusOK, map[string]any{"object": map[string]string{"sha": BaseSHA}})

	case r.Method == http.MethodGet && strings.HasPrefix(path, up+"/contents/"):
		contents, ok := f.trees[f.commits[q.Get("ref")]][strings.TrimPrefix(path, up+"/contents/")]
		if !ok {
			reply(w, http.StatusNotFound, message("Not Found"))
			return
		}
		reply(w, http.StatusOK, map[string]string{"encoding": "base64", "content": wrap60(base64.StdEncoding.EncodeToString([]byte(contents)))})

	case r.Method == http.MethodPost && path == up+"/forks":
		f.forked = true
		reply(w, http.StatusAccepted, map[string]any{"full_name": f.Login + "/zorvok-specs", "owner": map[string]string{"login": f.Login}})

	case r.Method == http.MethodPost && path == fork+"/merge-upstream":
		reply(w, http.StatusOK, message("Successfully fetched and fast-forwarded from upstream"))

	case r.Method == http.MethodGet && strings.HasPrefix(path, fork+"/git/commits/"):
		if !f.forked || f.forkNotReadyFor > 0 {
			if f.forkNotReadyFor > 0 {
				f.forkNotReadyFor--
			}
			reply(w, http.StatusNotFound, message("Not Found"))
			return
		}
		sha := strings.TrimPrefix(path, fork+"/git/commits/")
		tree, ok := f.commits[sha]
		if !ok {
			reply(w, http.StatusNotFound, message("No commit found for SHA: "+sha))
			return
		}
		reply(w, http.StatusOK, map[string]any{"sha": sha, "tree": map[string]string{"sha": tree}})

	case r.Method == http.MethodPost && path == fork+"/git/trees":
		var req struct {
			BaseTree string `json:"base_tree"`
			Tree     []struct {
				Path, Mode, Type, Content string
			} `json:"tree"`
		}
		if !decode(w, r, &req) {
			return
		}
		base, ok := f.trees[req.BaseTree]
		if !ok {
			reply(w, http.StatusUnprocessableEntity, message("base_tree is not a valid tree oid"))
			return
		}
		files := copyFiles(base)
		for _, e := range req.Tree {
			if e.Mode != "100644" || e.Type != "blob" {
				reply(w, http.StatusUnprocessableEntity, message("unexpected tree entry"))
				return
			}
			files[e.Path] = e.Content
		}
		sha := f.newID()
		f.trees[sha] = files
		reply(w, http.StatusCreated, map[string]string{"sha": sha})

	case r.Method == http.MethodPost && path == fork+"/git/commits":
		var req struct {
			Message, Tree string
			Parents       []string
		}
		if !decode(w, r, &req) {
			return
		}
		if _, ok := f.trees[req.Tree]; !ok || len(req.Parents) != 1 || f.commits[req.Parents[0]] == "" {
			reply(w, http.StatusUnprocessableEntity, message("Tree SHA does not exist"))
			return
		}
		sha := f.newID()
		f.commits[sha] = req.Tree
		reply(w, http.StatusCreated, map[string]string{"sha": sha})

	case r.Method == http.MethodPost && path == fork+"/git/refs":
		var req struct{ Ref, SHA string }
		if !decode(w, r, &req) {
			return
		}
		branch := strings.TrimPrefix(req.Ref, "refs/heads/")
		if _, exists := f.branches[branch]; exists {
			reply(w, http.StatusUnprocessableEntity, message("Reference already exists"))
			return
		}
		if _, ok := f.commits[req.SHA]; !ok {
			reply(w, http.StatusUnprocessableEntity, message("Object does not exist"))
			return
		}
		f.branches[branch] = req.SHA
		f.headMoved(branch)
		reply(w, http.StatusCreated, map[string]any{"ref": req.Ref, "object": map[string]string{"sha": req.SHA}})

	case r.Method == http.MethodPatch && strings.HasPrefix(path, fork+"/git/refs/heads/"):
		branch := strings.TrimPrefix(path, fork+"/git/refs/heads/")
		var req struct {
			SHA   string
			Force bool
		}
		if !decode(w, r, &req) {
			return
		}
		if _, exists := f.branches[branch]; !exists {
			reply(w, http.StatusUnprocessableEntity, message("Reference does not exist"))
			return
		}
		if !req.Force {
			reply(w, http.StatusUnprocessableEntity, message("Update is not a fast forward"))
			return
		}
		f.branches[branch] = req.SHA
		f.headMoved(branch)
		reply(w, http.StatusOK, map[string]any{"object": map[string]string{"sha": req.SHA}})

	case r.Method == http.MethodPost && path == up+"/pulls":
		var req struct{ Title, Head, Base, Body string }
		if !decode(w, r, &req) {
			return
		}
		for _, pr := range f.pulls {
			if pr.Head == req.Head && pr.State == "open" {
				reply(w, http.StatusUnprocessableEntity, message("Validation Failed"))
				return
			}
		}
		pr := &PullRequest{Number: len(f.pulls) + 1, Head: req.Head, Base: req.Base, Title: req.Title, Body: req.Body, State: "open"}
		f.pulls = append(f.pulls, pr)
		f.judge(pr)
		reply(w, http.StatusCreated, pullJSON(pr))

	case r.Method == http.MethodGet && path == up+"/pulls":
		list := []map[string]any{}
		for _, pr := range f.pulls {
			if (q.Get("head") == "" || pr.Head == q.Get("head")) && (q.Get("state") == "" || pr.State == q.Get("state")) {
				list = append(list, pullJSON(pr))
			}
		}
		reply(w, http.StatusOK, list)

	case r.Method == http.MethodGet && strings.HasPrefix(path, up+"/pulls/"):
		n, _ := strconv.Atoi(strings.TrimPrefix(path, up+"/pulls/"))
		pr := f.pull(n)
		if pr == nil {
			reply(w, http.StatusNotFound, message("Not Found"))
			return
		}
		reply(w, http.StatusOK, pullJSON(pr))

	case r.Method == http.MethodGet && strings.HasPrefix(path, up+"/issues/") && strings.HasSuffix(path, "/comments"):
		n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(path, up+"/issues/"), "/comments"))
		list := []map[string]any{}
		if page := q.Get("page"); page == "" || page == "1" {
			for _, c := range f.comments[n] {
				list = append(list, map[string]any{"body": c.Body, "user": map[string]string{"login": c.User}})
			}
		}
		reply(w, http.StatusOK, list)

	default:
		reply(w, http.StatusNotFound, message("fake GitHub has no route for "+r.Method+" "+path))
	}
}

// headMoved judges every open pull request from branch, as a push to it
// would trigger the publish workflow on each.
func (f *Fake) headMoved(branch string) {
	for _, pr := range f.pulls {
		if pr.Head == f.Login+":"+branch && pr.State == "open" {
			f.judge(pr)
		}
	}
}

func (f *Fake) judge(pr *PullRequest) {
	if f.verdict == "" {
		return
	}
	head, ok := f.branches[strings.TrimPrefix(pr.Head, f.Login+":")]
	if !ok {
		return
	}
	f.comments[pr.Number] = append(f.comments[pr.Number], Comment{User: BotLogin, Body: VerdictComment(head, f.verdict, f.verdictDetails)})
	if f.verdict == "published" {
		pr.Merged, pr.State = true, "closed"
	}
}

func (f *Fake) pull(number int) *PullRequest {
	for _, pr := range f.pulls {
		if pr.Number == number {
			return pr
		}
	}
	return nil
}

func (f *Fake) newID() string {
	f.lastID++
	return fmt.Sprintf("%040x", f.lastID)
}

func pullJSON(pr *PullRequest) map[string]any {
	return map[string]any{
		"number":   pr.Number,
		"html_url": fmt.Sprintf("https://github.com/%s/pull/%d", upstream, pr.Number),
		"state":    pr.State,
		"merged":   pr.Merged,
	}
}

func copyFiles(files map[string]string) map[string]string {
	c := make(map[string]string, len(files))
	for p, contents := range files {
		c[p] = contents
	}
	return c
}

// wrap60 breaks base64 into 60-character lines, as GitHub's contents API does.
func wrap60(s string) string {
	var b strings.Builder
	for len(s) > 60 {
		b.WriteString(s[:60] + "\n")
		s = s[60:]
	}
	b.WriteString(s)
	return b.String()
}

func message(m string) map[string]string {
	return map[string]string{"message": m}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		reply(w, http.StatusBadRequest, message("Problems parsing JSON"))
		return false
	}
	return true
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
