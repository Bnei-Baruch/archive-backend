#!/usr/bin/env bash
# Usage: misc/release.sh
# Build package, tag a commit, push it to origin, and then deploy the
# package on production server.

set -e

echo "Building..."
make build

version="$(./mdb2es version | awk '{print $NF}')"
[ -n "$version" ] || exit 1
echo $version

echo "Tagging commit and pushing to remote repo"
git commit --allow-empty -a -m "Release $version"
git tag "v$version"
git push origin master
git push origin "v$version"

echo "Uploading executable to server"
scp mdb2es root@app.archive.bbdomain.org:/sites/mdb2es/"mdb2es-$version"
ssh root@app.archive.bbdomain.org "ln -sf /sites/mdb2es/mdb2es-$version /sites/mdb2es/mdb2es"

echo "Restarting application"
ssh root@app.archive.bbdomain.org "supervisorctl restart mdb2es_esplorer"