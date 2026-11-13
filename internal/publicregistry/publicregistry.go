// Package publicregistry publishes podspecs to Zorvok's free public
// registry, zorvok-dev/zorvok-specs — a git repository, not a server
// (ADR 0009) — the way `pod trunk push` publishes to CocoaPods' Specs
// repo: only the podspec goes to the registry, as
// Specs/<name>/<version>/<name>.podspec.json; the pod's code, frameworks
// and xcframeworks stay wherever its `source` points.
//
// CocoaPods has a server, trunk.cocoapods.org, that holds write access to
// its Specs repo and commits on the publisher's behalf. The public
// registry has no server, so GitHub plays that part (ADR 0010): Publish
// commits the entry to a branch of the publisher's own fork and opens a
// pull request, and the registry's publish workflow — the only thing
// with write access to it — checks that pull request against the same
// rules a server would enforce, merges it itself, and comments a
// verdict, which WaitForVerdict reports back.
package publicregistry

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Repo is the public registry's GitHub repository.
const Repo = "zorvok-dev/zorvok-specs"

// URL is the registry's CocoaPods source URL: what a Podfile's `source`
// or `:source =>` and `zorvok install --native --source` point at.
const URL = "https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/"

// DefaultAPIURL is GitHub's REST API.
const DefaultAPIURL = "https://api.github.com"

// IsURL reports whether u is one of the addresses the public registry is
// reachable at — its raw-content URL (any branch), its GitHub Pages URL,
// or its repository page — so a registry_url set to any of them
// publishes here instead of being mistaken for a registry server.
func IsURL(u string) bool {
	parsed, err := url.Parse(strings.TrimSpace(u))
	if err != nil {
		return false
	}
	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	switch strings.ToLower(parsed.Host) {
	case "raw.githubusercontent.com":
		return path == "/zorvok-dev/zorvok-specs" || strings.HasPrefix(path, "/zorvok-dev/zorvok-specs/")
	case "zorvok-dev.github.io":
		return path == "/zorvok-specs" || strings.HasPrefix(path, "/zorvok-specs/")
	case "github.com", "www.github.com":
		return path == "/zorvok-dev/zorvok-specs" || path == "/zorvok-dev/zorvok-specs.git"
	}
	return false
}

// ErrNoGitHubLogin is returned by Token when there are no GitHub
// credentials to publish with.
var ErrNoGitHubLogin = errors.New("publishing to Zorvok's public registry uses your GitHub account: sign in once with `gh auth login` (install the GitHub CLI with `brew install gh`), or set GITHUB_TOKEN, then run this again")

// Token returns the GitHub token to publish with: GH_TOKEN, then
// GITHUB_TOKEN (the GitHub CLI's own precedence), then whatever `gh auth
// login` stored. Zorvok never stores it; it's sent only to the GitHub
// API.
func Token() (string, error) {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v, nil
		}
	}
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", ErrNoGitHubLogin
	}
	out, err := exec.Command(gh, "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return "", ErrNoGitHubLogin
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", ErrNoGitHubLogin
	}
	return token, nil
}

// Client publishes to the public registry through GitHub's REST API.
type Client struct {
	apiURL string
	token  string
	http   *http.Client

	// retryInterval and retryAttempts bound the wait for a fork GitHub
	// has only just started creating (about a minute); pollInterval
	// spaces out WaitForVerdict's checks. Tests shorten them.
	retryInterval time.Duration
	retryAttempts int
	pollInterval  time.Duration
}

// NewClient returns a Client that calls the GitHub API at apiURL with token.
func NewClient(apiURL, token string) *Client {
	return &Client{
		apiURL:        strings.TrimRight(apiURL, "/"),
		token:         token,
		http:          &http.Client{Timeout: 30 * time.Second},
		retryInterval: 2 * time.Second,
		retryAttempts: 30,
		pollInterval:  5 * time.Second,
	}
}

// APIError is a non-2xx response from the GitHub API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.StatusCode == http.StatusUnauthorized {
		return "GitHub rejected your token — sign in again with `gh auth login` (or check GITHUB_TOKEN)"
	}
	if e.Message != "" {
		return fmt.Sprintf("GitHub: %s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("GitHub: HTTP %d", e.StatusCode)
}

func isStatus(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == code
}

// do sends one API request, JSON-encoding body (if any) and decoding a
// 2xx response into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "zorvok")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("GitHub: reading the response to %s %s: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &apiErr)
		return &APIError{StatusCode: resp.StatusCode, Message: apiErr.Message}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("GitHub: unexpected response to %s %s: %w", method, path, err)
		}
	}
	return nil
}

// escapePath escapes each segment of a slash-separated path for use in a
// URL, keeping the slashes.
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// User is a GitHub account. The registry's OWNERS files identify owners
// by ID, which GitHub never reuses, rather than by username, which can be
// renamed and then claimed by someone else; the username sits next to it
// for whoever reads the file.
type User struct {
	ID    int64
	Login string
}

// OwnerLine is how an OWNERS file lists u.
func (u User) OwnerLine() string {
	return fmt.Sprintf("%d %s", u.ID, u.Login)
}

// User returns the GitHub account the token belongs to.
func (c *Client) User(ctx context.Context) (User, error) {
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.do(ctx, http.MethodGet, "/user", nil, &user); err != nil {
		return User{}, err
	}
	if user.ID <= 0 || user.Login == "" {
		return User{}, errors.New("GitHub didn't say which account your token belongs to")
	}
	return User{ID: user.ID, Login: user.Login}, nil
}

// Spec is the podspec being published: already linted and converted to
// JSON by internal/publisher.
type Spec struct {
	Name    string
	Version string
	JSON    string
	// Source summarizes where the pod's code comes from, for the pull
	// request's description.
	Source string
}

// Submission is the pull request a Publish opened or updated.
type Submission struct {
	Number int
	URL    string
	// HeadSHA is the commit the registry's publish workflow judges;
	// WaitForVerdict only trusts a verdict about this exact commit.
	HeadSHA string
	// NewPod is true when this publish also creates the pod's OWNERS file.
	NewPod bool
}

type gitRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type pullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Merged  bool   `json:"merged"`
}

// Publish adds spec to the public registry as user. It first checks the
// registry the way its publish workflow will, so a version that already
// exists or a pod user doesn't own fails right away rather than after a
// wait. Then it commits the podspec JSON — plus an OWNERS file listing
// user, for a brand-new pod — to a branch of user's fork, and opens a
// pull request, or updates the one an earlier attempt opened.
func (c *Client) Publish(ctx context.Context, user User, spec Spec) (*Submission, error) {
	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.do(ctx, http.MethodGet, "/repos/"+Repo, nil, &repo); err != nil {
		return nil, err
	}
	base := repo.DefaultBranch
	var baseRef gitRef
	if err := c.do(ctx, http.MethodGet, "/repos/"+Repo+"/git/ref/heads/"+escapePath(base), nil, &baseRef); err != nil {
		return nil, err
	}
	baseSHA := baseRef.Object.SHA

	specPath := fmt.Sprintf("Specs/%s/%s/%s.podspec.json", spec.Name, spec.Version, spec.Name)
	ownersPath := fmt.Sprintf("Specs/%s/OWNERS", spec.Name)

	_, published, err := c.file(ctx, Repo, specPath, baseSHA)
	if err != nil {
		return nil, err
	}
	if published {
		return nil, fmt.Errorf("%s %s is already published, and a published version can never change — bump the version in your podspec and push again", spec.Name, spec.Version)
	}
	owners, ownersExist, err := c.file(ctx, Repo, ownersPath, baseSHA)
	if err != nil {
		return nil, err
	}
	if ownersExist && !listsOwner(owners, user.ID) {
		return nil, fmt.Errorf("%s is owned by %s on Zorvok's public registry, and only its owners can publish new versions — ask one of them to add this line, your GitHub user ID and username, to %s:\n  %s", spec.Name, strings.Join(ownerList(owners), ", "), ownersPath, user.OwnerLine())
	}

	fork, err := c.fork(ctx)
	if err != nil {
		return nil, err
	}
	baseTree, err := c.commitTree(ctx, fork.FullName, baseSHA, base)
	if err != nil {
		return nil, err
	}

	// Everything goes in one commit on top of the registry's tip, and
	// the branch moves to it in one step: the publish workflow then runs
	// once, on the finished change, and the branch never points at a
	// commit the registry already has (which GitHub can treat as an open
	// pull request from it having been merged).
	files := map[string]string{specPath: spec.JSON}
	if !ownersExist {
		files[ownersPath] = user.OwnerLine() + "\n"
	}
	message := fmt.Sprintf("Publish %s %s", spec.Name, spec.Version)
	headSHA, err := c.commit(ctx, fork.FullName, baseTree, baseSHA, files, message)
	if err != nil {
		return nil, err
	}
	branch := "zorvok-publish/" + refSafe(spec.Name) + "-" + refSafe(spec.Version)
	if err := c.setBranch(ctx, fork.FullName, branch, headSHA); err != nil {
		return nil, err
	}

	pr, err := c.openPullRequest(ctx, fork.Owner.Login+":"+branch, base, message, pullRequestBody(spec, user.Login, !ownersExist))
	if err != nil {
		return nil, err
	}
	return &Submission{Number: pr.Number, URL: pr.HTMLURL, HeadSHA: headSHA, NewPod: !ownersExist}, nil
}

// file returns path's contents in repo at ref, and whether it exists there.
func (c *Client) file(ctx context.Context, repo, path, ref string) (string, bool, error) {
	var f struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	err := c.do(ctx, http.MethodGet, "/repos/"+repo+"/contents/"+escapePath(path)+"?ref="+url.QueryEscape(ref), nil, &f)
	if isStatus(err, http.StatusNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if f.Encoding != "base64" {
		return "", true, fmt.Errorf("GitHub returned %s in an unexpected encoding (%q)", path, f.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
	if err != nil {
		return "", true, fmt.Errorf("decoding %s from GitHub: %w", path, err)
	}
	return string(b), true, nil
}

// ownerFields splits an OWNERS file into its lines' fields, "<GitHub
// user ID> <username>", leaving out # comments and blank lines.
func ownerFields(owners string) [][]string {
	var lines [][]string
	for _, line := range strings.Split(owners, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if fields := strings.Fields(line); len(fields) > 0 {
			lines = append(lines, fields)
		}
	}
	return lines
}

// listsOwner mirrors the publish workflow's own ownership check
// (owner_ids in zorvok-specs' validate-pr.sh): some line's first field
// must be id, exactly.
func listsOwner(owners string, id int64) bool {
	want := strconv.FormatInt(id, 10)
	for _, fields := range ownerFields(owners) {
		if fields[0] == want {
			return true
		}
	}
	return false
}

// ownerList names an OWNERS file's owners by username, or by ID for a
// line without one.
func ownerList(owners string) []string {
	var list []string
	for _, fields := range ownerFields(owners) {
		list = append(list, fields[len(fields)-1])
	}
	return list
}

var refUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// refSafe makes s usable as part of a git branch name.
func refSafe(s string) string {
	return strings.ReplaceAll(refUnsafe.ReplaceAllString(s, "-"), "..", "-")
}

type forkInfo struct {
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// fork returns the publisher's fork of the registry, creating it the
// first time; GitHub returns the existing fork when there already is one.
func (c *Client) fork(ctx context.Context) (*forkInfo, error) {
	var fork forkInfo
	if err := c.do(ctx, http.MethodPost, "/repos/"+Repo+"/forks", map[string]bool{"default_branch_only": true}, &fork); err != nil {
		return nil, fmt.Errorf("couldn't fork %s to your account: %w", Repo, err)
	}
	if fork.FullName == "" || fork.Owner.Login == "" {
		return nil, fmt.Errorf("GitHub didn't return your fork of %s", Repo)
	}
	return &fork, nil
}

// commitTree returns the tree of commit sha as fork sees it. It retries
// while a fork GitHub has only just started creating isn't ready yet
// (404, or 409 "Git Repository is empty"), and syncs an older fork that
// doesn't have the registry's latest commit yet.
func (c *Client) commitTree(ctx context.Context, fork, sha, base string) (string, error) {
	synced := false
	var lastErr error
	for attempt := 0; attempt < c.retryAttempts; attempt++ {
		var commit struct {
			Tree struct {
				SHA string `json:"sha"`
			} `json:"tree"`
		}
		err := c.do(ctx, http.MethodGet, "/repos/"+fork+"/git/commits/"+sha, nil, &commit)
		if err == nil {
			return commit.Tree.SHA, nil
		}
		if !isStatus(err, http.StatusNotFound) && !isStatus(err, http.StatusConflict) && !isStatus(err, http.StatusUnprocessableEntity) {
			return "", err
		}
		if !synced {
			synced = true
			_ = c.do(ctx, http.MethodPost, "/repos/"+fork+"/merge-upstream", map[string]string{"branch": base}, nil)
		}
		lastErr = err
		if err := sleep(ctx, c.retryInterval); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("your fork %s isn't ready, or is out of date (sync it with %s on GitHub and try again): %w", fork, Repo, lastErr)
}

// commit creates one commit on fork, on top of parent, that adds files
// (path → contents) to parent's tree, and returns its SHA.
func (c *Client) commit(ctx context.Context, fork, parentTree, parent string, files map[string]string, message string) (string, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	entries := make([]map[string]string, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, map[string]string{"path": p, "mode": "100644", "type": "blob", "content": files[p]})
	}

	var tree, commit struct {
		SHA string `json:"sha"`
	}
	if err := c.do(ctx, http.MethodPost, "/repos/"+fork+"/git/trees", map[string]any{"base_tree": parentTree, "tree": entries}, &tree); err != nil {
		return "", fmt.Errorf("couldn't prepare the commit on your fork: %w", err)
	}
	if err := c.do(ctx, http.MethodPost, "/repos/"+fork+"/git/commits", map[string]any{"message": message, "tree": tree.SHA, "parents": []string{parent}}, &commit); err != nil {
		return "", fmt.Errorf("couldn't commit to your fork: %w", err)
	}
	return commit.SHA, nil
}

// setBranch points branch on fork at sha, creating it, or moving the one
// an earlier attempt left behind.
func (c *Client) setBranch(ctx context.Context, fork, branch, sha string) error {
	err := c.do(ctx, http.MethodPost, "/repos/"+fork+"/git/refs", map[string]string{"ref": "refs/heads/" + branch, "sha": sha}, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity && strings.Contains(apiErr.Message, "Reference already exists") {
		err = c.do(ctx, http.MethodPatch, "/repos/"+fork+"/git/refs/heads/"+escapePath(branch), map[string]any{"sha": sha, "force": true}, nil)
	}
	if err != nil {
		return fmt.Errorf("couldn't update the branch on your fork %s: %w", fork, err)
	}
	return nil
}

// openPullRequest opens head → base on the registry, or returns the pull
// request an earlier attempt already opened from the same branch (which
// now carries this attempt's commit, since setBranch moved it).
func (c *Client) openPullRequest(ctx context.Context, head, base, title, body string) (*pullRequest, error) {
	var pr pullRequest
	err := c.do(ctx, http.MethodPost, "/repos/"+Repo+"/pulls", map[string]any{
		"title":                 title,
		"head":                  head,
		"base":                  base,
		"body":                  body,
		"maintainer_can_modify": false,
	}, &pr)
	if err == nil {
		return &pr, nil
	}
	if !isStatus(err, http.StatusUnprocessableEntity) {
		return nil, fmt.Errorf("couldn't open the pull request: %w", err)
	}
	var open []pullRequest
	if lookupErr := c.do(ctx, http.MethodGet, "/repos/"+Repo+"/pulls?state=open&head="+url.QueryEscape(head), nil, &open); lookupErr != nil || len(open) == 0 {
		return nil, fmt.Errorf("couldn't open the pull request: %w", err)
	}
	return &open[0], nil
}

func pullRequestBody(spec Spec, login string, newPod bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Publishes **%s %s** to Zorvok's public registry, from `zorvok trunk push`.\n\n", spec.Name, spec.Version)
	if spec.Source != "" {
		fmt.Fprintf(&b, "Source: `%s`\n\n", spec.Source)
	}
	if newPod {
		fmt.Fprintf(&b, "This is %s's first version here, so it also adds `Specs/%s/OWNERS` listing @%s.\n\n", spec.Name, spec.Name, login)
	}
	b.WriteString("The registry's publish check verifies ownership and that no published version changes, then merges this automatically.\n")
	return b.String()
}

// Outcome is what the registry's publish workflow decided.
type Outcome string

const (
	Published   Outcome = "published"
	NeedsReview Outcome = "needs-review"
	Rejected    Outcome = "rejected"
	// Pending means no verdict arrived before the wait ran out.
	Pending Outcome = ""
)

// Verdict is the publish workflow's decision about one submission.
type Verdict struct {
	Outcome Outcome
	// Details is the workflow's explanation: why it needs a review, or
	// what it rejected.
	Details string
}

// verdictMarker is the machine-readable first line of the comment the
// registry's publish workflow (zorvok-specs
// .github/workflows/publish.yml) leaves on every pull request it judges.
var verdictMarker = regexp.MustCompile(`<!-- zorvok-publish sha=([0-9a-f]{40}) verdict=(published|needs-review|rejected) -->`)

// botLogin is the account GitHub Actions comments as. Only its comments
// count as verdicts, so nobody else can post one on a publisher's behalf.
const botLogin = "github-actions[bot]"

// WaitForVerdict polls sub's pull request until the registry's publish
// workflow posts its verdict on sub.HeadSHA, or timeout passes (Pending).
func (c *Client) WaitForVerdict(ctx context.Context, sub *Submission, timeout time.Duration) (Verdict, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		v, err := c.verdict(ctx, sub)
		if err != nil {
			if ctx.Err() != nil {
				return Verdict{Outcome: Pending}, nil
			}
			return Verdict{}, err
		}
		if v.Outcome != Pending {
			return v, nil
		}
		if err := sleep(ctx, c.pollInterval); err != nil {
			return Verdict{Outcome: Pending}, nil
		}
	}
}

func (c *Client) verdict(ctx context.Context, sub *Submission) (Verdict, error) {
	var found *Verdict
	for page := 1; ; page++ {
		var comments []struct {
			Body string `json:"body"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100&page=%d", Repo, sub.Number, page), nil, &comments); err != nil {
			return Verdict{}, err
		}
		for _, comment := range comments {
			m := verdictMarker.FindStringSubmatch(comment.Body)
			if m == nil || comment.User.Login != botLogin || m[1] != sub.HeadSHA {
				continue
			}
			found = &Verdict{
				Outcome: Outcome(m[2]),
				Details: verdictDetails(strings.Replace(comment.Body, m[0], "", 1)),
			}
		}
		if len(comments) < 100 {
			break
		}
	}
	if found != nil {
		return *found, nil
	}

	// No verdict comment yet — but a pull request a maintainer merged by
	// hand is published all the same.
	var pr pullRequest
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", Repo, sub.Number), nil, &pr); err != nil {
		return Verdict{}, err
	}
	switch {
	case pr.Merged:
		return Verdict{Outcome: Published}, nil
	case pr.State == "closed":
		return Verdict{Outcome: Rejected, Details: "The pull request was closed without being merged."}, nil
	}
	return Verdict{Outcome: Pending}, nil
}

// rejectedHeading opens the publish workflow's comment on a rejected
// pull request, above its reasons.
const rejectedHeading = "This can't be published:"

// verdictDetails turns the text of a verdict comment, after its marker,
// into something safe to print in a terminal. The reasons can quote a
// pull request or CocoaPods trunk, so control characters (which could
// drive the terminal) and bidirectional-text controls (which could make
// them read differently from what they say) are dropped. The workflow's
// heading and the indent it quotes reasons with are removed too, since
// `trunk push` prints its own.
func verdictDetails(text string) string {
	text = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)) {
			return -1
		}
		return r
	}, text)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if lines[0] == rejectedHeading {
		lines = lines[1:]
	}
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, "    ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
