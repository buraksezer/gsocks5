gsocks5
=======
Hassle-free and secure [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) server in the [Go](https://golang.org) programming language. 

gsocks5 uses [go-socks5](https://github.com/armon/go-socks5) library to handle the protocol. Due to go-socks5 doesn't support SOCKS5 over UDP,  gsocks5 cannot handle that protocol.

gsocks5 consists of two different parts: client and server. The server component runs on your server and accepts connections from your client processes. The client process works on your computer and accepts TCP connections from your local processes i.e. your browser, git or curl. 

TLS is used to encrypt traffic(SOCKS5 protocol messages) between server and client components. After SOCKS5 is done with its job, your client and the outside world continue communication over that secured socket. This may seem bad to you. But I think this design choice doesn't create a performance bottleneck or security hole.

#### Disclaimer
gsocks5 has been produced for personal use. 

#### Status
I use gsocks5 sice Jun, 2017 and it works fine for me.

Installation
------------
With a correctly configured Go toolchain:

```sh
go get -u github.com/purak/gsocks5
```

Edit the configuration file and and run it on your localhost:

```sh
gsocks5 -c path/to/client.json
```

On your server:
```sh
gsocks5 -c path/to/server.json
```

If you use systemd, service files for both components have been provided. Please take a look at **data** folder.

Configuration
-------------

There are two different configuration file under data folder. 

#### client.json

Field        | Description
------------ | -------------
debug | Boolean. Disables or enables debug mode.
insecure_skip_verify | Boolean. Disables TLS verification. It's useful if you use a self-signed TLS certificate.
server_addr | String. Remote SOCKS5 server address, the syntax of laddr is "host:port", like "127.0.0.1:8080".
client_addr | String. Local proxy server address, the syntax of laddr is "host:port", like "127.0.0.1:8080".
password | Password to authenticate local server on remote server. It's not relevant with SOCKS5 protocol. 

Contributions
-------------
Please don't hesitate to fork the project and send a pull request or just e-mail me to ask questions and share ideas.

License
-------
The Apache License, Version 2.0 - see LICENSE for more details.

TODO
----
* Implement UDP relay, if go-socks5 decides to support UDP.
* Unit tests
