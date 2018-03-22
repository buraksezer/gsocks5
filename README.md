gsocks5
=======

[![Go Report Card](https://goreportcard.com/badge/github.com/buraksezer/gsocks5)](https://goreportcard.com/report/github.com/buraksezer/gsocks5)

Hassle-free and secure [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) server in the [Go](https://golang.org) programming language.

gsocks5 uses [go-socks5](https://github.com/armon/go-socks5) library to handle the protocol. UDP isn't supported by go-socks5, so gsocks5 doesn't support that protocol.

gsocks5 consists of two different parts: client and server.

* The client component runs on your computer and accepts TCP connections from your local host.

* The server component runs on your remote host and accepts connections from the client component. TLS is enabled on this server by default.

TLS is used to encrypt traffic(SOCKS5 protocol messages and other plain text TCP traffic like HTTP) between server and client components. After SOCKS5 is done with its job, your client and the outside world continue communication over that secured socket. This may seem bad to you. I think, this design choice doesn't create a performance bottleneck or security problem.

So you need to use an SSL certificate to run gsocks5. [Self-signed SSL certificates are good for personal use.](https://security.stackexchange.com/a/68339)

gsocks5 supports SOCKS5 authentication protocol. You need to set `socks5_username` and `socks5_password` fields in `server.json` file to get it. In addition, gsocks5 supports an internal authentication mechanism to protect your bandwith from outsiders. Just set `password` field to get that feature.

gsocks5 has been tested on GNU/Linux and OSX.

Installation
------------
With a correctly configured Go toolchain:

```sh
go get -u github.com/buraksezer/gsocks5
```

Edit the configuration file and and run it on your local host:

```sh
gsocks5 -c path/to/client.json
```

On your server:

```sh
gsocks5 -c path/to/server.json
```

For systemd users, service files for both components have been provided. Please take a look at **data** folder.

Configuration
-------------
There are two different configuration file under **data** folder.

#### client.json

Field        |  Type   | Description
------------ | ------- |-------------
role | string | Role of this server. Set client on your localhost.
debug | boolean | Disables or enables debug mode.
insecure_skip_verify | boolean | Disables TLS verification. It's useful if you use a self-signed TLS certificate.
server_addr | string | Remote SOCKS5 server address, the syntax of laddr is "host:port", like "127.0.0.1:8080".
client_addr | string | Local proxy server address, the syntax of laddr is "host:port", like "127.0.0.1:8080".
password | string| Password to authenticate local server on remote server. It's not relevant with SOCKS5 protocol. 
keepalive_period | int | Period between keep alives, in seconds.
dial_timeout | int | Timeout value for dialing, in seconds.

#### server.json

Field        |  Type   | Description
------------ | ------- |-------------
role | string | Role of this server. Set server on the remote host.
debug | boolean | Disables or enables debug mode.
server_addr | string | Address to listen, the syntax of laddr is "host:port", like "127.0.0.1:8080".
password | string | Password to authenticate local server on remote server. It's not relevant with SOCKS5 protocol. 
socks5_username | string | Username for SOCKS5 Authentication protocol.
socks5_password | string | Password for SOCKS5 Authentication protocol.
server_cert | string | Path of the SSL certificate file.
server_key | string | Path of the private key file of your certificate.

You may need to generate a self-signed SSL certificate for the server component, the following command should work for you:

```sh
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365
```

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
