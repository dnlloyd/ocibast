package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/oracle/oci-go-sdk/v65/bastion"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

const logLevel = "DEBUG" // TODO: allow setting log level via env var

type SessionInfo struct {
	state bastion.SessionLifecycleStateEnum
	ip    string
	user  string
	port  int
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	checkError(err)

	return homeDir
}

func initializeOciClients() (identity.IdentityClient, bastion.BastionClient) {
	var config common.ConfigurationProvider

	profile, exists := os.LookupEnv("OCI_CLI_PROFILE")

	if exists {
		if logLevel == "DEBUG" {
			fmt.Println("Using profile " + profile)
		}

		homeDir := getHomeDir()
		configPath := homeDir + "/.oci/config"

		config = common.CustomProfileConfigProvider(configPath, profile)
	} else {
		if logLevel == "DEBUG" {
			fmt.Println("Using default profile")
		}
		config = common.DefaultConfigProvider()
	}

	identityClient, identityErr := identity.NewIdentityClientWithConfigurationProvider(config)
	checkError(identityErr)

	bastionClient, err := bastion.NewBastionClientWithConfigurationProvider(config)
	checkError(err)

	return identityClient, bastionClient
}

func getCompartmentInfo(tenancyId string, client identity.IdentityClient) map[string]string {
	response, err := client.ListCompartments(context.Background(), identity.ListCompartmentsRequest{CompartmentId: &tenancyId})
	checkError(err)

	compartmentInfo := make(map[string]string)

	for _, item := range response.Items {
		compartmentInfo[*item.Name] = *item.Id
	}

	return compartmentInfo
}

func listCompartmentNames(compartmentInfo map[string]string) {
	fmt.Println("\nCOMPARTMENTS:")

	compartmentNames := make([]string, 0, len(compartmentInfo))
	for compartmentName := range compartmentInfo {
		compartmentNames = append(compartmentNames, compartmentName)
	}
	sort.Strings(compartmentNames)

	for _, compartmentName := range compartmentNames {
		println(compartmentName)
	}

	fmt.Println("\nTo set compartment, you can export OCI_COMPARTMENT_NAME:")
	fmt.Println("   export OCI_COMPARTMENT_NAME=")
}

func getCompartmentName(flagCompartmentName string) string {
	var compartmentName string
	compartmentIdEnv, exists := os.LookupEnv("OCI_COMPARTMENT_NAME")
	if exists {
		compartmentName = compartmentIdEnv
		if logLevel == "DEBUG" {
			fmt.Println("Compartment name is set via OCI_COMPARTMENT_NAME to: " + compartmentName)
		}
	} else if flagCompartmentName == "" {
		fmt.Println("Must pass compartment name with -c or set with environment variable OCI_COMPARTMENT_NAME")
		os.Exit(1)
	} else {
		compartmentName = flagCompartmentName
	}

	return compartmentName
}

func getCompartmentId(compartmentInfo map[string]string, compartmentName string) string {
	compartmentId := compartmentInfo[compartmentName]
	if logLevel == "DEBUG" {
		fmt.Println("\n" + compartmentName + "'s compartment ID is " + compartmentId)
	}

	return compartmentId
}

func getBastionInfo(compartmentId string, client bastion.BastionClient) map[string]string {
	response, err := client.ListBastions(context.Background(), bastion.ListBastionsRequest{CompartmentId: &compartmentId})
	checkError(err)

	bastionInfo := make(map[string]string)

	for _, item := range response.Items {
		bastionInfo[*item.Name] = *item.Id
	}

	return bastionInfo
}

func listBastions(compartmentName string, bastionInfo map[string]string) {
	fmt.Println("\nBastions in compartment " + compartmentName)
	for bastionName := range bastionInfo {
		fmt.Println(bastionName)
	}
}

func getBastion(bastionName string, bastionId string, client bastion.BastionClient) {
	if logLevel == "DEBUG" {
		fmt.Println("\nGetting bastion for: " + bastionName + " (" + bastionId + ")")
	}

	_, err := client.GetBastion(context.Background(), bastion.GetBastionRequest{BastionId: &bastionId})
	checkError(err)
}

func getSshPubKeyContents(sshPrivateKeyFileLocation string) string {
	homeDir := getHomeDir()

	if sshPrivateKeyFileLocation == "" {
		sshPrivateKeyFileLocation = homeDir + "/.ssh/id_rsa"
		fmt.Println("\nUsing default SSH identity file at " + sshPrivateKeyFileLocation)
	}

	sshKeyContents, err := os.ReadFile(sshPrivateKeyFileLocation)
	checkError(err)

	return string(sshKeyContents)
}

func createSession(bastionId string, client bastion.BastionClient, targetInstance string, targetIp string, publicKeyContent string, sshUser string, sshPort int) *string {
	req := bastion.CreateSessionRequest{
		CreateSessionDetails: bastion.CreateSessionDetails{
			BastionId:           &bastionId,
			DisplayName:         common.String("OCIBastionSession"), // TODO: Maybe set this programmatically
			KeyDetails:          &bastion.PublicKeyDetails{PublicKeyContent: &publicKeyContent},
			SessionTtlInSeconds: common.Int(1800),
			TargetResourceDetails: bastion.CreateManagedSshSessionTargetResourceDetails{
				TargetResourceId:                      &targetInstance,
				TargetResourceOperatingSystemUserName: &sshUser,
				TargetResourcePort:                    &sshPort,
				TargetResourcePrivateIpAddress:        &targetIp,
			},
		},
	}

	fmt.Println("\nCreating session...")
	response, err := client.CreateSession(context.Background(), req)
	checkError(err)

	if logLevel == "DEBUG" {
		fmt.Println("\nCreateSessionResponse")
		fmt.Println(response)
	}

	sessionId := response.Session.Id
	fmt.Println("\nSession ID: ")
	fmt.Println(*sessionId)

	return sessionId
}

func checkSession(client bastion.BastionClient, sessionId *string) SessionInfo {
	response, err := client.GetSession(context.Background(), bastion.GetSessionRequest{SessionId: sessionId})
	checkError(err)

	if logLevel == "DEBUG" {
		fmt.Println("GetSessionResponse")
		fmt.Println(response.Session)

		fmt.Println("\nEndpoint")
		fmt.Println(client.Endpoint())
	}

	sshSessionTargetResourceDetails := response.Session.TargetResourceDetails.(bastion.ManagedSshSessionTargetResourceDetails)
	ipAddress := sshSessionTargetResourceDetails.TargetResourcePrivateIpAddress
	sshUser := sshSessionTargetResourceDetails.TargetResourceOperatingSystemUserName
	sshPort := sshSessionTargetResourceDetails.TargetResourcePort

	currentSessionInfo := SessionInfo{response.Session.LifecycleState, *ipAddress, *sshUser, *sshPort}

	return currentSessionInfo
}

func listActiveSessions(client bastion.BastionClient, bastionId string) {
	response, err := client.ListSessions(context.Background(), bastion.ListSessionsRequest{BastionId: &bastionId})
	checkError(err)

	fmt.Println("\nActive bastion sessions")
	for _, session := range response.Items {
		sshSessionTargetResourceDetails := session.TargetResourceDetails.(bastion.ManagedSshSessionTargetResourceDetails)
		instanceName := sshSessionTargetResourceDetails.TargetResourceDisplayName
		ipAddress := sshSessionTargetResourceDetails.TargetResourcePrivateIpAddress
		instanceID := sshSessionTargetResourceDetails.TargetResourceId

		if session.LifecycleState == "ACTIVE" {
			fmt.Println(*session.DisplayName)
			fmt.Println(*session.Id)
			fmt.Println(*session.TimeCreated)
			fmt.Println(*instanceName)
			fmt.Println(*ipAddress)
			fmt.Println(*instanceID)
			fmt.Println("")
		}
	}
}

func printSshCommands(client bastion.BastionClient, sessionId *string, instanceIp *string, sshUser *string, sshPort *int, sshIdentityFile string) {
	bastionEndpointUrl, err := url.Parse(client.Endpoint())
	checkError(err)

	sessionIdStr := *sessionId
	bastionHost := sessionIdStr + "@host." + bastionEndpointUrl.Host

	fmt.Println("\nTunnel:")
	fmt.Println("ssh -N -L <LOCAL PORT>:" + *instanceIp + ":<DESTINATION PORT> \\")
	fmt.Println("-i " + sshIdentityFile + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i " + sshIdentityFile + " -W %h:%p -p 22 " + bastionHost + "' \\")
	fmt.Println("-P " + strconv.Itoa(*sshPort) + " " + *sshUser + "@" + *instanceIp)

	fmt.Println("\nSSH:")
	fmt.Println("ssh -i " + sshIdentityFile + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i " + sshIdentityFile + " -W %h:%p -p 22 " + bastionHost + "' \\")
	fmt.Println("-P " + strconv.Itoa(*sshPort) + " " + *sshUser + "@" + *instanceIp)
}

func getTenancyId(tenancyIdFlag string, client identity.IdentityClient) string {
	tenancyId, exists := os.LookupEnv("OCI_CLI_TENANCY")
	if !exists {
		if tenancyIdFlag == "" {
			fmt.Println("Must pass tenancy ID with -t or set with environment variable OCI_CLI_TENANCY")
			os.Exit(1)
		}
	} else {
		if logLevel == "DEBUG" {
			fmt.Println("\nTenancy ID is set via OCI_CLI_TENANCY to: " + tenancyId)
		}
	}

	// Validate tenancy ID
	response, err := client.GetTenancy(context.Background(), identity.GetTenancyRequest{TenancyId: &tenancyId})
	checkError(err)

	if logLevel == "DEBUG" {
		fmt.Println("\nCurrent tenant: " + *response.Tenancy.Name)
	}

	return tenancyId
}

func main() {
	flagTenancyId := flag.String("t", "", "tenancy ID name")
	flagListCompartments := flag.Bool("lc", false, "list compartments")
	flagCompartmentName := flag.String("c", "", "compartment name")
	flagListBastions := flag.Bool("lb", false, "list bastions")
	flagBastionName := flag.String("b", "", "bastion name")
	flagInstanceId := flag.String("o", "", "instance ID of host to connect to")
	flagInstanceIp := flag.String("i", "", "instance IP address of host to connect to")
	flagSessionId := flag.String("s", "", "Session ID to check for")
	flagListSessions := flag.Bool("ls", false, "list sessions")
	flagSshUser := flag.String("u", "opc", "SSH user")
	flagSshPort := flag.Int("p", 22, "SSH port")
	flagSshPrivateKey := flag.String("k", "", "path to SSH private key (identity file)")
	flagSshPublicKey := flag.String("e", "", "path to SSH public key")

	// Extend flag's default usage function
	flag.Usage = func() {
		fmt.Println("OCI authentication:")
		fmt.Println("This tool will use the credentials set in $HOME/.oci/config")
		fmt.Println("This tool will use the profile set by the OCI_CLI_PROFILE environment variable")
		fmt.Println("If the OCI_CLI_PROFILE environment variable is not set it will use the DEFAULT profile")

		fmt.Println("\nEnvironment variables:")
		fmt.Println("The following environment variables will override their flag counterparts")
		fmt.Println("   OCI_CLI_TENANCY")
		fmt.Println("   OCI_COMPARTMENT_NAME")

		fmt.Println("\nDefaults:")
		fmt.Println("   SSH private key (-k): $HOME/.ssh/id_rsa")
		fmt.Println("   SSH public key (-e): $HOME/.ssh/id_rsa.pub")

		fmt.Println("\nCommon command patterns:")
		fmt.Println("List compartments")
		fmt.Println("   ocibast -list-compartments")
		fmt.Println("\nList bastions")
		fmt.Println("   ocibast -c compartment_name -list-bastions")
		fmt.Println("\nCreate bastion session")
		fmt.Println("   ocibast -b bastion_name -i ip_address -o instance_id")
		fmt.Println("\nCreate bastion session (long)")
		fmt.Println("   ocibast -t tenant_id -c compartment_name -b bastion_name -i ip_address -o instance_id -k path_to_ssh_private_key -e path_to_ssh_public_key")
		fmt.Println("\nList active sessions")
		fmt.Println("   ocibast -c mycompartment -b mybastion -list-sessions")
		fmt.Println("\nConnect to an active session")
		fmt.Println("   ocibast -c compartment_name -b bastion_name -k path_to_ssh_private_key -e path_to_ssh_public_key -s session_ocd")

		fmt.Println("\nExample of bastion session creation:")
		fmt.Println("   export OCI_CLI_TENANCY=ocid1.tenancy.oc1..aaaaaaaaabcdefghijklmnopqrstuvwxyz")
		fmt.Println("   export OCI_COMPARTMENT_NAME=mycompartment")
		fmt.Println("   ocibast -b mybastion -i 10.0.0.123 -o ocid1.instance.oc1.iad.abcdefg")
		fmt.Fprintf(flag.CommandLine.Output(), "\nAll flags for %s:\n", os.Args[0])

		flag.PrintDefaults()
	}

	flag.Parse()

	identityClient, bastionClient := initializeOciClients()

	tenancyId := getTenancyId(*flagTenancyId, identityClient)
	compartmentInfo := getCompartmentInfo(tenancyId, identityClient)

	// Using switch in preparation to convert main flow to individual directives
	// Will eventually switch on 1st argument: e.g. 'list' (not on flags)
	switch *flagListCompartments {
	case true:
		listCompartmentNames(compartmentInfo)
		os.Exit(0)
	}

	// Anything past this point requires a compartment and bastion info
	compartmentName := getCompartmentName(*flagCompartmentName)
	compartmentId := getCompartmentId(compartmentInfo, compartmentName)
	bastionInfo := getBastionInfo(compartmentId, bastionClient)

	if *flagListBastions {
		listBastions(compartmentName, bastionInfo)
		os.Exit(0)
	}

	// Anything past this point requires a bastion
	bastionId := bastionInfo[*flagBastionName]
	getBastion(*flagBastionName, bastionId, bastionClient)

	if *flagListSessions {
		listActiveSessions(bastionClient, bastionId)
		os.Exit(0)
	}

	homeDir := getHomeDir()

	var sshPrivateKeyFileLocation string
	if *flagSshPrivateKey == "" {
		// TODO: move this default to flags
		sshPrivateKeyFileLocation = homeDir + "/.ssh/id_rsa"
		if logLevel == "DEBUG" {
			fmt.Println("Using default SSH private key file at " + sshPrivateKeyFileLocation)
		}
	} else {
		sshPrivateKeyFileLocation = *flagSshPrivateKey
	}

	var sshPublicKeyFileLocation string
	if *flagSshPublicKey == "" {
		// TODO: move this default to flags
		sshPublicKeyFileLocation = homeDir + "/.ssh/id_rsa.pub"
		if logLevel == "DEBUG" {
			fmt.Println("\nUsing default SSH public key file at " + sshPublicKeyFileLocation)
		}
	} else {
		sshPublicKeyFileLocation = *flagSshPublicKey
	}

	publicKeyContent := getSshPubKeyContents(sshPublicKeyFileLocation)

	var sessionId *string
	if *flagSessionId == "" {
		// No session ID passed, create a new session
		sessionId = createSession(bastionId, bastionClient, *flagInstanceId, *flagInstanceIp, publicKeyContent, *flagSshUser, *flagSshPort)
	} else {
		// Check for existing session by session ID
		fmt.Println("Session ID passed, checking session...")
		sessionId = flagSessionId
		sessionInfo := checkSession(bastionClient, sessionId)

		if sessionInfo.state == "ACTIVE" {
			printSshCommands(bastionClient, sessionId, &sessionInfo.ip, &sessionInfo.user, &sessionInfo.port, sshPrivateKeyFileLocation)
		} else {
			fmt.Println("Session is no longer active. Current state is: " + sessionInfo.state)
		}

		os.Exit(0)
	}

	sessionInfo := checkSession(bastionClient, sessionId)

	for sessionInfo.state != "ACTIVE" {
		if sessionInfo.state == "DELETED" {
			fmt.Println("\nSession has been deleted, exiting")
			fmt.Println("State: " + sessionInfo.state)
			fmt.Println("\nSession Info")
			fmt.Println(sessionInfo)
			os.Exit(1)
		} else {
			fmt.Println("\nSession not yet active")
			fmt.Println("State: " + sessionInfo.state)
			fmt.Println("\nWaiting...")
			time.Sleep(10 * time.Second)
			sessionInfo = checkSession(bastionClient, sessionId)
		}
	}

	printSshCommands(bastionClient, sessionId, flagInstanceIp, flagSshUser, flagSshPort, sshPrivateKeyFileLocation)
}
