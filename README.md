gsocks5
=======
Hassle-free and secure [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) server in the [Go](https://golang.org) programming language. 

Design
------
gsocks5 uses [go-socks5](https://github.com/armon/go-socks5) library to handle SOCKS5 protocol. The other stuff (e.g. connection management and security) is handled by gsocks5. 
There is no interesting stuff at connection management. It's a standard [TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol) server with graceful shutdown. 

The key point of gsocks5 is to use [TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security) for hiding SOCKS5 protocol messages between local SOCKS5 server and remote one. If everything goes fine,
gsocks5 creates a new and plain TCP socket to the remote server and proxies client request over that socket.

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
* UDP support
* Unit tests
