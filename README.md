parget
======

A toy protocol for parallel file downloading on top of TCP.
The base idea is: instead of loading full file via single TCP connection, load separate chunks of file via multiple connections.
This does not give any meaningful performance increase over simple TCP connection as you are still limited by the slowest link.
But as I've already said, it's a toy.

The code was not tested well and many corner cases may occur. But it does work and does load files properly.

Client and server are under cmd folder, top-level package only contains some common messaging stuff.
