TAG = v1.0
PREFIX = gcr.io/kawych-test

metrics_exposer: metrics_exposer.go
	go build -a -o metrics_exposer metrics_exposer.go

docker: metrics_exposer
	docker build --pull -t ${PREFIX}/metrics-exposer:$(TAG) .

push: docker
	gcloud docker -- push ${PREFIX}/metrics-exposer:$(TAG)

clean:
	rm -rf metrics_exposer
