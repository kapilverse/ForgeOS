package builder

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Clone shallow-clones the given public git repo into dest (a fresh, empty
// directory). branch selects the ref to check out; empty means the repo's
// default branch. A depth-1 clone keeps it fast for build use.
//
// Only public HTTPS repos are supported in this sprint; private-repo auth
// (deploy keys / PATs) is deferred.
func Clone(ctx context.Context, repoURL, branch, dest string) error {
	refName := plumbing.NewBranchReferenceName(branch)

	opts := &git.CloneOptions{
		URL:   repoURL,
		Depth: 1,
	}
	// Pin to the requested branch when one is given. When branch is empty we
	// let git pick the remote's default HEAD.
	if branch != "" {
		opts.ReferenceName = refName
		opts.SingleBranch = true
	}

	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid clone options: %w", err)
	}

	if _, err := git.PlainCloneContext(ctx, dest, false, opts); err != nil {
		return fmt.Errorf("git clone %s: %w", repoURL, err)
	}
	return nil
}
