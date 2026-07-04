#!/bin/sh
#
# Staged release for this multi-module repo. Each layer's tags must be pushed
# before the next layer's go.mod can reference them:
#   1. tag+push raftpb
#   2. bump raftapi -> commit, tag+push
#   3. bump raftmod/raftgrpc/raftvrpc -> commit, tag+push
#
# Usage: ./release.sh v0.4.0

set -e
V=${1:?usage: ./release.sh vX.Y.Z}

# Avoid module proxy/sumdb propagation lag for tags pushed seconds ago.
export GOPRIVATE=go.arpabet.com
export GOPROXY=direct
export GOFLAGS=-mod=mod

echo "== testing workspace =="
for m in raftpb raftapi raftmod raftgrpc raftvrpc; do
	(cd "$m" && go build ./... && go test ./...)
done

echo "== 1/3 raftpb $V =="
git tag "raftpb/$V"
git push origin "raftpb/$V"

echo "== 2/3 raftapi $V =="
(cd raftapi && go get "go.arpabet.com/raft/raftpb@$V" && go mod tidy)
git add raftapi/go.mod raftapi/go.sum
git commit -m "release $V"
git tag "raftapi/$V"
git push origin HEAD "raftapi/$V"

echo "== 3/3 raftmod raftgrpc raftvrpc $V =="
for m in raftmod raftgrpc raftvrpc; do
	(cd "$m" && go get "go.arpabet.com/raft/raftapi@$V" "go.arpabet.com/raft/raftpb@$V" && go mod tidy)
done
git add raftmod/go.mod raftmod/go.sum raftgrpc/go.mod raftgrpc/go.sum raftvrpc/go.mod raftvrpc/go.sum
git commit -m "release $V"
git tag "raftmod/$V" "raftgrpc/$V" "raftvrpc/$V"
git push origin HEAD "raftmod/$V" "raftgrpc/$V" "raftvrpc/$V"

echo "== released $V =="
