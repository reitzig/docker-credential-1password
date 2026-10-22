#!/usr/bin/env bash

set -euo pipefail

# Confirm we are on main
if [[ "$(git branch --show-current)" != "main" ]]; then
    echo "❌ Will not release branch '$(git branch --show-current)'"
    exit 1
fi

# Confirm we are up to date
git fetch
if (($(git rev-list --left-only --count main...origin/main) > 0)); then
    echo "❌ Will not release while ahead of origin"
    exit 2
elif (($(git rev-list --right-only --count main...origin/main) > 0)); then
    echo "❌ Will not release while behind origin"
    exit 3
fi
# -> we can now assume that we release a state that tested green,
#    as long as we haven't circumvent any branch protection rules!

# Create release branch
release_branch="release/$(changie next auto)"
git switch -c "${release_branch}"

# Assemble & commit changelog
changie batch auto
changie merge
version="$(changie latest)"
git add .changes/unreleased/* ".changes/${version}.md" "CHANGELOG.md"
git commit --all -m "chore(docs): CHANGELOG for ${version}"

# Tag & push
git tag "${version}"
git push --set-upstream origin "${release_branch}" "${version}"

repo_url="$(
    git remote get-url origin \
    | sed \
        -e 's%:%/%' \
        -e 's%^git@%https://%' \
        -e "s%\\.git$%%"
)"

cat <<LINES
🥳 All done!
➡️ Create & merge: ${repo_url}/compare/${release_branch}?expand=1
    ⚠️ Merge ff-only!
⏳️ Wait for release workflow to complete: ${repo_url}/actions/workflows/release.yaml
➡️ Review & approve: ${repo_url}/releases/tag/${version}
LINES

git switch main
