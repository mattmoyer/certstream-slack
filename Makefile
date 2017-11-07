# Copyright 2017 by the contributors.
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

default: build

REPO ?= gcr.io/heptio-images/certstream-slack
VERSION ?= v0.1.0

.PHONY: build push build-container

build: build-container

build-container: ca-certificates.crt
	GOOS=linux GOARCH=amd64 go build -o certstream-slack main.go
	docker build . -t $(REPO):$(VERSION)

# pull ca-certificates.crt from Alpine
ca-certificates.crt:
	docker run -v "$$PWD":/out --rm --tty -i alpine:latest /bin/sh -c "apk add --update ca-certificates && cp /etc/ssl/certs/ca-certificates.crt /out/"

push:
	docker push $(REPO):$(VERSION)

format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"
