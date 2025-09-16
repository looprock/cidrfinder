BINARY_NAME=bootstrap
HANDLER_NAME=cidrfinder

.PHONY: build clean test deploy package

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) .

test:
	go test -v ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f function.zip

package: build
	zip function.zip $(BINARY_NAME)

deploy: package
	aws lambda update-function-code \
		--function-name $(HANDLER_NAME) \
		--zip-file fileb://function.zip

create-table:
	aws dynamodb create-table \
		--table-name cidr-registry \
		--attribute-definitions \
			AttributeName=key,AttributeType=S \
		--key-schema \
			AttributeName=key,KeyType=HASH \
		--billing-mode PAY_PER_REQUEST \
		--tags Key=Purpose,Value=CIDRManagement

delete-table:
	aws dynamodb delete-table --table-name cidr-registry

install-deps:
	go mod tidy
	go mod download
