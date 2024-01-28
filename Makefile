build-linux:
	GOOS=linux GOARCH=amd64 go build -o inspect-tf-plan ./cmd/inspect-tf-plan/main.go