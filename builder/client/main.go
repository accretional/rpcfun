package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/yourorg/buildservice/gen/buildservice/v1"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRemoteBuildServiceClient(conn)
	ctx := context.Background()

	// 1. Create build session
	session, err := client.CreateBuildSession(ctx, &pb.CreateBuildSessionRequest{
		ProjectName: "my-grpc-service",
	})
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	fmt.Printf("Session created: %s\n", session.SessionId)

	// 2. Upload files
	files, _ := collectProjectFiles("./myproject")
	_, err = client.UploadFilesSimple(ctx, &pb.UploadFilesSimpleRequest{
		SessionId: session.SessionId,
		Files:     files,
	})
	if err != nil {
		log.Fatalf("failed to upload files: %v", err)
	}

	// 3. Validate project
	validation, err := client.ValidateProject(ctx, &pb.ValidateProjectRequest{
		SessionId: session.SessionId,
		ProtoOptions: &pb.ProtoCompileOptions{
			GenerateGateway:    true,
			GenerateValidation: true,
		},
	})
	if err != nil {
		log.Fatalf("validation failed: %v", err)
	}
	fmt.Printf("Discovered %d services\n", len(validation.DiscoveredServices))

	// 4. Register services
	_, err = client.RegisterServices(ctx, &pb.RegisterServicesRequest{
		SessionId: session.SessionId,
		Services:  validation.SuggestedRegistrations,
		ServerConfig: &pb.ServerConfig{
			GrpcAddress:       ":50051",
			EnableReflection:  true,
			EnableHealthCheck: true,
			Interceptors: &pb.InterceptorConfig{
				Logging:  true,
				Recovery: true,
				Metrics:  true,
			},
		},
	})
	if err != nil {
		log.Fatalf("registration failed: %v", err)
	}

	// 5. Start build
	build, err := client.StartBuild(ctx, &pb.StartBuildRequest{
		SessionId: session.SessionId,
		Config: &pb.BuildConfig{
			GoVersion:  "1.22",
			Goos:       "linux",
			Goarch:     "amd64",
			StaticLink: true,
			TrimPath:   true,
		},
		RunTests:         true,
		BuildDockerImage: true,
		DockerConfig: &pb.DockerConfig{
			Tags: []string{"myregistry/myservice:latest"},
		},
	})
	if err != nil {
		log.Fatalf("build start failed: %v", err)
	}
	fmt.Printf("Build started: %s\n", build.BuildId)

	// 6. Stream logs
	logStream, err := client.StreamBuildLogs(ctx, &pb.StreamBuildLogsRequest{
		BuildId: build.BuildId,
	})
	if err != nil {
		log.Fatalf("failed to stream logs: %v", err)
	}

	for {
		entry, err := logStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("log stream error: %v", err)
		}
		fmt.Printf("[%s] %s: %s\n", entry.Stage, entry.Level, entry.Message)
	}

	// 7. Download artifact
	artifacts, _ := client.GetArtifacts(ctx, &pb.GetArtifactsRequest{
		BuildId: build.BuildId,
	})

	for _, artifact := range artifacts.Artifacts {
		if artifact.Type == pb.ArtifactType_ARTIFACT_TYPE_BINARY {
			downloadArtifact(client, build.BuildId, artifact)
		}
	}
}

func collectProjectFiles(dir string) ([]*pb.File, error) {
	var files []*pb.File
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, _ := os.ReadFile(path)
		relPath, _ := filepath.Rel(dir, path)
		
		fileType := pb.FileType_FILE_TYPE_OTHER
		switch filepath.Ext(path) {
		case ".proto":
			fileType = pb.FileType_FILE_TYPE_PROTO
		case ".go":
			fileType = pb.FileType_FILE_TYPE_GO_SOURCE
		}
		if filepath.Base(path) == "go.mod" {
			fileType = pb.FileType_FILE_TYPE_GO_MOD
		}
		
		files = append(files, &pb.File{
			Path:    relPath,
			Content: content,
			Type:    fileType,
		})
		return nil
	})
	return files, err
}

func downloadArtifact(client pb.RemoteBuildServiceClient, buildID string, artifact *pb.Artifact) {
	// Implementation for streaming download
}
