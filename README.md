gsocks5
=======
Hassle-free and secure [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) server in the [Go](https://golang.org) programming language. 

gsocks5 utilizes [HTTP/2](https://en.wikipedia.org/wiki/HTTP/2) protocol to hide SOCKS5 traffic between your client and remote SOCKS5 server. After doing SOCKS5 communication, gsocks5 creates
a new [TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol)(without [TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security)) socket to proxy your traffic through that socket. 

gsocks5 uses [go-socks5](https://github.com/armon/go-socks5) library to handle the protocol.

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

TO-DO

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
