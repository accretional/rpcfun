# Premise

What if we used something like the builder logic from https://pkg.go.dev/github.com/jhump/protoreflect/v2@v2.0.0-beta.2/protobuilder#pkg-overview, dynamic loading of service implementation binaries, and explicitly bound, named Method-Impl pairs to create a Function abstraction in proto/grpc?

Then, what if we implemented general support for the Unix File I/O pattern (actually more based on Plan9, because we want everything to be a file) and MapReduce to implement proto-based ETL on the command line? Like a command that sequences Unix pipes to process text, except with proto messages.

That would required some notion of higher order functions, maybe. It's annoying to represent those because they have different message types than their base Functions. The Functions below I think aren't capable of things like bound function arguments and currying.

But if we used https://pkg.go.dev/github.com/jhump/protoreflect/v2@v2.0.0-beta.2/protobuilder#pkg-overview to create composite types matching the relationship between a Higher Order Function and base function(s), we could represent those new messages!

```proto
message FooRequest {

}

message FooResponse {

}

service Foo {
  rpc DoFoo(FooRequest) returns (FooResponse);
}

message Collection {
  string name = 1;
  Type type = 2;
  // validate that this belongs to Type
  repeated google.protobuf.Any messages = 3;
  string file_uri = 4;
}

message File {
  string file_uri = 1;
  // whatever we know about this file being open, used as a fd, being something like a socket, soft link, named pipe, uds, etc.
  FileStatus file_status = 2;
}

message FileStatus {
  int fd = 1;
  // some enum for whether this is a socket, soft link, named pipe, uds, etc.
  // also something more read/buffer status-oriented?
}

message WriteReq {
  // something like this
  oneof {
    int fd_or_offset = 1;
    bytes data = 3;
  }
}

// some reference function signatures at http://rfc.nop.hu/plan9/rfc9p.pdf
service Plan9 {
  rpc Open(string) returns(FileStatus);
  rpc Read(int) returns(stream bytes);
  rpc Write(stream WriteReq) returns (int);
  rpc Close (int) returns (google.protobuf.Empty);
}

message Function {
  string name = 1;
  MethodDescriptor method = 2;
  File impl = 3; //path to binary or hot loaded package implementing method. Could also be discovered or something.
}

message HigherOrderFunction {
  string name = 1;
  Function hof = 2;
  Function target = 3;
}

message PipedCall {
  int fd_in = 1;
  HigherOrderFunction = 2;
  int fd_out = 3;
  optional int fd_err = 4;
}

message PipedCmd {
  string file_path_in = 1;
  repeated PipeCall = 2;
  optional file_path_out = 3; // otherwise to stdout
}

service MapReduce {
  rpc Cmd(PipedCmd) returns (stream google.protobuf.Any);
  rpc Plan(PipedCmd) returns (stream PipedCall);
  rpc Exec(PipedCall) returns (stream google.protobuf.Any);
  rpc End(stream google.protobuf.Any) returns (google.protobuf.Empty);

  rpc Load(google.protobuf.Any) returns(stream google.protobuf.Any);
  rpc Map(PipedCall) returns(PipedCall);
  rpc Filter(PipedCall) returns(PipedCall);
  rpc Reduce(PipedCall) returns (google.protobuf.Any);
  rpc Collect(stream google.protobuf.Any) returns (google.protobuf.Any);
}
```
