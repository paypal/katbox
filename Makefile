# Copyright 2019 The Kubernetes Authors.
#
# Modifications copyright 2020 PayPal.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)
VERSION := 1.0.0_$(shell git rev-parse --short HEAD)

all: build

clean:
	-rm -rf bin

build:
	CGO_ENABLED=0 GOOS="linux" GOARCH="amd64" go build -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/katboxplugin ./cmd/katboxplugin/main.go

docker-build-katbox:
	docker build  . --tag quay.io/katbox/katboxplugin:${VERSION} --no-cache

docker-push-katbox:
	docker push quay.io/katbox/katboxplugin:${VERSION}
   
docker-build-stream:
	docker build  ./stream --tag quay.io/katbox/stream:${VERSION} --no-cache

docker-push-stream:
	docker push quay.io/katbox/stream:${VERSION}

ci-build: docker-build-katbox docker-build-stream docker-push-katbox docker-push-stream 

