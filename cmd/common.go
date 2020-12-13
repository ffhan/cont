package cmd

import "google.golang.org/grpc"

func GrpcDial() (*grpc.ClientConn, error) {
	target, err := rootCmd.Flags().GetString("host")
	if err != nil {
		return nil, err
	}
	return grpc.Dial(target, grpc.WithInsecure())
}

const (
	Failed int32 = iota
	Created
	Started
	Done
	Killed // todo: make a distinction between done and killed
)
