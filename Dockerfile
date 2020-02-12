FROM golang:buster

ENV START_SSHD false
ENV USER_PASSWD rcom

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install openssh-server

RUN service ssh start

RUN useradd -ms /bin/bash -d /home/rcom rcom
RUN echo "rcom:$USER_PASSWD" | chpasswd

RUN mkdir -p /home/rcom/devel/rcom
COPY . /home/rcom/devel/rcom/
RUN chown -R rcom:rcom /home/rcom/devel/rcom/*
USER rcom
RUN cd /home/rcom/devel/rcom/cmd/rcom && go get
RUN cd /home/rcom/devel/rcom/cmd/rcom && go build
RUN cd /home/rcom/devel/rcom/cmd/rcom && go install

USER root
CMD sh -c 'if [ "$START_SSHD" = true ]; then /usr/sbin/sshd -D ; else su rcom ; fi'

