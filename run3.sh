#!/bin/sh

# Command to start a docker container as a paxos server
#docker build -t paxos_server -f lab5/Dockerfile .

docker run -h 172.28.1.2:1234-3 -itd --name paxos1 --net paxos-lab5 --ip 172.28.1.2 --expose=1234 --rm paxos_server
#docker attach paxos1

docker run -h 172.28.1.3:1235-3 -itd --name paxos2 --net paxos-lab5 --ip 172.28.1.3 --expose=1235 --rm paxos_server
#docker attach paxos2

docker run -h 172.28.1.4:1236-3 -itd --name paxos3 --net paxos-lab5 --ip 172.28.1.4 --expose=1236 --rm paxos_server
#docker attach paxos3