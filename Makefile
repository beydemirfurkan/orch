VERSION ?=
TAG ?=

.PHONY: test pack-dry changelog release-prepare release-patch release-minor release-major

test:
	go test ./...

pack-dry:
	npm pack --dry-run

changelog:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make changelog VERSION=0.1.1" && exit 1)
	node scripts/changelog.js $(VERSION)

release-prepare:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make release-prepare VERSION=0.1.1" && exit 1)
	node scripts/release.js $(VERSION) $(TAG)

release-patch:
	node scripts/release.js patch $(TAG)

release-minor:
	node scripts/release.js minor $(TAG)

release-major:
	node scripts/release.js major $(TAG)
