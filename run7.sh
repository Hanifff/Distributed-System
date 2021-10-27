#!/bin/sh

docker run -h 172.28.1.2:1234-7 -itd --name paxos1 --net paxos-lab5 --ip 172.28.1.2 --expose=1234 --rm paxos_server
#docker attach paxos1

docker run -h 172.28.1.3:1235-7 -itd --name paxos2 --net paxos-lab5 --ip 172.28.1.3 --expose=1235 --rm paxos_server
#docker attach paxos2

docker run -h 172.28.1.4:1236-7 -itd --name paxos3 --net paxos-lab5 --ip 172.28.1.4 --expose=1236 --rm paxos_server
#docker attach paxos3

docker run -h 172.28.1.5:1237-7 -itd --name paxos4 --net paxos-lab5 --ip 172.28.1.5 --expose=1237 --rm paxos_server
#docker attach paxos4

docker run -h 172.28.1.6:1238-7 -itd --name paxos5 --net paxos-lab5 --ip 172.28.1.6 --expose=1238 --rm paxos_server
#docker attach paxos5

docker run -h 172.28.1.7:1239-6 -itd --name paxos6 --net paxos-lab5 --ip 172.28.1.7 --expose=1239 --rm paxos_server
#docker attach paxos4

docker run -h 172.28.1.8:1240-7 -itd --name paxos7 --net paxos-lab5 --ip 172.28.1.8 --expose=1240 --rm paxos_server
#docker attach paxos5