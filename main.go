package main

import (
	"buf.build/gen/go/nativeconnect/api/grpc/go/nativeconnect/api/v1/apiv1grpc"
	v1 "buf.build/gen/go/nativeconnect/api/protocolbuffers/go/nativeconnect/api/v1"
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"github.com/DIMO-Network/shared"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().
		Timestamp().
		Str("app", "nativeconnect-go-example").
		//Str("git-sha1", gitSha1).
		Logger()
	settings, err := shared.LoadConfig[Settings]("settings.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load settings")
	}
	creds := credentials.NewClientTLSFromCert(nil, "") // Load the system's root CA pool

	if settings.ConsentEmail == "" {
		logger.Fatal().Msg("consent email is required setting")
	}
	conn, err := grpc.NewClient("dns:///nativeconnect.cloud:443", grpc.WithTransportCredentials(creds))
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to nativeconnect")
	}
	defer conn.Close()

	client := apiv1grpc.NewServiceClient(conn)

	ctx := context.Background()

	authenticate, err := client.Authenticate(ctx, &v1.AuthenticateRequest{Token: settings.CompassAPIKey})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to authenticate")
	}
	fmt.Println(authenticate)

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + authenticate.AccessToken,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	menuPrompt(&compassWrapper{
		client:   client,
		ctx:      ctx,
		logger:   logger,
		settings: &settings,
	})
}

func menuPrompt(cw *compassWrapper) {
	// Display menu and prompt the user
	for {
		fmt.Println("Please choose an option:")
		fmt.Println("1. Get Vehicles in Compass")
		fmt.Println("2. add a VIN to Compass")
		fmt.Println("3. Check the Consent for a VIN")
		fmt.Println("4. Check Compatibility for a VIN")
		fmt.Println("5. Get Last Reported Points for a VIN")
		fmt.Println("6. Get realtime data for a VIN")
		fmt.Println("7. Lock Vehicle")
		fmt.Println("8. Remove Vehicle")
		fmt.Print("Enter your choice: ")

		// Read user input
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		choice, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil {
			fmt.Println("Invalid input. Please enter a number between 1 and 6.")
			continue
		}

		// Call the corresponding function
		switch choice {
		case 1:
			cw.getVehicles()
		case 2:
			cw.onboardVIN()
		case 3:
			cw.checkConsent()
		case 4:
			cw.checkCompatibility()
		case 5:
			cw.lastReportedPoints()
		case 6:
			cw.realtimeData()
		case 7:
			cw.Lock()
		case 8:
			cw.RemoveVehicle()
		default:
			fmt.Println("Invalid choice. Please select a valid option.")
		}
		return
	}
}

type compassWrapper struct {
	client   apiv1grpc.ServiceClient
	ctx      context.Context
	logger   zerolog.Logger
	settings *Settings
}

func (cw *compassWrapper) RemoveVehicle() {
	vin, isVIN := promptForVIN()
	vins := []string{vin}
	if isVIN {
		vins = append(vins, vin)
	} else {
		// read from csv file in local path and input into vins
		fromCSV, err := readVinsFromCSV(vin)
		if err != nil {
			cw.logger.Fatal().Err(err).Msg("failed to read csv file")
		}
		vins = fromCSV
	}
	for _, v := range vins {
		vehicle, err := cw.client.RemoveVehicle(cw.ctx, &v1.RemoveVehicleRequest{
			Vins: []string{v},
		})
		if err != nil {
			cw.logger.Error().Err(err).Msg("failed to delete vehicle")
			continue
		}
		fmt.Println("removed")
		fmt.Println(vehicle)
	}
}

// Lock may not work in NA yet, but works in other regions
func (cw *compassWrapper) Lock() {
	vin, _ := promptForVIN()
	_, err := cw.client.IssueAction(cw.ctx, &v1.IssueActionRequest{
		Vin:     vin,
		Command: &v1.IssueActionRequest_Lock{Lock: &v1.SetLockCommand{Locked: true}},
	})
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to lock vehicle")
	}
	fmt.Println("locked")
}

// readVinsFromCSV opens the CSV file at filePath and returns a slice of VINs from the first column.
func readVinsFromCSV(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %q: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV file %q: %w", filePath, err)
	}

	var vins []string
	for _, record := range records {
		if len(record) > 0 {
			vins = append(vins, record[0])
		}
	}
	return vins, nil
}

func (cw *compassWrapper) getVehicles() {
	vehicles, err := cw.client.GetVehicles(cw.ctx, &emptypb.Empty{})

	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to get vehicles")
	}
	fmt.Println("number of vehicles:", len(vehicles.ProviderGet))
	if vehicles.ProviderGet != nil {
		for i, request := range vehicles.ProviderGet {
			fmt.Println(i, request)

			car := request.GetChrysler()
			fmt.Println("VIN: " + car.GetVin())
		}
	}
	fmt.Println(vehicles)
}

func (cw *compassWrapper) onboardVIN() {
	vin, _ := promptForVIN()
	vehicleSignUp, err := cw.client.BatchVehicleSignUp(cw.ctx, &v1.BatchVehicleSignUpRequest{
		ConsentEmail: cw.settings.ConsentEmail,
		Consent: []*v1.Consent{
			{
				ProviderAuth: &v1.AuthRequest{Provider: &v1.AuthRequest_Vin{Vin: &v1.VinAuth{Vin: vin}}},
				//Scopes:       make([]v1.Scope, v1.Scope_SCOPE_READ, v1.Scope_SCOPE_COMMAND),
				Region: 2, // NA
			},
		},
	})
	fmt.Println("using consent email: " + cw.settings.ConsentEmail)
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to sign up vehicle")
	}
	fmt.Println(vehicleSignUp)
}

func (cw *compassWrapper) checkConsent() {
	vin, _ := promptForVIN()
	consent, err := cw.client.CheckConsent(cw.ctx, &v1.CheckConsentRequest{Vin: vin})
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to check consent")
	}
	fmt.Println(consent)
}

func (cw *compassWrapper) checkCompatibility() {
	vin, _ := promptForVIN()
	compatibility, err := cw.client.CheckCompatibility(cw.ctx, &v1.CheckCompatibilityRequest{Vin: vin})
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to check compatibility")
	}
	fmt.Println(compatibility)
}

func (cw *compassWrapper) lastReportedPoints() {
	vin, _ := promptForVIN()
	lastReportedPoints, err := cw.client.GetLastReportedPoints(cw.ctx, &v1.GetLastReportedPointsRequest{Vin: vin,
		Points: 5})
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to get last reported points")
	}
	fmt.Println("number of events: ", len(lastReportedPoints.Events))
	for i, event := range lastReportedPoints.Events {
		fmt.Println(i, event)
	}
	fmt.Println(lastReportedPoints)
}

func (cw *compassWrapper) realtimeData() {
	vin, _ := promptForVIN()
	timeoutCtx, cancel := context.WithTimeout(cw.ctx, time.Minute*10)
	defer cancel()
	realtimeData, err := cw.client.RealtimeRawPointByVins(timeoutCtx, &v1.RealtimeRawPointByVinsRequest{Vins: []string{vin},
		MaxStalenessMinutes: 5})
	if err != nil {
		cw.logger.Fatal().Err(err).Msg("failed to get realtime data")
	}
	// Read messages from the stream
	fmt.Println("Receiving stream messages:")
	for {
		// Receive a message from the stream
		resp, err := realtimeData.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// Stream ended gracefully
				fmt.Println("Stream ended.")
				break
			}
			log.Fatalf("Error receiving from stream: %v", err)
		}

		// Process the received message
		fmt.Println(resp)
	}
}

func promptForVIN() (string, bool) {
	reader := bufio.NewReader(os.Stdin)

	// Prompt for VIN input
	for {
		fmt.Print("Enter a 17-character VIN, or a file name csv with multiple VINs (remove only): ")
		vin, _ := reader.ReadString('\n')
		vin = strings.TrimSpace(vin) // Remove any leading/trailing spaces

		// Validate VIN length
		if len(vin) != 17 {
			fmt.Println("File name recognized")
			return vin, false
		}

		// Optionally, validate VIN content (alphanumeric, no special characters except letters/numbers)
		if !isValidVIN(vin) {
			fmt.Println("Invalid VIN. It should only contain alphanumeric characters. Please try again.")
			continue
		}

		// If valid, process the VIN
		fmt.Printf("Processing VIN: %s\n", vin)

		// Example work: Extracting parts of the VIN (e.g., WMI, VDS, VIS)
		wmi := vin[:3]  // World Manufacturer Identifier
		vds := vin[3:9] // Vehicle Descriptor Section
		vis := vin[9:]  // Vehicle Identifier Section

		// Display extracted components
		fmt.Println("Extracted VIN components:")
		fmt.Printf("  WMI (Manufacturer): %s\n", wmi)
		fmt.Printf("  VDS (Descriptor): %s\n", vds)
		fmt.Printf("  VIS (Identifier): %s\n", vis)

		return vin, true
	}
}

// Helper function to validate VIN content (alphanumeric characters only)
func isValidVIN(vin string) bool {
	for _, char := range vin {
		// VINs should only contain letters and numbers, but exclude I, O, and Q
		if !(('A' <= char && char <= 'Z') || ('0' <= char && char <= '9')) {
			return false
		}
		if char == 'I' || char == 'O' || char == 'Q' {
			return false
		}
	}
	return true
}
