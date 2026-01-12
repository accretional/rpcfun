# rpcfun

What if you could do functional programming in grpc and protobuf? And what if you used that to implement mapreduce over a filesystem, or unix-style pipe-processing? Functions as a Service, also cool idea.

Here's the idea:

```bash
alias grpcfun=$(grpcloader --bind bar=serviceFoo.methodBar:/usr/bin/BarImpl cool=coolService.Cool:~/cool)

grpcfun --plan9.Open foo.csv | grpcfun --map --bar | grpcfun --reduce --cool | grpcurl --somermoteservice

cat stuff.csv | grep "junk" | grpcfun --parse serviceFoo.TableRecord > records.binarypb

```
