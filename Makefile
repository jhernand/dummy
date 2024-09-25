image := quay.io/jhernand/dummy:latest

build:
	CGO_ENABLED=0 go build

image: build
	podman build -t "$(image)" .

push: image
	podman push "$(image)"
