gsocks5
=======
Hassle-free and secure [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) server in the [Go](https://golang.org) programming language. 

gsocks5 uses [go-socks5](https://github.com/armon/go-socks5) library to handle the protocol and TLS to hide the traffic between your client 
and remote SOCKS5 server.

Installation
------------
With a correctly configured Go toolchain:
```sh
go get -u github.com/purak/gsocks5
```

Get vendored dependencies:
```sh
git submodule update --init --recursive
```

Edit the configuration file and and run it:
```sh
gsocks5 -c data/gsocks5.yml
```

Configuration
-------------

There are two different configuration file under data folder. 

*client.json*

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
* UDP relay
* Unit tests
