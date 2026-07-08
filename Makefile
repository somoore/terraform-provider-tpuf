.PHONY: build test testacc fmt vet check-security docs hooks

build:
	go build -o terraform-provider-tpuf

# Fail if any file is not gofmt-clean, listing the offenders.
fmt:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then echo "gofmt needed:"; echo "$$files"; exit 1; fi

vet:
	go vet ./...

# Dependency-pinning, secret, and .env checks (see ci/checks.sh).
check-security:
	@./ci/checks.sh

# `make test` runs everything: format check, vet, every unit test in the
# repo, and the security checks. Acceptance tests are excluded (they need
# real credentials — use `make testacc`).
test: fmt vet check-security
	go test ./...

# Acceptance tests — provision real namespaces (prefixed tf-acc-test-) and
# may incur costs. Requires TURBOPUFFER_API_KEY and TURBOPUFFER_REGION.
testacc:
	TF_ACC=1 go test ./internal/provider/ -run TestAcc -v -timeout 30m

# Regenerate docs/ from schema descriptions + examples/.
# NOTE: tfplugindocs wipes docs/ of anything it didn't render.
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name tpuf

# Point git at the versioned hooks dir so pre-commit AND pre-push run
# automatically. One setting, tracked in the repo, survives fresh clones
# (each clone just needs `make hooks` once).
hooks:
	@git config core.hooksPath ci/hooks
	@chmod +x ci/hooks/*
	@echo "git hooks activated (core.hooksPath -> ci/hooks)"
