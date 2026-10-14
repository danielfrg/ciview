package ci

// MatchesBranchScope keeps branch names local to their repository. All-scope
// history includes every run; named scopes require known local provenance for
// GitHub pull request events, whose head branch may belong to a fork.
func MatchesBranchScope(scope string, repository Repository, run Run) bool {
	if scope == "all" {
		return true
	}
	if scope == "default" {
		scope = repository.DefaultBranch
	}
	if run.Branch != scope {
		return false
	}
	if run.HeadRepositoryID != "" && repository.ID != "" && run.HeadRepositoryID != repository.ID {
		return false
	}
	if repository.Provider == "github" {
		switch run.Event {
		case "pull_request", "pull_request_target", "pull_request_review", "pull_request_review_comment":
			return run.HeadRepositoryID != "" && repository.ID != "" && run.HeadRepositoryID == repository.ID
		}
	}
	return true
}
