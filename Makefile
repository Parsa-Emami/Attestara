.PHONY: setup start dev build check test package
setup:
	npm run setup
start:
	npm start
dev:
	npm run dev
build:
	npm run build
check:
	npm run check
test:
	go test ./...
	npm test
package:
	npm run package
