FROM ubuntu:latest

RUN apt-get update && \
    apt-get -uy upgrade && \
    apt-get install -y ca-certificates software-properties-common gpg && \
    add-apt-repository ppa:longsleep/golang-backports && \
    apt-get update && \
    update-ca-certificates
RUN apt-get -y install ed git golang-go make

ADD . /coredns-ens/
RUN chmod 755 coredns-ens/build.sh && coredns-ens/build.sh

FROM ubuntu:latest
COPY --from=0 /etc/ssl/certs /etc/ssl/certs
COPY --from=0 /coredns /coredns

EXPOSE 53 53/udp
EXPOSE 853
EXPOSE 443
ENTRYPOINT ["/coredns"]
