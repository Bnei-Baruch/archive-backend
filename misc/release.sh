#!/usr/bin/env bash
# Usage: misc/release.sh
# Build package, tag a commit, push it to origin, and then deploy the
# package on production server.

set -e

echo "Building..."
make build

version="$(./archive-backend version | awk '{print $NF}')"
[ -n "$version" ] || exit 1
echo $version

echo "Tagging commit and pushing to remote repo"
git commit --allow-empty -a -m "Release $version"
git tag "v$version"
git push origin master
git push origin "v$version"

echo "Uploading executable to server"
scp archive-backend archive@app.archive.bbdomain.org:/sites/archive-backend/"archive-backend-$version"
ssh archive@app.archive.bbdomain.org "ln -sf /sites/archive-backend/archive-backend-$version /sites/archive-backend/archive-backend"

echo "Restarting application"
ssh archive@app.archive.bbdomain.org "supervisorctl restart archive"
ssh archive@app.archive.bbdomain.org "supervisorctl restart events"