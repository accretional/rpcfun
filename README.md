# rpcfun

What if you could do functional programming in grpc and protobuf? And what if you used that to implement mapreduce over a filesystem, or unix-style pipe-processing? Functions as a Service, also cool idea.

Here's the idea:

```bash
alias grpcfun=$(grpcloader --bind bar=serviceFoo.methodBar:/usr/bin/BarImpl cool=coolService.Cool:~/cool)

grpcfun --plan9.Open foo.csv | grpcfun --map --bar | grpcfun --reduce --cool | grpcurl --somermoteservice

cat stuff.csv | grep "junk" | grpcfun --parse serviceFoo.TableRecord > records.binarypb

```

# Resources

Some notes, resoures, related projects.

## Golang Unix-style Scripting

This tool seems extremely relevant: https://github.com/bitfield/script it doesn't implement mapreduce or concern itself with grpc/proto, but it does implement unix-style command piping in Go.

I think using Tee might work for map, but another problem is implementing reduce as an accumulator. Nothing really seems to implement that pattern, although there is something for sinks. Don't really want to do this in pure go, might try modifying the package itself.

## Become Another Process

I realized slightly more embarrasingly late than I'd like that Go doesn't actually support hot module/runtime loading very well: they have a plugin system but it's brittle and not meant for the kind of stuff I want to do with dynamic loading.

Command and subprocess libraries would work, but they require starting a child process, and then you have to keep track of it and manage its lifecycle. So it would be better to just switch processes entirely in some cases, and you can do that with syscall.Exec.

```go

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	// 1. Specify the new program to run.
	// We'll use "ls" as an example.
	binary, lookErr := exec.LookPath("ls")
	if lookErr != nil {
		panic(lookErr)
	}

	// 2. Define the arguments.
	// The first argument must be the program name.
	args := []string{"ls", "-l", "-h"}

	// 3. Get the current environment variables.
	env := os.Environ()

	// 4. Call syscall.Exec
	// On success, this function does not return.
	execErr := syscall.Exec(binary, args, env)
	if execErr != nil {
		// If it returns, there was an error.
		fmt.Printf("Exec error: %v\n", execErr)
		os.Exit(1)
	}
}
```

## Reflection

https://github.com/jhump/protoreflect this will help generate builders, new .proto files, and validate types.

## Protocompile

https://pkg.go.dev/github.com/bufbuild/protocompile#section-readme this will help compile the protos

## Protocmp

https://pkg.go.dev/google.golang.org/protobuf/testing/protocmp#Transform this seems useful for type/compatibility checking.

## Protovalidate

https://github.com/bufbuild/protovalidate this might help constrain the ways higher order functions interact with regular functions.

## Golang Build Internals

https://pkg.go.dev/go these could actually be useful for speeding up/reusing resources across similar go builds.
