# Description of the issue
An automated PR to kickstart the process of syncing the latest changes from [cw-agent](https://github.com/aws/amazon-cloudwatch-agent/)

# Description of changes

### Follow the git CLI instructions resolve the merge conflicts 

```shell
git pull origin main
git checkout repo-sync-<hash>-<run_id>
git merge main # do a regular merge -- we want to keep the commits
# resolve merge conflicts in your preferred IDE
git push -u origin repo-sync-<hash>-<run_id>
```

Some useful commands
* [Restore conflict resolution in a single file](https://stackoverflow.com/questions/14409420/restart-undo-conflict-resolution-in-a-single-file) - `git checkout -m <FILE>`
* Total reset - `git merge --abort`

### Related docs
* Resolving conflicts with:
    * [Git CLI](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/addressing-merge-conflicts/resolving-a-merge-conflict-using-the-command-line)
    * [IntelliJ](https://www.jetbrains.com/help/idea/resolving-conflicts.html#distributed-version-control-systems)
    * [GoLand](https://www.jetbrains.com/help/go/resolve-conflicts.html)
    * [VSCode](https://learn.microsoft.com/en-us/visualstudio/version-control/git-resolve-conflicts?view=vs-2022)

### Best practices 

* Remember to update all references from `amazon-cloudwatch-agent` to `private-amazon-cloudwatch-agent-staging`
* Resolve the `go.sum` with `go mod tidy`. Don't bother manually resolving conflicts in this file
* When finished, ensure builds work by using `make build` or `make release`
* When unsure or blocked, do a deep dive on the `git blame` for greater context. Maybe even look for the associated PR's and ask the original authors and PR approvers
* If another automated PR arrives before your work is merged, just close your current one and save the branch
* After your PR is approved, **do a regular merge to preserve the commits**. 
* Remember to cleanup your commits because none of them will be squashed in a regular merge

# License
By submitting this pull request, I confirm that you can use, modify, copy, and redistribute this contribution, under the terms of your choice.

# Tests
n/a

# Requirements
_Before commit the code, please do the following steps._
1. Run `make fmt` and `make fmt-sh`
2. Run `make lint`
