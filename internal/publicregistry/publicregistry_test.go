package publicregistry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zorvok-dev/zorvok-ios/internal/githubtest"
)

const branch = "zorvok-publish/MyPod-1.0.0"

// alice is the account every test's fake GitHub is signed in as.
var alice = User{ID: githubtest.UserID, Login: "alice"}

func newTestClient(f *githubtest.Fake) *Client {
	c := NewClient(f.URL, f.Token)
	c.retryInterval = time.Millisecond
	c.pollInterval = time.Millisecond
	return c
}

func myPod(version string) Spec {
	return Spec{
		Name:    "MyPod",
		Version: version,
		JSON:    `{"name": "MyPod", "version": "` + version + `", "source": {"git": "https://github.com/alice/MyPod.git", "tag": "` + version + `"}}` + "\n",
		Source:  `{"git":"https://github.com/alice/MyPod.git","tag":"` + version + `"}`,
	}
}

// TestPublishNewPodAddsSpecAndOwnersInOneCommit covers a pod's first
// publish: the podspec JSON lands verbatim at the path the registry's
// checks expect, OWNERS lists the publisher's GitHub user ID and login, both
// arrive in a single commit on the publisher's fork, and the pull
// request targets the registry's default branch.
func TestPublishNewPodAddsSpecAndOwnersInOneCommit(t *testing.T) {
	f := githubtest.New(t, "alice")
	spec := myPod("1.0.0")

	sub, err := newTestClient(f).Publish(context.Background(), alice, spec)
	if err != nil {
		t.Fatal(err)
	}

	files, ok := f.BranchFiles(branch)
	if !ok {
		t.Fatalf("no %s branch on the fork", branch)
	}
	if got := files["Specs/MyPod/1.0.0/MyPod.podspec.json"]; got != spec.JSON {
		t.Errorf("podspec JSON = %q, want it verbatim: %q", got, spec.JSON)
	}
	if got := files["Specs/MyPod/OWNERS"]; got != "1001 alice\n" {
		t.Errorf("OWNERS = %q, want %q", got, "1001 alice\n")
	}
	if len(files) != 2 {
		t.Errorf("branch has %d files, want only the podspec and OWNERS: %v", len(files), files)
	}
	if !sub.NewPod || sub.HeadSHA != f.BranchHead(branch) {
		t.Errorf("submission = %+v, want NewPod and HeadSHA = the branch head %s", sub, f.BranchHead(branch))
	}

	pulls := f.Pulls()
	if len(pulls) != 1 {
		t.Fatalf("%d pull requests, want 1", len(pulls))
	}
	pr := pulls[0]
	if pr.Head != "alice:"+branch || pr.Base != "main" || pr.Title != "Publish MyPod 1.0.0" {
		t.Errorf("pull request = %+v", pr)
	}
	if !strings.Contains(pr.Body, "Specs/MyPod/OWNERS") || !strings.Contains(pr.Body, "@alice") {
		t.Errorf("pull request body doesn't mention the new OWNERS file: %q", pr.Body)
	}
	if sub.URL != "https://github.com/zorvok-dev/zorvok-specs/pull/1" {
		t.Errorf("submission URL = %q", sub.URL)
	}
}

// TestPublishNewVersionByAnOwnerAddsOnlyTheSpec covers every publish
// after the first: OWNERS already exists and lists the publisher, so
// only the new version's podspec is added and OWNERS is left alone.
func TestPublishNewVersionByAnOwnerAddsOnlyTheSpec(t *testing.T) {
	f := githubtest.New(t, "alice")
	f.SetUpstreamFile("Specs/MyPod/OWNERS", "# maintainers\n1002 bob\n1001 alice\n")
	f.SetUpstreamFile("Specs/MyPod/1.0.0/MyPod.podspec.json", `{"name": "MyPod", "version": "1.0.0"}`)
	spec := myPod("1.1.0")

	sub, err := newTestClient(f).Publish(context.Background(), alice, spec)
	if err != nil {
		t.Fatal(err)
	}

	files, _ := f.BranchFiles("zorvok-publish/MyPod-1.1.0")
	if got := files["Specs/MyPod/1.1.0/MyPod.podspec.json"]; got != spec.JSON {
		t.Errorf("podspec JSON = %q, want %q", got, spec.JSON)
	}
	if got := files["Specs/MyPod/OWNERS"]; got != "# maintainers\n1002 bob\n1001 alice\n" {
		t.Errorf("OWNERS = %q, want it unchanged", got)
	}
	if sub.NewPod {
		t.Error("NewPod = true for a pod that already has owners")
	}
	if body := f.Pulls()[0].Body; strings.Contains(body, "OWNERS") {
		t.Errorf("pull request body claims to add OWNERS: %q", body)
	}
}

// TestPublishRefusesSomeoneWhoDoesNotOwnThePod proves the ownership rule
// is checked before anything is created on GitHub — no fork, no pull
// request — and that the error says who does own it and how to get added.
func TestPublishRefusesSomeoneWhoDoesNotOwnThePod(t *testing.T) {
	f := githubtest.New(t, "alice")
	// "alice" as bob's username, or alice's ID in a comment, isn't alice.
	f.SetUpstreamFile("Specs/MyPod/OWNERS", "1002 alice # was bob, renamed\n# 1001\n")
	f.SetUpstreamFile("Specs/MyPod/1.0.0/MyPod.podspec.json", `{"name": "MyPod", "version": "1.0.0"}`)

	_, err := newTestClient(f).Publish(context.Background(), alice, myPod("1.1.0"))
	if err == nil {
		t.Fatal("publish succeeded for someone who isn't listed in OWNERS")
	}
	for _, want := range []string{"owned by alice on", "1001 alice", "Specs/MyPod/OWNERS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q doesn't mention %q", err, want)
		}
	}
	if f.Forked() || len(f.Pulls()) != 0 {
		t.Error("a refused publish still forked the registry or opened a pull request")
	}
}

// TestPublishRefusesAVersionThatIsAlreadyPublished proves published
// versions are immutable from the client's side too.
func TestPublishRefusesAVersionThatIsAlreadyPublished(t *testing.T) {
	f := githubtest.New(t, "alice")
	f.SetUpstreamFile("Specs/MyPod/OWNERS", "1001 alice\n")
	f.SetUpstreamFile("Specs/MyPod/1.0.0/MyPod.podspec.json", `{"name": "MyPod", "version": "1.0.0"}`)

	_, err := newTestClient(f).Publish(context.Background(), alice, myPod("1.0.0"))
	if err == nil || !strings.Contains(err.Error(), "already published") {
		t.Fatalf("err = %v, want an already-published error", err)
	}
	if f.Forked() {
		t.Error("a refused publish still forked the registry")
	}
}

// TestPublishWaitsForANewForkToBeReady covers a first-time publisher:
// GitHub creates forks asynchronously and answers 404 until it's done.
func TestPublishWaitsForANewForkToBeReady(t *testing.T) {
	f := githubtest.New(t, "alice")
	f.DelayFork(3)

	if _, err := newTestClient(f).Publish(context.Background(), alice, myPod("1.0.0")); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.BranchFiles(branch); !ok {
		t.Fatal("nothing was committed once the fork was ready")
	}
}

// TestPublishUpdatesTheBranchAndPullRequestOfAnEarlierAttempt covers
// running `trunk push` again after a rejected attempt: the same branch
// moves to a fresh commit (no leftovers from the broken attempt) and the
// pull request already open from it is reused, not duplicated.
func TestPublishUpdatesTheBranchAndPullRequestOfAnEarlierAttempt(t *testing.T) {
	f := githubtest.New(t, "alice")
	oldHead := f.SeedBranch(branch, map[string]string{
		"Specs/MyPod/1.0.0/MyPod.podspec.json": `{"broken": true}`,
		"Specs/MyPod/OWNERS":                   "1001 alice\n",
		"Specs/MyPod/1.0.0/leftover.txt":       "from the earlier attempt",
	})
	number := f.OpenPull(branch)
	spec := myPod("1.0.0")

	sub, err := newTestClient(f).Publish(context.Background(), alice, spec)
	if err != nil {
		t.Fatal(err)
	}

	if sub.Number != number || len(f.Pulls()) != 1 {
		t.Errorf("submission is pull request #%d with %d open, want the existing #%d reused", sub.Number, len(f.Pulls()), number)
	}
	if head := f.BranchHead(branch); head == oldHead || head != sub.HeadSHA {
		t.Errorf("branch head = %s, want the new commit %s (not the earlier attempt's %s)", head, sub.HeadSHA, oldHead)
	}
	files, _ := f.BranchFiles(branch)
	if files["Specs/MyPod/1.0.0/MyPod.podspec.json"] != spec.JSON {
		t.Errorf("podspec JSON = %q, want this attempt's", files["Specs/MyPod/1.0.0/MyPod.podspec.json"])
	}
	if _, ok := files["Specs/MyPod/1.0.0/leftover.txt"]; ok {
		t.Error("a file from the earlier attempt survived on the branch")
	}
}

// TestPublishAndWaitForTheWorkflowToMergeIt is the whole round trip: the
// registry's publish workflow (played by the fake) judges the pull
// request, merges it, and WaitForVerdict reports it published.
func TestPublishAndWaitForTheWorkflowToMergeIt(t *testing.T) {
	f := githubtest.New(t, "alice")
	f.ActAsPublishWorkflow("published", "")
	c := newTestClient(f)

	sub, err := c.Publish(context.Background(), alice, myPod("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.WaitForVerdict(context.Background(), sub, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if v.Outcome != Published {
		t.Fatalf("outcome = %q, want %q", v.Outcome, Published)
	}
	if f.Pulls()[0].State != "closed" {
		t.Error("the workflow didn't close the pull request")
	}
}

// TestWaitForVerdictTrustsOnlyTheWorkflowsVerdictOnTheSubmittedCommit
// proves a verdict counts only when GitHub Actions posted it, about the
// exact commit this attempt pushed — not one anyone else wrote, and not
// one left over from an earlier attempt.
func TestWaitForVerdictTrustsOnlyTheWorkflowsVerdictOnTheSubmittedCommit(t *testing.T) {
	f := githubtest.New(t, "alice")
	c := newTestClient(f)
	sub, err := c.Publish(context.Background(), alice, myPod("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}

	// The genuine verdict comes first, so the decoys after it would win
	// if they weren't filtered out.
	f.AddComment(sub.Number, githubtest.BotLogin, githubtest.VerdictComment(sub.HeadSHA, "needs-review", "MyPod already exists on CocoaPods trunk."))
	f.AddComment(sub.Number, "alice", githubtest.VerdictComment(sub.HeadSHA, "published", "not from the workflow"))
	f.AddComment(sub.Number, githubtest.BotLogin, githubtest.VerdictComment(strings.Repeat("c", 40), "published", "about another commit"))

	v, err := c.WaitForVerdict(context.Background(), sub, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if v.Outcome != NeedsReview || v.Details != "MyPod already exists on CocoaPods trunk." {
		t.Fatalf("verdict = %+v, want needs-review with the workflow's own details", v)
	}
}

// TestWaitForVerdictCleansTheWorkflowsReasons covers a rejection in the
// format the publish workflow posts it: the reasons, which can quote a
// pull request or CocoaPods trunk, come back without the comment's
// heading and indent, and without control characters that could drive
// the terminal or bidirectional-text controls that could disguise them.
func TestWaitForVerdictCleansTheWorkflowsReasons(t *testing.T) {
	f := githubtest.New(t, "alice")
	c := newTestClient(f)
	sub, err := c.Publish(context.Background(), alice, myPod("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	body := "This can't be published:\n\n" +
		"    - MyPod already exists on CocoaPods trunk\n" +
		"      - see \x1b]8;;https://evil.example\x07trunk\x1b]8;;\x07\u202e for its owners\r\n"
	f.AddComment(sub.Number, githubtest.BotLogin, githubtest.VerdictComment(sub.HeadSHA, "rejected", body))

	v, err := c.WaitForVerdict(context.Background(), sub, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := "- MyPod already exists on CocoaPods trunk\n  - see ]8;;https://evil.exampletrunk]8;; for its owners"
	if v.Outcome != Rejected || v.Details != want {
		t.Fatalf("verdict = %+v, want rejected with details %q", v, want)
	}
}

// TestWaitForVerdictReportsPendingWhenNothingArrivesInTime proves a slow
// workflow ends the wait cleanly rather than as an error.
func TestWaitForVerdictReportsPendingWhenNothingArrivesInTime(t *testing.T) {
	f := githubtest.New(t, "alice")
	c := newTestClient(f)
	sub, err := c.Publish(context.Background(), alice, myPod("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}

	v, err := c.WaitForVerdict(context.Background(), sub, 20*time.Millisecond)
	if err != nil || v.Outcome != Pending {
		t.Fatalf("verdict = %+v, err = %v, want Pending and no error", v, err)
	}
}

// TestWaitForVerdictDoesNotTrustAMergedFlag proves only the workflow's
// verdict means published. GitHub marks a pull request merged whenever
// its head becomes part of the registry's history, which says nothing
// about whether anything was checked, so a pull request that's merged or
// closed without a verdict is reported as not published.
func TestWaitForVerdictDoesNotTrustAMergedFlag(t *testing.T) {
	f := githubtest.New(t, "alice")
	c := newTestClient(f)
	sub, err := c.Publish(context.Background(), alice, myPod("1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	f.MergePull(sub.Number)

	v, err := c.WaitForVerdict(context.Background(), sub, time.Second)
	if err != nil || v.Outcome != Rejected || !strings.Contains(v.Details, "before the registry's check gave a verdict") {
		t.Fatalf("verdict = %+v, err = %v, want Rejected for want of a verdict", v, err)
	}
}

func TestUserReturnsTheAccountsIDAndLogin(t *testing.T) {
	f := githubtest.New(t, "alice")
	user, err := newTestClient(f).User(context.Background())
	if err != nil || user != alice {
		t.Fatalf("user = %+v, err = %v, want %+v", user, err, alice)
	}
}

func TestUserExplainsARejectedToken(t *testing.T) {
	f := githubtest.New(t, "alice")
	_, err := NewClient(f.URL, "expired-token").User(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("err = %v, want a hint to sign in again", err)
	}
}

func TestIsURL(t *testing.T) {
	for _, u := range []string{
		"https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main/",
		"https://raw.githubusercontent.com/zorvok-dev/zorvok-specs/main",
		"https://zorvok-dev.github.io/zorvok-specs/",
		"https://github.com/zorvok-dev/zorvok-specs",
		"https://github.com/Zorvok-Dev/Zorvok-Specs.git",
	} {
		if !IsURL(u) {
			t.Errorf("IsURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"",
		"http://localhost:8080",
		"https://pods.example.com/",
		"https://raw.githubusercontent.com/someone-else/zorvok-specs/main/",
		"https://raw.githubusercontent.com/zorvok-dev/zorvok-specs-fork/main/",
		"https://github.com/zorvok-dev/zorvok-ios",
	} {
		if IsURL(u) {
			t.Errorf("IsURL(%q) = true, want false", u)
		}
	}
}

func TestTokenPrefersTheEnvironment(t *testing.T) {
	t.Setenv("GH_TOKEN", " from-gh-token \n")
	t.Setenv("GITHUB_TOKEN", "from-github-token")
	if got, err := Token(); err != nil || got != "from-gh-token" {
		t.Fatalf("Token() = %q, %v; want GH_TOKEN first", got, err)
	}
	t.Setenv("GH_TOKEN", "")
	if got, err := Token(); err != nil || got != "from-github-token" {
		t.Fatalf("Token() = %q, %v; want GITHUB_TOKEN next", got, err)
	}
}

func TestTokenFallsBackToTheGitHubCLIsLogin(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	bin := t.TempDir()
	fakeGH := "#!/bin/sh\n[ \"$1 $2\" = \"auth token\" ] && echo gho_from_cli\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	if got, err := Token(); err != nil || got != "gho_from_cli" {
		t.Fatalf("Token() = %q, %v; want the GitHub CLI's token", got, err)
	}
}

func TestTokenWithoutAGitHubLoginSaysHowToSignIn(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PATH", t.TempDir())

	_, err := Token()
	if !errors.Is(err, ErrNoGitHubLogin) || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("err = %v, want ErrNoGitHubLogin explaining `gh auth login`", err)
	}
}
