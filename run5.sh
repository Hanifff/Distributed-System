#!/bin/sh


docker run -h 172.28.1.2:1234-5 -itd --name paxos1 --net paxos-lab5 --ip 172.28.1.2 --expose=1234 --rm paxos_server
#docker attach paxos1

docker run -h 172.28.1.3:1235-5 -itd --name paxos2 --net paxos-lab5 --ip 172.28.1.3 --expose=1235 --rm paxos_server
#docker attach paxos2

docker run -h 172.28.1.4:1236-5 -itd --name paxos3 --net paxos-lab5 --ip 172.28.1.4 --expose=1236 --rm paxos_server
#docker attach paxos3

# NB: change the number of nodes -4 to -5
docker run -h 172.28.1.5:1237-5 -itd --name paxos4 --net paxos-lab5 --ip 172.28.1.5 --expose=1237 --rm paxos_server
#docker attach paxos4

docker run -h 172.28.1.6:1238-5 -itd --name paxos5 --net paxos-lab5 --ip 172.28.1.6 --expose=1238 --rm paxos_server
#docker attach paxos5