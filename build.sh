gitCommit=$(git show -s --format=%s)
version=$(git describe --abbrev=0 --tags)
cd ./cmd/miner-proxy/
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64   go build -a -ldflags "-s -w -extldflags '-static' -X 'main.version=${version}'   -X 'main.gitCommit=${gitCommit}'" -o ../../miner-proxy_darwin_amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64   go build -a -ldflags "-s -w -extldflags '-static' -X 'main.version=${version}'  -X 'main.gitCommit=${gitCommit}'" -o ../../miner-proxy_linux_amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm   go build -a -ldflags "-s -w -extldflags '-static' -X 'main.version=${version}'  -X 'main.gitCommit=${gitCommit}'" -o ../../miner-proxy_linux_arm .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -a -ldflags "-s -w -extldflags '-static' -X 'main.version=${version}'  -X 'main.gitCommit=${gitCommit}'" -o ../../miner-proxy_windows_amd64.exe .
CGO_ENABLED=0 GOOS=windows GOARCH=arm  go build -a -ldflags "-s -w -extldflags '-static' -X 'main.version=${version}'  -X 'main.gitCommit=${gitCommit}'" -o ../../miner-proxy_windows_arm.exe .