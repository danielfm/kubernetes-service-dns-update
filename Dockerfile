FROM centurylink/ca-certs
MAINTAINER Daniel Martins <daniel.tritone@gmail.com>

COPY ./kubernetes-service-dns-update /kubernetes-service-dns-update
ENTRYPOINT ["/kubernetes-service-dns-update"]
