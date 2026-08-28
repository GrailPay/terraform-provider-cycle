default: build

.PHONY: build
build:
	go build -o terraform-provider-cycle .

.PHONY: install
install:
	go install .

.PHONY: fmt
fmt:
	gofmt -w .
	terraform fmt -recursive ./examples/

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./... -v

.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v -timeout 120m

.PHONY: docs
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name cycle
