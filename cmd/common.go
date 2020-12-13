package cmd

import "google.golang.org/grpc"

func GrpcDial() (*grpc.ClientConn, error) {
	target, err := rootCmd.Flags().GetString("host")
	if err != nil {
		return nil, err
	}
	return grpc.Dial(target, grpc.WithInsecure())
}
