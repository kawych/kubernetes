FROM gcr.io/google-containers/debian-base-amd64:0.1

ADD metrics_exposer metrics_exposer

RUN chmod +x metrics_exposer

RUN clean-install ca-certificates

USER 65534:65534
