#!/usr/bin/env bash
# push-to-main.sh MESSAGE PATH...
#
# Commits PATH... on top of the commit this run measured and pushes it to main,
# for the two jobs that write generated files back: the coverage badge and the
# documentation figures.
#
# A merge that lands while the job runs moves main, and a plain push is then
# refused ("fetch first"). So the commit is rebased onto the main it finds and
# the push retried. The files these jobs write are written by nothing else, so
# the rebase cannot conflict with a person's commit. The workflow keeps two
# runs of the same job from pushing at once (a concurrency group per job on
# main), which is what keeps an older render from landing after a newer one.
set -euo pipefail

message=$1
shift

# Staged first, so that a file the job created — a new figure — counts as a
# change as well as one it rewrote.
git add -- "$@"
if git diff --cached --quiet -- "$@"; then
	echo "nothing to commit in $*"
	exit 0
fi

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git commit -m "$message"

for attempt in 1 2 3 4 5; do
	git fetch origin main
	# main is protected against force pushes, so the commit this run measured
	# is always behind it. If it is not, something rewrote main, and writing a
	# badge or a figure onto that is not this job's decision.
	if ! git merge-base --is-ancestor "$GITHUB_SHA" origin/main; then
		echo "::warning::main no longer contains $GITHUB_SHA; not pushing"
		exit 0
	fi
	git rebase origin/main
	if git push origin HEAD:main; then
		exit 0
	fi
	echo "push refused (attempt $attempt); main moved again"
	sleep $((attempt * 5))
done

echo "::error::could not push to main after 5 attempts"
exit 1
