# Demo Bank application

This project is a simple bank application that utilizes the standard consensus algorithm [PAXOS](https://lamport.azurewebsites.net/pubs/paxos-simple.pdf).</br>

## Requirements
This project is implemented in [Go](https://golang.org/). To compile the application you need the following:<br>

- cURL — latest version
- Go — version 1.17.x
- Docker Compose — version 1.29.x <br/>


## Run our bank application
You can either run the application locally or in a docker network. To run in an existing docker network:<br>

```shell
docker build -t paxos_server -f bank_app/Dockerfile .
./ runx.sh
```

This command will run the application on x (3, 5, or 7) TCP servers. You can by issuing the following command interact with one of the servers:<br>

```shell
 docker run -h 3,:42237 -it --name cli2 --net paxos-lab5 --ip 172.28.1.10 --expose=42237 --rm paxos_server
```

### Good luck :-)
