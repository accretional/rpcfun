# Premise

What if we used something like the builder logic from https://pkg.go.dev/github.com/jhump/protoreflect/v2@v2.0.0-beta.2/protobuilder#pkg-overview, dynamic loading of service implementation binaries, and explicitly bound, named Method-Impl pairs to create a Function abstraction in proto/grpc?

Then, what if we implemented general support for the Unix File I/O pattern (actually more based on Plan9, because we want everything to be a file) and MapReduce to implement proto-based ETL on the command line? Like a command that sequences Unix pipes to process text, except with proto messages or arbitrary byte streams. Some of the code below is AI generated but it's based on a pattern I've seen before, starting a grpc server over uds dynamically and configuring stdin through it and stdout out of it. It just also implements hot loading of a service impl

```proto

service StreamService {
  rpc Process(stream Chunk) returns (stream Chunk);
}

message Chunk {
  bytes data = 1; // or google.protobuf.Any
  bool eof = 2;
}

---

// plugin/interface.go
package plugin

import (
	"google.golang.org/grpc"
)

// ServicePlugin is implemented by dynamically loaded services
type ServicePlugin interface {
	Register(server *grpc.Server)
}

// Factory function signature - plugins export "NewPlugin" with this type
type Factory func() ServicePlugin

---

func NewPlugin() pluginiface.ServicePlugin {
	return &EchoService{}
}

type EchoService struct {
	pb.UnimplementedStreamServiceServer
}

func (s *EchoService) Register(server *grpc.Server) {
	pb.RegisterStreamServiceServer(server, s)
}

func (s *EchoService) Process(stream pb.StreamService_ProcessServer) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if chunk.Eof {
			return nil
		}

		// Echo back with transformation
		response := &pb.Chunk{
			Data: []byte(fmt.Sprintf("ECHO: %s", chunk.Data)),
		}
		if err := stream.Send(response); err != nil {
			return err
		}
	}
}

---

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__server__" {
		// Server mode: args are [binary, __server__, socket_path, plugin_path]
		if len(os.Args) < 4 {
			log.Fatal("server mode requires socket and plugin paths")
		}
		runServer(os.Args[2], os.Args[3])
		return
	}

	// Client mode
	runClient()
}

// =============================================================================
// SERVER MODE
// =============================================================================

func runServer(socketPath, pluginPath string) {
	// Load the plugin
	p, err := goplugin.Open(pluginPath)
	if err != nil {
		log.Fatalf("failed to open plugin: %v", err)
	}

	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		log.Fatalf("plugin missing NewPlugin: %v", err)
	}

	factory, ok := sym.(func() pluginiface.ServicePlugin)
	if !ok {
		log.Fatal("invalid NewPlugin signature")
	}

	svc := factory()

	// Create UDS listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", socketPath, err)
	}
	defer listener.Close()

	// Create gRPC server
	server := grpc.NewServer()
	svc.Register(server)
	reflection.Register(server)

	// Signal parent we're ready by writing to stdout
	os.Stdout.Write([]byte("READY\n"))

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		cancel()
		server.GracefulStop()
	}()

	// Also stop when stdin closes (parent died)
	go func() {
		buf := make([]byte, 1)
		for {
			_, err := os.Stdin.Read(buf)
			if err != nil {
				cancel()
				server.GracefulStop()
				return
			}
		}
	}()

	// Serve
	if err := server.Serve(listener); err != nil && ctx.Err() == nil {
		log.Fatalf("serve error: %v", err)
	}
}

func runClient() {
	// Read first line from stdin: the plugin path
	reader := bufio.NewReader(os.Stdin)
	pluginPath, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("failed to read plugin path from stdin: %v", err)
	}
	pluginPath = pluginPath[:len(pluginPath)-1] // trim newline

	// Resolve plugin path
	absPluginPath, err := filepath.Abs(pluginPath)
	if err != nil {
		log.Fatalf("invalid plugin path: %v", err)
	}

	if _, err := os.Stat(absPluginPath); os.IsNotExist(err) {
		log.Fatalf("plugin not found: %s", absPluginPath)
	}

	// Create temp socket
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("grpc-%d.sock", os.Getpid()))
	defer os.Remove(socketPath)

	// Spawn server subprocess
	self, _ := os.Executable()
	cmd := exec.Command(self, "__server__", socketPath, absPluginPath)
	cmd.Stdout = os.Stdout // Server stdout -> our stdout
	cmd.Stderr = os.Stderr

	// Create pipe so server knows when we die
	serverStdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("failed to create stdin pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	// Cleanup on exit
	defer func() {
		serverStdin.Close()
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
		os.Remove(socketPath)
	}()

	// Wait for server to be ready (it prints READY to stdout)
	// We need to wait for the socket to exist
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Small extra delay for server to start listening
	time.Sleep(50 * time.Millisecond)

	// Connect to server
	conn, err := grpc.Dial(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewStreamServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.Process(ctx)
	if err != nil {
		log.Fatalf("failed to create stream: %v", err)
	}

	// Receive responses in background
	done := make(chan error, 1)
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				done <- nil
				return
			}
			if err != nil {
				done <- err
				return
			}
			// Write response data directly to stdout
			os.Stdout.Write(resp.Data)
			os.Stdout.Write([]byte("\n"))
		}
	}()

	// Stream remaining stdin to server
	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := &pb.Chunk{Data: buf[:n]}
			if err := stream.Send(chunk); err != nil {
				log.Fatalf("send error: %v", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("stdin read error: %v", err)
		}
	}

	// Signal end of input
	stream.Send(&pb.Chunk{Eof: true})
	stream.CloseSend()

	// Wait for receiver
	if err := <-done; err != nil {
		log.Fatalf("receive error: %v", err)
	}
}

```

Doing this with Map Reduce would required some notion of higher order functions, I think. It's annoying to represent those because they have different message types than their base Functions. The Functions below I think aren't capable of things like bound function arguments and currying.

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

Instead of PipedCall we could actually just do a BuildService after hot loading, and create the permutations between HoF and Functions either with some upfront parsing/planning, or at each step they are ran. I think?
