.PHONY: help
help:
	@echo "Make targets for trumpet"
	@echo "------------------------"
	@echo "This Makefile is used to build and run the Trumpet application. It includes several targets for building different components of the application, such as yt-dlp, ffmpeg, and the Trumpet application itself. It also includes targets for building and running the application in development mode. Additionally, there are targets for pushing the built images to a container registry and for stopping the running application. The Makefile includes a help target that provides a summary of all available targets."
	@echo
	@echo "Usage:"
	@echo
	@echo "* Build yt-dlp dev or latest:"
	@echo "    make build-yt-dlp"
	@echo "    make build-yt-dlp-dev"
	@echo
	@echo "* Build ffmpeg:"
	@echo "    make build-ffmpeg"
	@echo
	@echo "* Build trumpet dev or latest:"
	@echo "    make build-trumpet"
	@echo "    make build-trumpet-dev"
	@echo
	@echo "* Build dev or latest:"
	@echo "    make docker-build"
	@echo "    make docker-build-dev"
	@echo
	@echo "* Run trumpet in dev or latest"
	@echo "    make docker-run"
	@echo "    make docker-run-debug"
	@echo
	@echo "* Stop trumpet:"
	@echo "    make docker-stop"


build-yt-dlp:
	cd yt-dlp && \
	docker build -t ghcr.io/goproslowyo/chainguard-python-yt-dlp:latest .

build-yt-dlp-dev:
	cd yt-dlp && \
	docker build -f Dockerfile.dev -t ghcr.io/goproslowyo/chainguard-python-yt-dlp:dev .

build-ffmpeg:
	git submodule update --init --recursive && \
	cd static-ffmpeg && \
	git checkout -b trumpet && \
	git apply ../ffmpeg.patch && \
	docker build -t ghcr.io/goproslowyo/ffmpeg-static:latest . && \
	git checkout master && \
	git checkout -- Dockerfile && \
	git branch -D trumpet

build-trumpet:
	cd src && \
	docker build -f ../Dockerfile -t ghcr.io/goproslowyo/trumpet:latest .

build-trumpet-dev:
	cd src && \
	docker build -f ../Dockerfile.dev -t ghcr.io/goproslowyo/trumpet:dev .

docker-build:
	$(MAKE) build-yt-dlp && \
	$(MAKE) build-ffmpeg && \
	$(MAKE) build-trumpet


docker-build-dev:
	$(MAKE) build-yt-dlp-dev && \
	$(MAKE) build-ffmpeg && \
	$(MAKE) build-trumpet-dev

docker-push:
	docker push ghcr.io/goproslowyo/chainguard-python-yt-dlp:latest && \
	docker push ghcr.io/goproslowyo/ffmpeg-static:latest && \
	docker push ghcr.io/goproslowyo/trumpet:latest

docker-run:
	$(MAKE) docker-build && \
	docker compose up -d

docker-run-debug:
	$(MAKE) docker-build-dev && \
	docker compose -f docker-compose.debug.yml up -d && \
	docker exec -it trumpet-trumpet-1 bash

docker-stop:
	docker compose down
