package main

import (
	"buf.build/gen/go/nativeconnect/api/grpc/go/nativeconnect/api/v1/apiv1grpc"
	v1 "buf.build/gen/go/nativeconnect/api/protocolbuffers/go/nativeconnect/api/v1"
	"context"
	"fmt"
	"github.com/DIMO-Network/shared"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"time"
)

type compassWrapper struct {
	client   apiv1grpc.ServiceClient
	ctx      context.Context
	logger   zerolog.Logger
	settings Settings
}

func main() {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().
		Timestamp().
		Str("app", "nativeconnect-go-service").
		Logger()

	settings, err := shared.LoadConfig[Settings]("./settings.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load settings")
	}

	creds := credentials.NewClientTLSFromCert(nil, "") // Load the system's root CA pool
	conn, err := grpc.Dial("dns:///nativeconnect.cloud:443", grpc.WithTransportCredentials(creds))
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to nativeconnect")
	}
	defer conn.Close()

	client := apiv1grpc.NewServiceClient(conn)
	ctx := context.Background()

	cw := &compassWrapper{
		client:   client,
		ctx:      ctx,
		logger:   logger,
		settings: settings,
	}

	// Get access token on startup
	authenticate, err := cw.client.Authenticate(ctx, &v1.AuthenticateRequest{Token: settings.CompassAPIKey})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to authenticate")
	}
	cw.ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
		"authorization": "Bearer " + authenticate.AccessToken,
	}))

	// Get all vehicles and VINs
	vehicles, err := cw.getVehicles()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to get vehicles")
	}

	// Start stream based on all VINs found
	cw.startStream(vehicles)
}

func (cw *compassWrapper) getVehicles() ([]string, error) {
	vehicles, err := cw.client.GetVehicles(cw.ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	cw.logger.Info().Msgf("Received vehicles from Compass API service: %d", len(vehicles.ProviderGet))

	var vins []string
	if vehicles.ProviderGet != nil {
		for _, request := range vehicles.ProviderGet {
			fmt.Println(request)
			car := request.GetJeep()
			vins = append(vins, car.GetVin())
		}
	}
	return vins, nil
}

func (cw *compassWrapper) startStream(vins []string) {
	for {
		timeoutCtx, cancel := context.WithTimeout(cw.ctx, time.Minute*10)
		defer cancel()
		// Use the provided context
		realtimeData, err := cw.client.RealtimeRawPointByVins(timeoutCtx, &v1.RealtimeRawPointByVinsRequest{
			Vins:                vins,
			MaxStalenessMinutes: 5,
		})
		if err != nil {
			cw.logger.Error().Err(err).Msg("failed to get realtime data, retrying...")
			time.Sleep(time.Second * 5) // Wait before retrying
			continue
		}

		// Read messages from the stream
		cw.logger.Info().Msg("Receiving stream messages:")
		for {
			resp, err := realtimeData.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					cw.logger.Info().Msg("Stream ended.")
					break
				}

				cw.logger.Err(err).Msg("Error receiving from stream, retrying...")
				break
			}

			// Log the received message
			cw.logger.Info().Interface("stream_data", resp).Msg("Received stream data")
		}
	}
}
