FROM golang:1.17.6-alpine3.15

ADD . /home/miner-proxy

ENV GOPROXY=https://goproxy.io,direct

RUN cd /home/miner-proxy && go mod tidy && cd ./cmd/miner-proxy && go build .

RUN mv /home/miner-proxy/docker-entrypoint /usr/bin/ && \
    mv /home/miner-proxy/cmd/miner-proxy/miner-proxy /usr/bin/ && \
    rm -rf /home/miner-proxy

WORKDIR /home

EXPOSE 9999

ENTRYPOINT ["docker-entrypoint"]

CMD ["miner-proxy","-h"]
