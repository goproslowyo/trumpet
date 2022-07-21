docker-build:
	docker build -t trumpet:latest .

docker-run:
	docker run -itv $PWD/config.json:/config.json -v $PWD/audio:/audio -v $PWD/announcements:/announcements -v $PWD/google-translate-api-credentials.json:/google-translate-api-credentials.json trumpet:latest
